package template

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testProjectJSON = `{
  "app_name": "myservice",
  "port": 8080,
  "use_kafka": false,
  "stack_profile": ["document-db", "relational-db"],
  "db": {"name": "app", "db_port": 5432}
}`

const testTemplateFile = `app={{app_name}}
port={{port}}
kafka={{use_kafka}}
stack={{stack_profile}}
db_name={{name}}
`

func newTestTemplate(t *testing.T) *dirTemplate {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project.json"), []byte(testProjectJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "template"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "template", "{{app_name}}.txt"), []byte(testTemplateFile), 0644); err != nil {
		t.Fatal(err)
	}

	tmpl, err := Get(dir)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	return tmpl.(*dirTemplate)
}

func render(t *testing.T, tmpl *dirTemplate, fileName string) string {
	t.Helper()

	out := t.TempDir()
	if err := tmpl.Execute(out); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(out, fileName))
	if err != nil {
		t.Fatalf("expected rendered file %q: %v", fileName, err)
	}

	return string(b)
}

func TestOverrideValuesApplies(t *testing.T) {
	tmpl := newTestTemplate(t)

	err := tmpl.OverrideValues(map[string]interface{}{
		"app_name":      "orders",
		"stack_profile": "relational-db",
		"db":            map[string]interface{}{"name": "ordersdb"},
	})
	if err != nil {
		t.Fatalf("OverrideValues() error: %v", err)
	}
	if !tmpl.ShouldUseDefaults {
		t.Error("OverrideValues() should enable default (non-interactive) binding")
	}

	got := render(t, tmpl, "orders.txt")
	for _, want := range []string{
		"app=orders\n",
		"port=8080\n",           // missing key keeps default
		"kafka=false\n",         // missing key keeps default
		"stack=relational-db\n", // chosen option, not first item
		"db_name=ordersdb\n",    // nested override
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output missing %q, got:\n%s", want, got)
		}
	}

	db := tmpl.Context["db"].(map[string]interface{})
	if db["db_port"] != float64(5432) {
		t.Errorf("nested missing key should keep default 5432, got %v", db["db_port"])
	}
}

func TestOverrideValuesEmptyKeepsDefaults(t *testing.T) {
	tmpl := newTestTemplate(t)

	if err := tmpl.OverrideValues(map[string]interface{}{}); err != nil {
		t.Fatalf("OverrideValues({}) error: %v", err)
	}

	got := render(t, tmpl, "myservice.txt")
	if !strings.Contains(got, "stack=document-db\n") {
		t.Errorf("expected first choice by default, got:\n%s", got)
	}
}

func TestOverrideValuesErrors(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]interface{}
		want   []string
	}{
		{"unknown key", map[string]interface{}{"app_nmae": "x"}, []string{`unknown key "app_nmae"`}},
		{"invalid choice", map[string]interface{}{"stack_profile": "graph-db"}, []string{
			`invalid option for "stack_profile"`, "document-db", "relational-db",
		}},
		{"bool as string", map[string]interface{}{"use_kafka": "true"}, []string{
			`type mismatch for "use_kafka": expected boolean, got string`,
		}},
		{"number as string", map[string]interface{}{"port": "9090"}, []string{
			`type mismatch for "port": expected number, got string`,
		}},
		{"null value", map[string]interface{}{"app_name": nil}, []string{
			`type mismatch for "app_name": expected string, got null`,
		}},
		{"group not object", map[string]interface{}{"db": "x"}, []string{
			`type mismatch for "db": expected object, got string`,
		}},
		{"unknown nested key", map[string]interface{}{"db": map[string]interface{}{"nmae": "orders"}}, []string{
			`unknown key "db.nmae"`,
		}},
		{"nested type mismatch", map[string]interface{}{"db": map[string]interface{}{"db_port": true}}, []string{
			`type mismatch for "db.db_port": expected number, got boolean`,
		}},
		{"multiple bad keys", map[string]interface{}{"app_nmae": "x", "port": "1", "stack_profile": "y"}, []string{
			`unknown key "app_nmae"`, `"port"`, `"stack_profile"`,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := newTestTemplate(t)
			before := deepCopy(tmpl.Context)

			err := tmpl.OverrideValues(tt.values)
			if err == nil {
				t.Fatal("OverrideValues() expected error, got nil")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q should contain %q", err, want)
				}
			}

			if !reflect.DeepEqual(before, tmpl.Context) {
				t.Errorf("failed OverrideValues() changed Context: %v", tmpl.Context)
			}
			if tmpl.ShouldUseDefaults {
				t.Error("failed OverrideValues() should not enable defaults")
			}
		})
	}
}

func TestOverrideValuesNoContext(t *testing.T) {
	tmpl := &dirTemplate{}

	err := tmpl.OverrideValues(map[string]interface{}{"x": 1.0})
	if err == nil || !strings.Contains(err.Error(), `unknown key "x"`) {
		t.Errorf("expected unknown key error, got %v", err)
	}
}

func TestEffectiveValues(t *testing.T) {
	tmpl := newTestTemplate(t)
	tmpl.Context["tld"] = []interface{}{"com", "org"}

	if err := tmpl.OverrideValues(map[string]interface{}{"stack_profile": "relational-db"}); err != nil {
		t.Fatal(err)
	}

	want := map[string]interface{}{
		"app_name":      "myservice",
		"port":          float64(8080),
		"use_kafka":     false,
		"stack_profile": "relational-db",
		"tld":           "com",
		"db":            map[string]interface{}{"name": "app", "db_port": float64(5432)},
	}

	if got := tmpl.EffectiveValues(); !reflect.DeepEqual(got, want) {
		t.Errorf("EffectiveValues() = %v, want %v", got, want)
	}

	if _, ok := tmpl.Context["tld"].([]interface{}); !ok {
		t.Error("EffectiveValues() must not mutate Context")
	}
}
