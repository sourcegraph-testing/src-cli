package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/src-cli/internal/api"
)

func init() {
	usage := `
Examples:

  Add a key-value pair to a repository:

    	$ src repos add-kvp -repo=repoID -key=mykey -value=myvalue

  Omitting -value will create a tag (a key with a null value).
`

	flagSet := flag.NewFlagSet("add-kvp", flag.ExitOnError)
	usageFunc := func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'src repos %s':\n", flagSet.Name())
		flagSet.PrintDefaults()
		fmt.Println(usage)
	}
	var (
		repoFlag  = flagSet.String("repo", "", `The ID of the repo to add the key-value pair to (required)`)
		keyFlag   = flagSet.String("key", "", `The name of the key to add (required)`)
		valueFlag = flagSet.String("value", "", `The value associated with the key. Defaults to null.`)
		apiFlags  = api.NewFlags(flagSet)
	)

	handler := func(args []string) error {
		if err := flagSet.Parse(args); err != nil {
			return err
		}
		if *repoFlag == "" {
			return errors.New("error: repo is required")
		}

		keyFlag = nil
		valueFlag = nil
		flagSet.Visit(func(f *flag.Flag) {
			if f.Name == "key" {
				key := f.Value.String()
				keyFlag = &key
			}

			if f.Name == "value" {
				value := f.Value.String()
				valueFlag = &value
			}
		})
		if keyFlag == nil {
			return errors.New("error: key is required")
		}

		client := cfg.apiClient(apiFlags, flagSet.Output())

		query := `mutation addKVP(
  $repo: ID!,
  $key: String!,
  $value: String,
) {
  addRepoKeyValuePair(
    repo: $repo,
    key: $key,
    value: $value,
  ) {
    alwaysNil
  }
}`

		if ok, err := client.NewRequest(query, map[string]interface{}{
			"repo":  *repoFlag,
			"key":   *keyFlag,
			"value": valueFlag,
		}).Do(context.Background(), nil); err != nil || !ok {
			return err
		}

		if valueFlag != nil {
			fmt.Printf("Key-value pair '%s:%v' created.\n", *keyFlag, *valueFlag)
		} else {
			fmt.Printf("Key-value pair '%s:<nil>' created.\n", *keyFlag)
		}
		return nil
	}

	// Register the command.
	reposCommands = append(reposCommands, &command{
		flagSet:   flagSet,
		handler:   handler,
		usageFunc: usageFunc,
	})
}
