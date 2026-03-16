package main

import (
	"os"

	"github.com/agios-sh/agios/browser"
	"github.com/agios-sh/agios/cmd"
	"github.com/agios-sh/agios/tasks"
	"github.com/agios-sh/agios/terminal"
	"github.com/agios-sh/agios/updater"
)

var version = "dev"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		cmd.RunHome(version)
		return
	}

	switch args[0] {
	case "--version":
		cmd.RunVersion(version)
	case "--help", "-h", "help":
		cmd.RunHelp()
	case "--update-check":
		updater.RunBackgroundCheck(args[1:])
	case "--terminal-server":
		terminal.RunServer()
	case "init":
		cmd.RunInit()
	case "add":
		cmd.RunAdd(args[1:])
	case "remove":
		cmd.RunRemove(args[1:])
	case "status":
		cmd.RunStatus()
	case "jobs":
		cmd.RunJobs(args[1:])
	case "browser":
		browser.Run(args[1:])
	case "terminal":
		terminal.Run(args[1:])
	case "tasks":
		tasks.Run(args[1:])
	case "update":
		cmd.RunUpdate(args[1:], version)
	default:
		cmd.RunApp(args[0], args[1:])
	}
}
