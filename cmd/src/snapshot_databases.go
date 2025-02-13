package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
	"gopkg.in/yaml.v3"

	"github.com/sourcegraph/src-cli/internal/pgdump"
)

func init() {
	usage := `'src snapshot databases' generates commands to export Sourcegraph database dumps.
Note that these commands are intended for use as reference - you may need to adjust the commands for your deployment.

USAGE
	src [-v] snapshot databases <pg_dump|docker|kubectl> [--targets=<docker|k8s|"targets.yaml">]

TARGETS FILES
	Predefined targets are available based on default Sourcegraph configurations ('docker', 'k8s').
	Custom targets configuration can be provided in YAML format with '--targets=target.yaml', e.g.

		primary:
			target: ...   # the DSN of the database deployment, e.g. in docker, the name of the database container
			dbname: ...   # name of database
			username: ... # username for database access
			password: ... # password for database access - only include password if it is non-sensitive
		codeintel:
			# same as above
		codeinsights:
			# same as above

	See the pgdump.Targets type for more details.
`
	flagSet := flag.NewFlagSet("databases", flag.ExitOnError)
	targetsKeyFlag := flagSet.String("targets", "auto", "predefined targets ('docker' or 'k8s'), or a custom targets.yaml file")

	snapshotCommands = append(snapshotCommands, &command{
		flagSet: flagSet,
		handler: func(args []string) error {
			if err := flagSet.Parse(args); err != nil {
				return err
			}
			out := output.NewOutput(flagSet.Output(), output.OutputOpts{Verbose: *verbose})

			var builder string
			if len(args) > 0 {
				builder = args[0]
			}

			targetKey := "docker"
			var commandBuilder pgdump.CommandBuilder
			switch builder {
			case "pg_dump", "":
				targetKey = "local"
				commandBuilder = func(t pgdump.Target) (string, error) {
					cmd := pgdump.Command(t)
					if t.Target != "" {
						return fmt.Sprintf("%s --host=%s", cmd, t.Target), nil
					}
					return cmd, nil
				}
			case "docker":
				commandBuilder = func(t pgdump.Target) (string, error) {
					return fmt.Sprintf("docker exec -it %s sh -c '%s'", t.Target, pgdump.Command(t)), nil
				}
			case "kubectl":
				targetKey = "k8s"
				commandBuilder = func(t pgdump.Target) (string, error) {
					return fmt.Sprintf("kubectl exec -it %s -- bash -c '%s'", t.Target, pgdump.Command(t)), nil
				}
			default:
				return errors.Newf("unknown or invalid template type %q", builder)
			}
			if *targetsKeyFlag != "auto" {
				targetKey = *targetsKeyFlag
			}

			targets, ok := predefinedDatabaseDumpTargets[targetKey]
			if !ok {
				out.WriteLine(output.Emojif(output.EmojiInfo, "Using targets defined in targets file %q", targetKey))
				f, err := os.Open(targetKey)
				if err != nil {
					return errors.Wrapf(err, "invalid targets file %q", targetKey)
				}
				if err := yaml.NewDecoder(f).Decode(&targets); err != nil {
					return errors.Wrapf(err, "invalid targets file %q", targetKey)
				}
			} else {
				out.WriteLine(output.Emojif(output.EmojiInfo, "Using predefined targets for %s environments", targetKey))
			}

			commands, err := pgdump.BuildCommands(srcSnapshotDir, commandBuilder, targets)
			if err != nil {
				return errors.Wrap(err, "failed to build commands")
			}

			_ = os.MkdirAll(srcSnapshotDir, os.ModePerm)

			b := out.Block(output.Emoji(output.EmojiSuccess, "Run these commands to generate the required database dumps:"))
			b.Write("\n" + strings.Join(commands, "\n"))
			b.Close()

			out.WriteLine(output.Styledf(output.StyleSuggestion, "Note that you may need to do some additional setup, such as authentication, beforehand."))

			return nil
		},
		usageFunc: func() { fmt.Fprint(flag.CommandLine.Output(), usage) },
	})
}

// predefinedDatabaseDumpTargets is based on default Sourcegraph configurations.
var predefinedDatabaseDumpTargets = map[string]pgdump.Targets{
	"local": {
		Primary: pgdump.Target{
			DBName:   "sg",
			Username: "sg",
			Password: "sg",
		},
		CodeIntel: pgdump.Target{
			DBName:   "sg",
			Username: "sg",
			Password: "sg",
		},
		CodeInsights: pgdump.Target{
			DBName:   "postgres",
			Username: "postgres",
			Password: "password",
		},
	},
	"docker": { // based on deploy-sourcegraph-managed
		Primary: pgdump.Target{
			Target:   "pgsql",
			DBName:   "sg",
			Username: "sg",
			Password: "sg",
		},
		CodeIntel: pgdump.Target{
			Target:   "codeintel-db",
			DBName:   "sg",
			Username: "sg",
			Password: "sg",
		},
		CodeInsights: pgdump.Target{
			Target:   "codeinsights-db",
			DBName:   "postgres",
			Username: "postgres",
			Password: "password",
		},
	},
	"k8s": { // based on deploy-sourcegraph-helm
		Primary: pgdump.Target{
			Target:   "statefulset/pgsql",
			DBName:   "sg",
			Username: "sg",
			Password: "sg",
		},
		CodeIntel: pgdump.Target{
			Target:   "statefulset/codeintel-db",
			DBName:   "sg",
			Username: "sg",
			Password: "sg",
		},
		CodeInsights: pgdump.Target{
			Target:   "statefulset/codeinsights-db",
			DBName:   "postgres",
			Username: "postgres",
			Password: "password",
		},
	},
}
