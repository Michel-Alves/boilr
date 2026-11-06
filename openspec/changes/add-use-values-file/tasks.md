# Tasks

## 1. Template overrides (`pkg/template`)

- [x] 1.1 Add `OverrideValues(map[string]interface{}) error` and `EffectiveValues() map[string]interface{}` to `template.Interface` and implement them on `*dirTemplate` (validate on a deep copy, sorted keys, `errors.Join`, commit and set `ShouldUseDefaults = true` only on success); verify with `go build ./...`
- [x] 1.2 Implement the per-key rules in one recursive helper with a `jsonKind` classifier: unknown key, choice membership (error lists the options; a valid value is stored as a scalar), type mismatch including `null`, nested groups using a dotted key path; verify with the tests in 1.3
- [x] 1.3 Add `pkg/template/override_test.go`, using a temp template (string, number, bool, choice, nested group) and `Get` → `OverrideValues` → `Execute` with rendered-output checks, covering: string override; valid choice renders the chosen option rather than the first; invalid choice error lists the options; unknown key `app_nmae`; bool-as-string and number-as-string type errors; nested override plus an unknown nested key (`db.nmae`); missing key keeps the default; multiple bad keys reported together; a failed call leaves `Context` unchanged; verify that `go test ./pkg/template/...` passes
- [x] 1.4 Add an `EffectiveValues` test: a choice that isn't overridden shows its first item, an overridden one shows its value, and groups keep their structure; verify with `go test ./pkg/template/...`

## 2. `--values` flag on `use` (`pkg/cmd`)

- [x] 2.1 Register `Use.Flags().String("values", "", "JSON file whose values override project.json; implies non-interactive")` in `pkg/cmd/root.go` (local, not persistent, not on root); verify `boilr template use --help` lists it and `boilr --help` doesn't
- [x] 2.2 In `use.go`, after `template.Get` and the `use-defaults` block, read `--values` via `cmd.Flags().GetString`, read and `json.Unmarshal` the file into `map[string]interface{}`, call `tmpl.OverrideValues`, and send any error to `exit.Fatal` before `executeTemplate`; verify with the tests in 2.4
- [x] 2.3 After a successful `executeTemplate`, when `--values` was given, print `json.MarshalIndent(tmpl.EffectiveValues())` to stdout before `exit.OK`; verify with the tests in 2.4
- [x] 2.4 Replace the skipped test in `pkg/cmd/use_test.go` with subprocess tests (a `TestMain` re-exec guarded by an env var, `HOME` set to a temp dir, a template saved into that registry, `Stdin` = `/dev/null`, a context timeout). Cover: a successful run with `--values` renders the overrides and prints the effective JSON before the success line; `-f` plus `--values` works; an unknown key exits non-zero with `unknown key "app_nmae"` and no target dir; a missing file, invalid JSON and a non-object top level each exit non-zero; closed stdin completes within the timeout with no `[?]` prompt in the output; a run without `--values` prints no JSON. Verify that `go test ./pkg/cmd/...` passes
- [x] 2.5 Document `--values` (format, strict validation, evidence output, an example) in `README.md` and `wiki/Usage.md`, and fix the unclosed code fence in `wiki/Usage.md`; verify that the documented command runs as written against the test template

## 3. `--from` flag on `use` (optional; can be deferred without affecting groups 1–2)

- [x] 3.1 Register `Use.Flags().String("from", "", "Run the template from this source directory instead of the registry")` in `root.go`; verify with `boilr template use --help`
- [x] 3.2 In `use.go`, when `--from` is set, validate exactly one `target-dir` argument, skip the `MustValidateTemplateDir` and registry lookup, call `MustValidateTemplate(dir)`, then `template.Get(dir)`, and use `dir` in the success message; leave the remaining flow unchanged; verify with the tests in 3.3
- [x] 3.3 Add subprocess tests: `--from <dir> <target> --values v.json` renders from the source directory even when a registry entry with the same name differs; a `--from` dir without `template/` exits non-zero; `--from <dir> tag target` (two positional args) exits non-zero; verify that `go test ./pkg/cmd/...` passes
- [x] 3.4 Document `--from` and the contract form `boilr template use --from <template_source> {repo_path} --values {values_file}` in `README.md` and `wiki/Usage.md`; verify that the documented command runs as written

## 4. Integration check with the real template

- [x] 4.1 Run `go vet ./...` and `go test ./...`; verify that both pass
- [x] 4.2 Run `go install .` (overwrites `~/go/bin/boilr`) and confirm `which boilr` resolves to `~/go/bin/boilr`
- [x] 4.3 Run `echo '{"app_name":"orders","stack_profile":"relational-db","tld":"org"}' > /tmp/v.json && boilr template use kotlin-microservice /tmp/orders --values /tmp/v.json </dev/null`; verify exit 0, that the effective-values JSON is printed, and that `! grep -r myservice /tmp/orders` succeeds
- [x] 4.4 Run `echo '{"app_nmae":"x"}' > /tmp/bad.json && boilr template use kotlin-microservice /tmp/x --values /tmp/bad.json`; verify a non-zero exit, an error containing `unknown key "app_nmae"`, and that `/tmp/x` doesn't exist
- [x] 4.5 If group 3 was implemented, repeat 4.3 with `--from <path to projects_template/kotlin-microservice>` in place of the tag; verify the same results
