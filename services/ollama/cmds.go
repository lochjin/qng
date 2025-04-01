package ollama

import (
	"github.com/ollama/ollama/runner"
	"github.com/urfave/cli/v2"
	"os"
)

func Cmds() *cli.Command {
	return &cli.Command{
		Name:        "runner",
		Aliases:     []string{"r"},
		Category:    "Ollama",
		Usage:       "Ollama",
		Description: "Ollama",
		Action: func(ctx *cli.Context) error {
			return runner.Execute(os.Args[1:])
		},
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			return runner.Execute(os.Args[1:])
		},
	}
}
