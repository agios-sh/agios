package tasks

import (
	"os"
	"strings"
)

// sourceFlag returns " --source <name>" when src differs from the default source,
// so help hints point agents at the correct source.
func sourceFlag(src, def Source) string {
	if def == nil || src.Name() != def.Name() {
		return " --source " + src.Name()
	}
	return ""
}

// doList handles: agios tasks list [--status <s>] [--assignee <a>] [--source <src>]
func doList(args []string) {
	cfg, projectRoot := loadConfigAndRoot()
	sources, err := resolveSources(cfg, projectRoot)
	if err != nil {
		emitError("Failed to resolve sources: "+err.Error(), "SOURCE_ERROR")
		os.Exit(1)
	}

	def := resolveDefault(sources, configDefault(cfg))
	src, remaining, err := resolveSource(args, sources, def)
	if err != nil {
		emitError(err.Error(), "SOURCE_NOT_FOUND",
			"Run `agios tasks status` to see all sources",
		)
		os.Exit(1)
	}

	opts := ListOptions{}
	for i := 0; i < len(remaining); i++ {
		switch remaining[i] {
		case "--status":
			if i+1 < len(remaining) {
				i++
				opts.Status = remaining[i]
			}
		case "--assignee":
			if i+1 < len(remaining) {
				i++
				opts.Assignee = remaining[i]
			}
		}
	}

	tasks, err := src.List(opts)
	if err != nil {
		emitError("Failed to list tasks: "+err.Error(), "LIST_ERROR",
			"Run `agios tasks status` to check source health",
		)
		os.Exit(1)
	}

	if tasks == nil {
		tasks = []TaskSummary{}
	}

	sf := sourceFlag(src, def)
	var help []string
	if len(tasks) == 0 {
		help = []string{
			"Run `agios tasks create --title \"...\" --body \"...\"" + sf + "` to create a new task",
			"Run `agios tasks list --status closed" + sf + "` to check closed tasks",
		}
	} else {
		help = []string{
			"Run `agios tasks get <id>" + sf + "` to see full task details",
			"Run `agios tasks create --title \"...\" --body \"...\"" + sf + "` to create a new task",
		}
	}

	emitResult(map[string]any{
		"tasks":  tasks,
		"source": src.Name(),
		"help":   help,
	})
}

// doGet handles: agios tasks get <id> [--source <src>]
func doGet(args []string) {
	cfg, projectRoot := loadConfigAndRoot()
	sources, err := resolveSources(cfg, projectRoot)
	if err != nil {
		emitError("Failed to resolve sources: "+err.Error(), "SOURCE_ERROR")
		os.Exit(1)
	}

	def := resolveDefault(sources, configDefault(cfg))
	src, remaining, err := resolveSource(args, sources, def)
	if err != nil {
		emitError(err.Error(), "SOURCE_NOT_FOUND",
			"Run `agios tasks status` to see all sources",
		)
		os.Exit(1)
	}

	if len(remaining) < 1 {
		emitError("Task ID required", "INVALID_ARGS",
			"Usage: `agios tasks get <id> [--source <src>]`",
		)
		os.Exit(1)
	}

	id := remaining[0]
	task, err := src.Get(id)
	if err != nil {
		emitError("Failed to get task: "+err.Error(), "GET_ERROR",
			"Run `agios tasks list` to see available tasks",
		)
		os.Exit(1)
	}

	sf := sourceFlag(src, def)
	var statusHint string
	if task.Status == "closed" {
		statusHint = "Run `agios tasks update " + id + " --status open" + sf + "` to reopen this task"
	} else {
		statusHint = "Run `agios tasks update " + id + " --status closed" + sf + "` to close this task"
	}

	emitResult(map[string]any{
		"task":   task,
		"source": src.Name(),
		"help": []string{
			statusHint,
			"Run `agios tasks comment " + id + " \"...\"" + sf + "` to add a comment",
		},
	})
}

// doCreate handles: agios tasks create --title <t> --body <b> [--status <s>] [--assignee <a>] [--source <src>]
func doCreate(args []string) {
	cfg, projectRoot := loadConfigAndRoot()
	sources, err := resolveSources(cfg, projectRoot)
	if err != nil {
		emitError("Failed to resolve sources: "+err.Error(), "SOURCE_ERROR")
		os.Exit(1)
	}

	def := resolveDefault(sources, configDefault(cfg))
	src, remaining, err := resolveSource(args, sources, def)
	if err != nil {
		emitError(err.Error(), "SOURCE_NOT_FOUND",
			"Run `agios tasks status` to see all sources",
		)
		os.Exit(1)
	}

	opts := CreateOptions{}
	for i := 0; i < len(remaining); i++ {
		switch remaining[i] {
		case "--title":
			if i+1 < len(remaining) {
				i++
				opts.Title = remaining[i]
			}
		case "--body":
			if i+1 < len(remaining) {
				i++
				opts.Body = remaining[i]
			}
		case "--status":
			if i+1 < len(remaining) {
				i++
				opts.Status = remaining[i]
			}
		case "--assignee":
			if i+1 < len(remaining) {
				i++
				opts.Assignee = remaining[i]
			}
		}
	}

	if opts.Title == "" || opts.Body == "" {
		emitError("Title and body are required", "INVALID_ARGS",
			"Usage: `agios tasks create --title \"...\" --body \"...\" [--status open|closed] [--assignee <name>]`",
		)
		os.Exit(1)
	}

	task, err := src.Create(opts)
	if err != nil {
		emitError("Failed to create task: "+err.Error(), "CREATE_ERROR",
			"Run `agios tasks list` to see existing tasks",
		)
		os.Exit(1)
	}

	sf := sourceFlag(src, def)
	emitResult(map[string]any{
		"task":   task,
		"source": src.Name(),
		"help": []string{
			"Run `agios tasks get " + task.ID + sf + "` to see full details",
			"Run `agios tasks list" + sf + "` to see all tasks",
		},
	})
}

