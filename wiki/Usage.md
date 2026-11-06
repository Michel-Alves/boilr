# Usage
Use `boilr help` to get the list of available commands.

## Download Template
In order to download a template from a github repository, use the following command:

```bash
boilr template download <github-repo-path> <template-name>
boilr template download tmrts/boilr-license license
```

The downloaded template will be saved to local `boilr` registry.

## Save Local Template
In order to save a template from filesystem to the template registry use the following command:

```bash
boilr template save <template-path> <template-name>
boilr template save ~/boilr-license license
```

The saved template will be saved to local `boilr` registry.

## Use Template
In order to use a template from template registry use the following command:

```bash
boilr template use <template-name> <target-dir>
boilr template use license ~/Workspace/example-project/
```

You will be prompted for values when using a template.

## Non-interactive use with a values file
`--values <file>` supplies variable values from a JSON object instead of prompting.
Keys in the file override the defaults in `project.json`. Keys left out keep their defaults.
No prompt is shown and stdin is never read, so the command is safe in scripts and CI
(with or without `-f`).

```bash
echo '{"Author": "Jane Doe", "License": "MIT"}' > values.json
boilr template use license /workspace/example-project --values values.json </dev/null
```

The values are validated against `project.json` before anything is generated, and
every problem is reported in one run:

* an unknown key (e.g. a typo such as `Athor`) is rejected: `unknown key "Athor"`;
* a value for a choice (list) variable must be one of its options; the error lists them;
* a value must have the same JSON type as its default (boolean, string, number or object),
  so `"true"` for a boolean or `null` is an error, not converted;
* nested objects (advanced-mode groups) are checked key by key, and errors name the
  full path, such as `db.name`.

After a successful run, the effective values (choices resolved to the selected option)
are printed as one indented JSON object before the success line, so automation can
record what was used:

```bash
{
  "Author": "Jane Doe",
  "License": "MIT",
  "Year": "2015"
}
[✔] Successfully executed the project template license in /workspace/example-project
```

## Use a template from its source directory
`--from <template-dir>` runs a template straight from a directory containing
`project.json` and `template/`, bypassing the registry (which doesn't need to be
initialized). The only positional argument is then the target directory:

```bash
boilr template use --from <template-dir> <target-dir> [--values <file>]
boilr template use --from ~/boilr-license /workspace/example-project --values values.json
```

This guarantees the files come from the source as it is checked out, even if a registry
copy with the same name is older. For a reproducible, scripted run use:

```bash
boilr template use --from <template_source> {repo_path} --values {values_file}
```
