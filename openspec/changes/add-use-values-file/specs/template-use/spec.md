# Spec Delta

## Purpose

Defines how `boilr template use` runs a template into a target directory: where the template comes from (the registry or a source directory) and how its variable values are supplied (interactive prompts, defaults, or a validated values file).

## ADDED Requirements

### Requirement: Values file option
`boilr template use` SHALL accept a `--values <file>` option naming a JSON file whose top-level value is an object. The option SHALL exist only on `template use`. If the file can't be read, isn't valid JSON, or its top level isn't an object, the command SHALL exit with a non-zero status, print an error, and create nothing in the target directory.

#### Scenario: Unreadable values file
- **WHEN** the user runs `boilr template use <tag> <target> --values /missing.json`
- **THEN** the command exits non-zero with an error naming the file, and `<target>` is not created

#### Scenario: Values file is not a JSON object
- **WHEN** the values file contains `["orders"]`
- **THEN** the command exits non-zero with an error, and nothing is generated

### Requirement: Values override template defaults
For each key in the values file that matches a top-level variable in the template's `project.json`, the command SHALL use the file's value instead of the default. Variables not present in the file SHALL keep their defaults.

#### Scenario: Override a string variable
- **WHEN** `project.json` has `"app_name": "myservice"` and the values file has `{"app_name": "orders"}`
- **THEN** every use of `app_name` in the generated files and file names renders `orders`

#### Scenario: Missing key falls back to default
- **WHEN** `project.json` has `"tld": "com"` and the values file doesn't contain `tld`
- **THEN** `tld` renders `com`

### Requirement: Values file implies non-interactive execution
With `--values`, the command SHALL NOT prompt and SHALL NOT read standard input, whether or not `-f/--use-defaults` is also given.

#### Scenario: Closed stdin
- **WHEN** the user runs `boilr template use <tag> <target> --values v.json </dev/null`
- **THEN** the command finishes without hanging, prints no prompt, and exits zero

#### Scenario: Combined with use-defaults
- **WHEN** the user passes both `-f` and `--values v.json`
- **THEN** the command runs as if only `--values v.json` were given

### Requirement: Unknown keys are rejected
If the values file has a key that isn't a variable in `project.json` at the same nesting level, the command SHALL exit non-zero with an error naming the key, and generate nothing.

#### Scenario: Typo in key
- **WHEN** the values file is `{"app_nmae": "x"}` and `project.json` has no `app_nmae`
- **THEN** the command fails with an error containing `unknown key "app_nmae"` and the target directory is not created

### Requirement: Choice values must be a listed option
For a variable whose default is a list of options, the supplied value SHALL equal one of the listed items. A matching value SHALL be used in place of the first item. A non-matching value SHALL make the command exit non-zero with an error naming the key and listing the valid options.

#### Scenario: Valid option chosen
- **WHEN** `project.json` has `"stack_profile": ["document-db", "relational-db"]` and the values file has `{"stack_profile": "relational-db"}`
- **THEN** `stack_profile` renders `relational-db`

#### Scenario: Invalid option
- **WHEN** the values file has `{"stack_profile": "graph-db"}`
- **THEN** the command fails with an error naming `stack_profile` and listing `document-db` and `relational-db`

### Requirement: Value types must match defaults
For a non-choice variable, the supplied value SHALL have the same JSON type as the default: boolean, string, number, or object. On a mismatch, including `null`, the command SHALL exit non-zero with an error naming the key and both types.

#### Scenario: Boolean given as string
- **WHEN** `project.json` has `"use_kafka": false` and the values file has `{"use_kafka": "true"}`
- **THEN** the command fails with an error naming `use_kafka` and stating that a boolean was expected

#### Scenario: Number given as string
- **WHEN** `project.json` has `"port": 8080` and the values file has `{"port": "9090"}`
- **THEN** the command fails with an error naming `port`

### Requirement: Nested variable groups
For a variable whose default is an object (an advanced-mode group), the supplied value SHALL be an object. Its child keys SHALL be checked and applied with the same rules as top-level keys, and errors SHALL name the full key path. Child keys missing from the file SHALL keep their defaults. No group prompt SHALL be shown.

#### Scenario: Override nested key
- **WHEN** `project.json` has `"db": {"name": "app", "port": 5432}` and the values file has `{"db": {"name": "orders"}}`
- **THEN** `name` renders `orders` and `port` renders `5432`

#### Scenario: Unknown nested key
- **WHEN** the values file has `{"db": {"nmae": "orders"}}`
- **THEN** the command fails with an error naming `db.nmae`

### Requirement: Effective values are reported
After a successful run with `--values`, the command SHALL print to standard output, before the success message, one JSON object with the value used for every variable. Choice variables SHALL show the selected option, not the list. Nested groups SHALL keep their structure.

#### Scenario: Evidence output
- **WHEN** the run with `--values` `{"app_name": "orders"}` succeeds on a template with `"app_name": "myservice"` and `"tld": ["com", "org"]`
- **THEN** stdout contains a JSON object with `"app_name": "orders"` and `"tld": "com"`, followed by the success line

#### Scenario: No values flag
- **WHEN** the user runs `use` without `--values`
- **THEN** no values JSON is printed, and output is unchanged from before

### Requirement: Run template from a source directory
`boilr template use` SHALL accept `--from <dir>` naming a template directory: `project.json` plus a `template/` subdirectory. With `--from`, the command SHALL take exactly one positional argument, `<target-dir>`, SHALL run the template from `<dir>` without reading the registry, and SHALL NOT require the registry to be initialized. An invalid `<dir>` SHALL make the command exit non-zero.

#### Scenario: Run from source
- **WHEN** the user runs `boilr template use --from ./projects_template/kotlin-microservice /tmp/orders --values v.json`
- **THEN** the files are generated from the source directory's current contents, even if a registry copy with the same name differs

#### Scenario: Invalid source directory
- **WHEN** `--from` names a directory without a `template/` subdirectory
- **THEN** the command exits non-zero with an error, and nothing is generated

#### Scenario: Extra positional argument
- **WHEN** the user runs `boilr template use --from <dir> kotlin-microservice /tmp/orders`
- **THEN** the command exits non-zero with an argument error
