// Command syncroom is the single Syncroom binary. It dispatches to a
// coordinator or client subcommand based on the first positional argument.
// Tasks 1-3 only wire up `serve` well enough to keep the binary buildable;
// participant subcommands are implemented in later tasks.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "syncroom:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return fmt.Errorf("missing command")
	}
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q (available: help)", args[0])
	}
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, "usage: syncroom <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Tasks 1-3 have shipped the module, store, and coordinator API.")
	fmt.Fprintln(w, "CLI subcommands (serve/room/attach/watch/...) land in tasks 4+.")
}
