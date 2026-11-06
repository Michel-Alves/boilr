# Design

## Context

See proposal.md (Why) for the motivation. This section covers only the current code that shapes the approach.

- `template.Get(path)` loads `project.json` into `dirTemplate.Context` (`map[string]interface{}`) using plain `encoding/json`. Every JSON number therefore arrives as `float64`, a choice variable as `[]interface{}`, and an advanced-mode group as `map[string]interface{}`.
- `BindPrompts()` calls `handleBindDefaults` when `ShouldUseDefaults` is true and `handleBindPrompts` otherwise. `handleBindDefaults` returns `val[0]` for a list and the value itself for anything else. For a group, it binds the parent function to `false` and binds each child key into the flat `FuncMap`. So writing a scalar into `Context` (or into a child map) is enough to change what gets rendered, and no prompt code runs.
- With closed stdin, the prompt path doesn't hang: `scanLine` gets EOF and the default is used. The run silently produces default values, which is the risk this change addresses.
- `use.go` validates exactly two positional args (`template-tag`, `target-dir`), requires an initialized registry, and resolves the template via `boilr.TemplatePath`. The existing `use` flags are registered in `root.go` as **persistent** flags and read through `GetBoolFlag`/`GetStringFlag`, which only look at `PersistentFlags()`.
- `exit.Fatal` and `exit.OK` call `os.Exit`, and `tlog` writes to stdout. `pkg/cmd/use_test.go` exists but calls `t.SkipNow()`.
- `util.ValidateTemplate(dir)` (already used by `save`) checks the `template/` subdirectory and dry-runs the template with defaults.

## Goals / Non-Goals

**Goals:**
- Check every override before any file is written, and report every bad key in one run.
- Keep the template's execution path unchanged. Overrides only rewrite `Context`, and generation then uses the existing defaults-binding path.
- Print evidence output that a script can capture.

**Non-Goals:**
- Supporting values-file formats other than JSON (YAML, TOML).
- Converting types (e.g. `"true"` → `true`). Mismatches are errors on purpose.
- Resolving `path@commit` inside boilr. With `--from`, the caller checks out the commit. boilr only reads a directory.
- Changing the behavior when the new flags aren't given.

## Decisions

**1. Local flags on `Use`, read with `cmd.Flags()`.**
As requested, `--values` and `--from` are registered as local flags (`Use.Flags().String(...)`) in `root.go`, next to the existing `Use` flags. They're read with `cmd.Flags().GetString(name)`, because the existing `GetStringFlag` helper only looks at persistent flags. *Alternative:* make them persistent for consistency. Rejected: `use` has no subcommands, so a persistent flag gains nothing, and the request names local flags.

**2. `OverrideValues` validates a copy, then commits.**
`OverrideValues(values map[string]interface{}) error` is added to `template.Interface`, implemented on `*dirTemplate`. It deep-copies `Context` and applies each key to the copy. Problems are collected as `key: message` errors, using a dotted path for nested keys. The method iterates over keys in sorted order so the output is deterministic, and returns `errors.Join(...)`. Only when there are no errors does it replace `t.Context` and set `t.ShouldUseDefaults = true`. A failed call leaves the template untouched. *Alternative:* fail on the first error. Rejected because a values file with several typos would take several runs to fix.

Per-key rules, applied by one recursive helper `override(dst map, src map, path string) []error`:
- The key isn't in `dst` → `unknown key "<path>"`.
- The default is `[]interface{}` (a choice) → the value must `reflect.DeepEqual` one of the items. Otherwise the error is `invalid option for "<path>": got X, valid options: [a, b]`. On a match, the scalar is stored.
- The default is `map[string]interface{}` (a group) → the value must also be a `map[string]interface{}`, and the helper recurses into it.
- Otherwise, compare `jsonKind(default)` with `jsonKind(value)`, where the kinds are `boolean`, `string`, `number`, `object`, `list` and `null`. On a mismatch: `type mismatch for "<path>": expected boolean, got string`.

If the template has no `project.json` (a nil `Context`), every key is unknown. An empty `{}` file is valid and is the same as `-f`.

**3. Effective values come from a new `EffectiveValues()` method.**
`EffectiveValues() map[string]interface{}` is added to `template.Interface`. It returns a deep copy of `Context` in which every list becomes its first item, recursing into groups. This matches what `handleBindDefaults` renders. `use.go` prints it with `json.MarshalIndent` through `fmt.Println` to stdout, after `executeTemplate()` succeeds and before `exit.OK`. *Alternative:* dump the raw `Context`. Rejected because choices that weren't overridden would print as lists, not as the values actually used, and that makes the evidence misleading.

**4. Wiring in `use.go`.**
After `template.Get` and the existing `use-defaults` handling: if `values != ""`, the command reads the file, unmarshals it into `map[string]interface{}` (a top level that isn't an object fails here), and calls `tmpl.OverrideValues`. Any error goes to `exit.Fatal(fmt.Errorf("use: %v", err))`. This happens before `executeTemplate`, so nothing is written on failure. `-f` combined with `--values` causes no conflict, because both end up setting `ShouldUseDefaults`.

**5. `--from` changes how the template path is resolved, and only that.**
With `--from <dir>`, the command validates a single `target-dir` arg and skips `MustValidateTemplateDir` and the registry lookup. It calls `MustValidateTemplate(dir)`, the same check `save` uses, and then `template.Get(dir)`. The success message names `dir`, not a tag. Everything after `template.Get` is shared with the registry path. *Alternative:* a separate `use-dir` command. Rejected because the contract command reads more naturally as one `use` with a flag, and splitting would duplicate the rest of the code.

**6. Command tests use a subprocess.**
`exit.*` calls `os.Exit`, so `pkg/cmd/use_test.go` re-executes the test binary. A `TestMain` checks an env var (e.g. `BOILR_TEST_RUN_MAIN=1`) and, when it's set, calls `cmd.Run()` with the given args. Each test:
- sets `HOME` to a `t.TempDir()`, so `boilr`'s `init()` points the registry there;
- writes a small template (`project.json` with a string, a choice, a bool and a group, plus `template/` files that use them) into that registry or into a `--from` dir;
- runs the child with `cmd.Stdin = nil` (`/dev/null`) and a `context.WithTimeout`. A timeout means a hang, and the test fails.
The existing skipped test is replaced. The validation rules themselves are unit-tested in `pkg/template` by calling `Get` + `OverrideValues` + `Execute` against a temp template, with no subprocess.

## Risks / Trade-offs

- [`errors.Join` output spans several lines and is printed through `tlog.Error`, which only colors the first line] → Acceptable: every line still names its key. The tests assert with `strings.Contains`.
- [Choice options that are numbers compare as `float64`, so `1` and `1.0` are equal and `"1"` isn't] → This matches JSON semantics. The error message lists the options, so the fix is obvious.
- [Child keys share the flat `FuncMap` namespace with top-level keys, which is existing behavior] → Out of scope. Overrides don't make this collision any worse.
- [Evidence JSON and the colored success line both go to stdout] → The JSON is printed first, as one indented block. Consumers can parse up to the first top-level closing `}`.
- [Callers outside this repo must update their command to `--from … --values …`] → Additive change. The old command form keeps working.

## Migration Plan

Additive. Build and install with `go install .`, which overwrites `~/go/bin/boilr`. To roll back, reinstall the previous build. No data or registry format changes.
