package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	cli "github.com/spf13/cobra"

	"github.com/Ilyes512/boilr/pkg/boilr"
	"github.com/Ilyes512/boilr/pkg/template"
	"github.com/Ilyes512/boilr/pkg/util/exit"
	"github.com/Ilyes512/boilr/pkg/util/osutil"
	"github.com/Ilyes512/boilr/pkg/util/tlog"
	"github.com/Ilyes512/boilr/pkg/util/validate"
)

// TemplateInRegistry checks whether the given name exists in the template registry.
func TemplateInRegistry(name string) (bool, error) {
	names, err := ListTemplates()
	if err != nil {
		return false, err
	}

	_, ok := names[name]
	return ok, nil
}

// readValuesFile reads a JSON file whose top-level value must be an object.
func readValuesFile(path string) (map[string]interface{}, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("couldn't read values file %q: %v", path, err)
	}

	var raw interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("values file %q is not valid JSON: %v", path, err)
	}

	values, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("values file %q must contain a JSON object at the top level", path)
	}

	return values, nil
}

// Use contains the cli-command for using templates located in the local template registry.
// TODO add --use-cache flag to execute a template from previous answers to prompts
var Use = &cli.Command{
	Use:   "use <template-tag> <target-dir>",
	Short: "Execute a project template in the given directory",
	Long: `Execute a project template in the given directory.

With --from <template-dir>, the template is read from that source directory
instead of the registry and the only positional argument is <target-dir>.

With --values <file>, the JSON object in <file> overrides the defaults in
project.json. Values are validated strictly, nothing is prompted, and the
effective values are printed as JSON before the success message.`,
	Run: func(cmd *cli.Command, args []string) {
		fromDir, err := cmd.Flags().GetString("from")
		if err != nil {
			exit.Fatal(fmt.Errorf("use: %s", err))
		}

		valuesFile, err := cmd.Flags().GetString("values")
		if err != nil {
			exit.Fatal(fmt.Errorf("use: %s", err))
		}

		var tmplName, tmplPath, targetArg string

		if fromDir != "" {
			MustValidateArgs(args, []validate.Argument{
				{Name: "target-dir", Validate: validate.UnixPath},
			})

			tlog.SetLogLevel(GetStringFlag(cmd, "log-level"))

			MustValidateTemplate(fromDir)

			tmplName, tmplPath, targetArg = fromDir, fromDir, args[0]
		} else {
			MustValidateArgs(args, []validate.Argument{
				{Name: "template-tag", Validate: validate.UnixPath},
				{Name: "target-dir", Validate: validate.UnixPath},
			})

			MustValidateTemplateDir()

			tlog.SetLogLevel(GetStringFlag(cmd, "log-level"))

			tmplName, targetArg = args[0], args[1]

			templateFound, err := TemplateInRegistry(tmplName)
			if err != nil {
				exit.Fatal(fmt.Errorf("use: %s", err))
			}

			if !templateFound {
				exit.Fatal(fmt.Errorf("template %q couldn't be found in the template registry", tmplName))
			}

			tmplPath, err = boilr.TemplatePath(tmplName)
			if err != nil {
				exit.Fatal(fmt.Errorf("use: %s", err))
			}
		}

		targetDir, err := filepath.Abs(targetArg)
		if err != nil {
			exit.Fatal(fmt.Errorf("use: %s", err))
		}

		tmpl, err := template.Get(tmplPath)
		if err != nil {
			exit.Fatal(fmt.Errorf("use: %s", err))
		}

		if shouldUseDefaults := GetBoolFlag(cmd, "use-defaults"); shouldUseDefaults {
			tmpl.UseDefaultValues()
		}

		if valuesFile != "" {
			values, err := readValuesFile(valuesFile)
			if err != nil {
				exit.Fatal(fmt.Errorf("use: %v", err))
			}

			if err := tmpl.OverrideValues(values); err != nil {
				exit.Fatal(fmt.Errorf("use: invalid values in %q:\n%v", valuesFile, err))
			}
		}

		executeTemplate := func() error {
			parentDir := filepath.Dir(targetDir)

			exists, err := osutil.DirExists(parentDir)
			if err != nil {
				return err
			}

			if !exists {
				return fmt.Errorf("use: parent directory %q doesn't exist", parentDir)
			}

			tmpDir, err := os.MkdirTemp("", "boilr-use-template")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			if err := tmpl.Execute(tmpDir); err != nil {
				return err
			}

			// Complete the template execution transaction by copying the temporary dir to the target directory.
			return osutil.CopyRecursively(tmpDir, targetDir)
		}

		if err := executeTemplate(); err != nil {
			exit.Fatal(fmt.Errorf("use: %v", err))
		}

		if valuesFile != "" {
			b, err := json.MarshalIndent(tmpl.EffectiveValues(), "", "  ")
			if err != nil {
				exit.Fatal(fmt.Errorf("use: %v", err))
			}

			fmt.Println(string(b))
		}

		exit.OK("Successfully executed the project template %v in %v", tmplName, targetDir)
	},
}
