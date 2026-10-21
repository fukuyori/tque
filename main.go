package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"tque/cmd"
	"tque/internal/tui"
)

//go:embed VERSION
var versionFile string

func main() {
	root := cmd.New(cmd.Options{Version: strings.TrimSpace(versionFile), RunTUI: tui.Run})
	if e := root.ExecuteContext(context.Background()); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(cmd.ExitCode(e))
	}
}
