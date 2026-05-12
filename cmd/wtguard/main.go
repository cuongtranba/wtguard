package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cuongtranba/wtguard/internal/cli"
	"github.com/cuongtranba/wtguard/internal/proxy"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// argv[0] dispatch: when invoked as `git` (via the ~/.wtguard/bin/git
	// symlink), run the proxy. Otherwise run the regular CLI.
	if filepath.Base(os.Args[0]) == "git" {
		os.Exit(proxy.Run(os.Args[1:]))
	}
	app := cli.NewApp(cli.BuildInfo{Version: version, Commit: commit, Date: date})
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "wtguard:", err)
		log.SetFlags(0)
		os.Exit(1)
	}
}
