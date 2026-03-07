package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/agios-sh/agios/config"
	"github.com/agios-sh/agios/output"
)

// Run dispatches tasks subcommands.
func Run(args []string) {
	if len(args) == 0 {
		respondOverview()
		return
	}

	switch args[0] {
	case "list":
		doList(args[1:])
	case "get":
		doGet(args[1:])
	case "create":
		doCreate(args[1:])
	case "update":
		doUpdate(args[1:])
	case "comment":
		doComment(args[1:])
	case "status":
		respondStatus()
	case "help":
		respondHelp()
	case "peek":
		respondPeek()
	default:
		emitError("Unknown command: "+args[0], "UNKNOWN_COMMAND",
			"Run `agios tasks help` to see available commands",
		)
		os.Exit(1)
	}
}

// TasksStatus returns the status of the built-in tasks app for use in agios status.
func TasksStatus() (string, map[string]int) {
	cfg, projectRoot := loadConfigAndRoot()
	sources, err := resolveSources(cfg, projectRoot)
	if err != nil || len(sources) == 0 {
		return "info", nil
	}

	// Get summary from default source
	def := resolveDefault(sources, configDefault(cfg))
	if def == nil {
		return "info", nil
	}

	counts, err := def.Summary()
	if err != nil {
		return "info", nil
	}

	return "ok", counts
}

func loadConfigAndRoot() (*config.Config, string) {
	cwd, err := os.Getwd()
	if err != nil {
		return &config.Config{}, cwd
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return &config.Config{}, cwd
	}
	return cfg, filepath.Dir(cfg.Path)
}

func configDefault(cfg *config.Config) string {
	if cfg.Tasks != nil {
		return cfg.Tasks.Default
	}
	return ""
}

func emitResult(v map[string]any) {
	data, err := output.Process(v)
	if err != nil {
		enc := json.NewEncoder(os.Stdout)
		enc.Encode(v)
		return
	}
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

func emitError(msg, code string, help ...string) {
	result := map[string]any{
		"error": msg,
		"code":  code,
	}
	if len(help) > 0 {
		result["help"] = help
	} else {
		result["help"] = []string{"Run `agios tasks help` for usage information"}
	}
	emitResult(result)
}
