package cli

import (
	_ "embed"
	"fmt"

	"github.com/urfave/cli/v2"
)

//go:embed explain.md
var explainDoc string

func cmdExplain() *cli.Command {
	return &cli.Command{
		Name:  "explain",
		Usage: "print a single self-contained reference for LLMs / humans",
		Description: "Dumps a markdown reference covering purpose, block rule, " +
			"every subcommand, config keys, env vars, and examples. Designed to " +
			"be pasted into an LLM context window in one shot.",
		Action: func(c *cli.Context) error {
			fmt.Print(explainDoc)
			return nil
		},
	}
}
