// Command cerebro is a ChatGPT-like terminal interface for local LLMs.
package main

import (
	"flag"
	"fmt"
	"os"

	"cerebro/internal/app"
	"cerebro/internal/paths"
)

func main() {
	var (
		doSetup  = flag.Bool("setup", false, "create the dedicated venv and install MLX runtimes, then exit")
		showPath = flag.Bool("config", false, "print configuration and data paths, then exit")
		version  = flag.Bool("version", false, "print version and exit")
	)
	flag.Usage = usage
	flag.Parse()

	switch {
	case *version:
		fmt.Println("cerebro 0.1.0")
		return

	case *showPath:
		fmt.Printf("config file:    %s\n", paths.ConfigFile())
		fmt.Printf("header prompts: %s\n", paths.HeaderPromptsFile())
		fmt.Printf("history dir:    %s\n", paths.HistoryDir())
		fmt.Printf("venv:           %s\n", paths.VenvDir())
		fmt.Printf("server log:     %s\n", paths.ServerLogPath())
		return

	case *doSetup:
		if err := app.Setup(); err != nil {
			fmt.Fprintln(os.Stderr, "setup failed:", err)
			os.Exit(1)
		}
		return
	}

	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "cerebro:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `cerebro — a terminal chat UI for local LLMs

Usage:
  cerebro            launch the chat interface
  cerebro --setup    create the venv and install MLX runtimes (one-time)
  cerebro --config   print config/data file locations
  cerebro --version  print version

Inside the app, type /help for commands.`)
}
