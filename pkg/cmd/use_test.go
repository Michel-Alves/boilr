package cmd_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ilyes512/boilr/pkg/cmd"
	"github.com/Ilyes512/boilr/pkg/util/tlog"
)

const (
	runMainEnv = "BOILR_TEST_RUN_MAIN"
	argsEnv    = "BOILR_TEST_ARGS"
	argsSep    = "\x1f"
)

// TestMain re-executes the test binary as the boilr CLI when runMainEnv is
// set, because the commands terminate through os.Exit.
func TestMain(m *testing.M) {
	if os.Getenv(runMainEnv) == "1" {
		os.Args = append([]string{"boilr"}, strings.Split(os.Getenv(argsEnv), argsSep)...)
		cmd.Run()
		os.Exit(0)
	}

	os.Exit(m.Run())
}

const useProjectJSON = `{
  "app_name": "myservice",
  "use_kafka": false,
  "stack_profile": ["document-db", "relational-db"],
  "db": {"name": "app", "db_port": 5432}
}`

const useTemplateFile = `app={{app_name}}
kafka={{use_kafka}}
stack={{stack_profile}}
db={{name}}:{{db_port}}
`

type result struct {
	out  string
	code int
}

// boilr runs the CLI in a child process with HOME pointed at home and
// stdin closed, failing the test if it doesn't finish in time.
func boilr(t *testing.T, home string, args ...string) result {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
	c.Env = append(os.Environ(),
		runMainEnv+"=1",
		argsEnv+"="+strings.Join(args, argsSep),
		"HOME="+home,
	)
	c.Stdin = nil // /dev/null

	out, err := c.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("boilr %v timed out (hang?), output:\n%s", args, out)
	}

	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running boilr %v: %v", args, err)
	}

	return result{out: string(out), code: code}
}

func writeTemplate(t *testing.T, dir, projectJSON, file string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "template"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.json"), []byte(projectJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "template", "{{app_name}}.txt"), []byte(file), 0644); err != nil {
		t.Fatal(err)
	}
}

// setup creates a HOME with an initialized registry containing the "svc"
// template, and returns HOME and a scratch dir for targets and values files.
func setup(t *testing.T) (home, work string) {
	t.Helper()

	home, work = t.TempDir(), t.TempDir()
	writeTemplate(t, filepath.Join(home, ".config", "boilr", "templates", "svc"), useProjectJSON, useTemplateFile)

	return home, work
}

func writeFile(t *testing.T, path, contents string) string {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}

	return path
}

func readRendered(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected rendered file: %v", err)
	}

	return string(b)
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s should not exist (stat err: %v)", path, err)
	}
}

func assertNoPrompt(t *testing.T, out string) {
	t.Helper()

	if strings.Contains(out, "["+tlog.QuestionMark+"]") {
		t.Errorf("output should contain no prompt, got:\n%s", out)
	}
}

// effectiveJSON parses the JSON object printed before the success line.
func effectiveJSON(t *testing.T, out string) (map[string]interface{}, string) {
	t.Helper()

	start := strings.Index(out, "{")
	end := strings.Index(out, "\n}")
	if start < 0 || end < 0 {
		t.Fatalf("no effective-values JSON in output:\n%s", out)
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out[start:end+2]), &m); err != nil {
		t.Fatalf("invalid effective-values JSON: %v\n%s", err, out)
	}

	return m, out[end+2:]
}

func TestUseWithValues(t *testing.T) {
	for name, extra := range map[string][]string{"values only": nil, "with use-defaults": {"-f"}} {
		t.Run(name, func(t *testing.T) {
			home, work := setup(t)
			values := writeFile(t, filepath.Join(work, "v.json"),
				`{"app_name": "orders", "stack_profile": "relational-db", "db": {"name": "ordersdb"}}`)
			target := filepath.Join(work, "out")

			args := append([]string{"template", "use", "svc", target, "--values", values}, extra...)
			r := boilr(t, home, args...)
			if r.code != 0 {
				t.Fatalf("exit %d, output:\n%s", r.code, r.out)
			}
			assertNoPrompt(t, r.out)

			got := readRendered(t, filepath.Join(target, "orders.txt"))
			want := "app=orders\nkafka=false\nstack=relational-db\ndb=ordersdb:5432\n"
			if got != want {
				t.Errorf("rendered = %q, want %q", got, want)
			}

			eff, rest := effectiveJSON(t, r.out)
			if eff["app_name"] != "orders" || eff["stack_profile"] != "relational-db" {
				t.Errorf("unexpected effective values: %v", eff)
			}
			if db, _ := eff["db"].(map[string]interface{}); db["name"] != "ordersdb" || db["db_port"] != float64(5432) {
				t.Errorf("unexpected effective db group: %v", eff["db"])
			}
			if !strings.Contains(rest, "Successfully executed") {
				t.Errorf("success line should follow the JSON, got:\n%s", r.out)
			}
		})
	}
}

