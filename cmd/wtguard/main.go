package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cuongtranba/wtguard/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	app := cli.NewApp(cli.BuildInfo{Version: version, Commit: commit, Date: date})
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "wtguard:", err)
		log.SetFlags(0)
		os.Exit(1)
	}
}
