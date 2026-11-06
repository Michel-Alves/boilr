# Proposal

## Why

`boilr template use` can only run interactively (one stdin prompt per variable) or with `-f`, which takes every default from `project.json`. Automation such as the verification session and the walking-skeleton can't run a template with *chosen* values without driving the prompts. When stdin is closed, the prompts quietly fall back to the defaults, so a run can succeed with the wrong values (for example, a project still named `myservice`). A second risk: `use` only reads from the registry, and the registry copy can drift from the template's source directory (one copy was last saved 10 days earlier). The `template_source: path@commit` recorded in `target.md` therefore doesn't guarantee which template actually ran.

## What Changes

- New `--values <file>` flag, on `template use` only and not on the root command. It reads a JSON file whose values override `project.json` and implies non-interactive execution. Works together with `-f/--use-defaults`.
- The values are checked strictly against the template context before anything is generated. Each error names the key:
  - unknown key, which catches typos such as `app_nmae`;
  - a value for a choice (list) variable that isn't one of its options; the error lists the valid options;
  - a value whose type (bool / string / number) differs from the default's type;
  - nested objects (boilr's "advanced mode" groups) are checked key by key with the same rules.
- Keys missing from the values file keep their defaults.
- A valid value for a choice variable replaces the list with the chosen value, so the chosen option is used instead of the first item.
- With `--values`, the final values used are printed as JSON before the success message, so automation can record them as evidence.
- New `--from <dir>` flag on `template use` (optional part of this change): run a template straight from its source directory, without going through the registry. The positional arguments become `<target-dir>` only. This removes the drift between registry and source, and makes `boilr template use --from <template_source> {repo_path} --values {values_file}` a reliable contract command.
- Tests for each validation rule, for the defaults fallback, for nested keys, and for running with closed stdin (`</dev/null`) without hanging or prompting.

No breaking changes: without the new flags, `use` behaves exactly as before.

## Capabilities

### New Capabilities
- `template-use`: how `boilr template use` runs a template into a target directory. Covers where the template comes from (registry or a source directory) and how variable values are supplied (prompts, defaults, or a validated values file).

### Modified Capabilities
<!-- none: no existing specs in openspec/specs/ -->

## Impact

- `pkg/template/template.go`: new `OverrideValues(map[string]interface{}) error` on `template.Interface`/`dirTemplate`, plus a way to read the final context for printing.
- `pkg/cmd/use.go`: reads `--values` and `--from`; changes argument validation when `--from` is set.
- `pkg/cmd/root.go`: registers the new flags on `Use`.
- Tests: new `pkg/template/*_test.go` cases, and `pkg/cmd/use_test.go`, which is currently skipped.
- Docs: `README.md` usage section and `wiki/Usage.md`.
- No new dependencies. Callers outside this repo (verification session, walking-skeleton contract) get a new command form to adopt.