func TestUseWithValuesErrors(t *testing.T) {
	tests := []struct {
		name     string
		contents *string // nil = missing file
		want     string
	}{
		{"unknown key", ptr(`{"app_nmae": "x"}`), `unknown key "app_nmae"`},
		{"invalid choice", ptr(`{"stack_profile": "graph-db"}`), "document-db, relational-db"},
		{"type mismatch", ptr(`{"use_kafka": "true"}`), "expected boolean"},
		{"missing file", nil, "couldn't read values file"},
		{"invalid json", ptr(`{"app_name": `), "not valid JSON"},
		{"non-object", ptr(`["orders"]`), "must contain a JSON object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, work := setup(t)
			values := filepath.Join(work, "v.json")
			if tt.contents != nil {
				writeFile(t, values, *tt.contents)
			}
			target := filepath.Join(work, "out")

			r := boilr(t, home, "template", "use", "svc", target, "--values", values)
			if r.code == 0 {
				t.Fatalf("expected non-zero exit, output:\n%s", r.out)
			}
			if !strings.Contains(r.out, tt.want) {
				t.Errorf("output should contain %q, got:\n%s", tt.want, r.out)
			}
			assertNotExists(t, target)
		})
	}
}

func TestUseWithoutValuesPrintsNoJSON(t *testing.T) {
	home, work := setup(t)
	target := filepath.Join(work, "out")

	r := boilr(t, home, "template", "use", "svc", target, "-f")
	if r.code != 0 {
		t.Fatalf("exit %d, output:\n%s", r.code, r.out)
	}
	if strings.Contains(r.out, "{") {
		t.Errorf("no JSON expected without --values, got:\n%s", r.out)
	}
	if got := readRendered(t, filepath.Join(target, "myservice.txt")); !strings.Contains(got, "stack=document-db") {
		t.Errorf("defaults expected, got %q", got)
	}
}

func TestUseFromSourceDir(t *testing.T) {
	home, work := setup(t)

	// Source differs from the registry copy of the same name.
	src := filepath.Join(work, "svc")
	writeTemplate(t, src, useProjectJSON, "source={{app_name}}\n")
	values := writeFile(t, filepath.Join(work, "v.json"), `{"app_name": "orders"}`)
	target := filepath.Join(work, "out")

	r := boilr(t, home, "template", "use", "--from", src, target, "--values", values)
	if r.code != 0 {
		t.Fatalf("exit %d, output:\n%s", r.code, r.out)
	}
	assertNoPrompt(t, r.out)

	if got := readRendered(t, filepath.Join(target, "orders.txt")); got != "source=orders\n" {
		t.Errorf("expected render from source dir, got %q", got)
	}
	if eff, _ := effectiveJSON(t, r.out); eff["app_name"] != "orders" {
		t.Errorf("unexpected effective values: %v", eff)
	}
}

func TestUseFromWithoutRegistry(t *testing.T) {
	home, work := t.TempDir(), t.TempDir() // no registry initialized
	src := filepath.Join(work, "svc")
	writeTemplate(t, src, useProjectJSON, useTemplateFile)
	target := filepath.Join(work, "out")

	r := boilr(t, home, "template", "use", "--from", src, target, "-f")
	if r.code != 0 {
		t.Fatalf("exit %d, output:\n%s", r.code, r.out)
	}
	readRendered(t, filepath.Join(target, "myservice.txt"))
}

func TestUseFromErrors(t *testing.T) {
	t.Run("missing template dir", func(t *testing.T) {
		home, work := setup(t)
		src := filepath.Join(work, "src")
		writeFile(t, filepath.Join(work, "project.json"), useProjectJSON)
		if err := os.MkdirAll(src, 0755); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(work, "out")

		r := boilr(t, home, "template", "use", "--from", src, target)
		if r.code == 0 {
			t.Fatalf("expected non-zero exit, output:\n%s", r.out)
		}
		assertNotExists(t, target)
	})

	t.Run("extra positional argument", func(t *testing.T) {
		home, work := setup(t)
		src := filepath.Join(work, "svc")
		writeTemplate(t, src, useProjectJSON, useTemplateFile)
		target := filepath.Join(work, "out")

		r := boilr(t, home, "template", "use", "--from", src, "svc", target)
		if r.code == 0 {
			t.Fatalf("expected non-zero exit, output:\n%s", r.out)
		}
		if !strings.Contains(r.out, "unexpected arguments") {
			t.Errorf("expected argument error, got:\n%s", r.out)
		}
		assertNotExists(t, target)
	})
}

func ptr(s string) *string { return &s }