// doUpdate handles: agios tasks update <id> [--title <t>] [--status <s>] [--assignee <a>] [--body <b>] [--source <src>]
func doUpdate(args []string) {
	cfg, projectRoot := loadConfigAndRoot()
	sources, err := resolveSources(cfg, projectRoot)
	if err != nil {
		emitError("Failed to resolve sources: "+err.Error(), "SOURCE_ERROR")
		os.Exit(1)
	}

	def := resolveDefault(sources, configDefault(cfg))
	src, remaining, err := resolveSource(args, sources, def)
	if err != nil {
		emitError(err.Error(), "SOURCE_NOT_FOUND",
			"Run `agios tasks status` to see all sources",
		)
		os.Exit(1)
	}

	if len(remaining) < 1 {
		emitError("Task ID required", "INVALID_ARGS",
			"Usage: `agios tasks update <id> [--title \"...\"] [--status open|closed] [--assignee <name>]`",
		)
		os.Exit(1)
	}

	id := remaining[0]
	opts := UpdateOptions{}
	for i := 1; i < len(remaining); i++ {
		switch remaining[i] {
		case "--title":
			if i+1 < len(remaining) {
				i++
				opts.Title = &remaining[i]
			}
		case "--body":
			if i+1 < len(remaining) {
				i++
				opts.Body = &remaining[i]
			}
		case "--status":
			if i+1 < len(remaining) {
				i++
				opts.Status = &remaining[i]
			}
		case "--assignee":
			if i+1 < len(remaining) {
				i++
				opts.Assignee = &remaining[i]
			}
		}
	}

	if opts.Title == nil && opts.Body == nil && opts.Status == nil && opts.Assignee == nil {
		emitError("No fields to update", "INVALID_ARGS",
			"Usage: `agios tasks update <id> [--title \"...\"] [--status open|closed] [--assignee <name>] [--body \"...\"]`",
		)
		os.Exit(1)
	}

	task, err := src.Update(id, opts)
	if err != nil {
		emitError("Failed to update task: "+err.Error(), "UPDATE_ERROR",
			"Run `agios tasks get "+id+"` to verify the task exists",
		)
		os.Exit(1)
	}

	sf := sourceFlag(src, def)
	help := []string{
		"Run `agios tasks get " + id + sf + "` to see full details",
	}
	if task.Status == "closed" {
		help = append(help, "Run `agios tasks update "+id+" --status open"+sf+"` to reopen this task")
	} else {
		help = append(help, "Run `agios tasks update "+id+" --status closed"+sf+"` to close this task")
	}

	emitResult(map[string]any{
		"task":   task,
		"source": src.Name(),
		"help":   help,
	})
}

// doComment handles: agios tasks comment <id> <text> [--source <src>]
func doComment(args []string) {
	cfg, projectRoot := loadConfigAndRoot()
	sources, err := resolveSources(cfg, projectRoot)
	if err != nil {
		emitError("Failed to resolve sources: "+err.Error(), "SOURCE_ERROR")
		os.Exit(1)
	}

	def := resolveDefault(sources, configDefault(cfg))
	src, remaining, err := resolveSource(args, sources, def)
	if err != nil {
		emitError(err.Error(), "SOURCE_NOT_FOUND",
			"Run `agios tasks status` to see all sources",
		)
		os.Exit(1)
	}

	if len(remaining) < 2 {
		emitError("Task ID and comment text required", "INVALID_ARGS",
			"Usage: `agios tasks comment <id> \"comment text\" [--source <src>]`",
		)
		os.Exit(1)
	}

	id := remaining[0]
	body := strings.TrimSpace(strings.Join(remaining[1:], " "))
	if body == "" {
		emitError("Comment text cannot be empty", "INVALID_ARGS",
			"Usage: `agios tasks comment <id> \"comment text\" [--source <src>]`",
		)
		os.Exit(1)
	}

	task, err := src.Comment(id, body)
	if err != nil {
		emitError("Failed to add comment: "+err.Error(), "COMMENT_ERROR",
			"Run `agios tasks get "+id+"` to verify the task exists",
		)
		os.Exit(1)
	}

	sf := sourceFlag(src, def)
	emitResult(map[string]any{
		"task":   task,
		"source": src.Name(),
		"help": []string{
			"Run `agios tasks get " + id + sf + "` to see the full task with comments",
		},
	})
}
