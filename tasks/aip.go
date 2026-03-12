package tasks

import (
	"strings"
	"sync"
	"time"
)

// respondOverview shows aggregated task counts + recent tasks from ALL sources.
// This is the dock view: `agios tasks` with no arguments.
func respondOverview() {
	cfg, projectRoot := loadConfigAndRoot()
	sources, err := resolveSources(cfg, projectRoot)
	if err != nil {
		emitError("Failed to resolve sources: "+err.Error(), "SOURCE_ERROR")
		return
	}

	if len(sources) == 0 {
		emitResult(map[string]any{
			"sources": []any{},
			"help": []string{
				"No task sources available",
				"Run `agios tasks create --title \"...\" --body \"...\"` to create a task (will auto-create local source)",
			},
		})
		return
	}

	type sourceInfo struct {
		Name    string         `json:"name"`
		Type    string         `json:"type"`
		Default bool           `json:"default,omitempty"`
		Tasks   map[string]int `json:"tasks"`
	}

	def := resolveDefault(sources, configDefault(cfg))

	var mu sync.Mutex
	sourceInfos := make([]sourceInfo, len(sources))
	var allRecent []TaskSummary

	var wg sync.WaitGroup
	for i, src := range sources {
		i, src := i, src
		wg.Add(1)
		go func() {
			defer wg.Done()

			info := sourceInfo{Name: src.Name(), Type: src.Type()}
			if def != nil && src.Name() == def.Name() {
				info.Default = true
			}

			counts, err := src.Summary()
			if err != nil {
				info.Tasks = map[string]int{}
			} else {
				info.Tasks = counts
			}

			mu.Lock()
			sourceInfos[i] = info
			mu.Unlock()

			// Fetch recent activity
			recent, err := src.RecentActivity(time.Now().Add(-7 * 24 * time.Hour))
			if err == nil {
				mu.Lock()
				for _, t := range recent {
					allRecent = append(allRecent, TaskSummary{
						ID:       t.ID,
						Title:    t.Title,
						Status:   t.Status,
						Assignee: t.Assignee,
						Updated:  t.Updated.Format(time.RFC3339),
						Source:   t.Source,
					})
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Cap recent items
	if len(allRecent) > 10 {
		allRecent = allRecent[:10]
	}
	if allRecent == nil {
		allRecent = []TaskSummary{}
	}

	// Build help dynamically based on current state
	totalOpen := 0
	for _, info := range sourceInfos {
		totalOpen += info.Tasks["open"]
	}

	var help []string
	if totalOpen > 0 {
		help = append(help, "Run `agios tasks list` to see all tasks")
	} else {
		help = append(help, "Run `agios tasks create --title \"...\" --body \"...\"` to create a task")
	}
	if len(allRecent) > 0 {
		help = append(help, "Run `agios tasks get <id>` to see task details")
	}
	if totalOpen > 0 {
		help = append(help, "Run `agios tasks create --title \"...\" --body \"...\"` to create a task")
	} else {
		help = append(help, "Run `agios tasks list --status closed` to see closed tasks")
	}

	emitResult(map[string]any{
		"sources": sourceInfos,
		"recent":  allRecent,
		"help":    help,
	})
}

// respondStatus returns the AIP status response.
func respondStatus() {
	cfg, projectRoot := loadConfigAndRoot()
	sources, err := resolveSources(cfg, projectRoot)
	if err != nil {
		emitResult(map[string]any{
			"name":        "tasks",
			"description": "Built-in task tracking using local files",
			"version":     "1.0.0",
			"status":      "error",
			"status_msg":  "Failed to resolve sources: " + err.Error(),
			"commands":    taskCommands(),
		})
		return
	}

	var sourceNames []string
	for _, s := range sources {
		sourceNames = append(sourceNames, s.Name()+" ("+s.Type()+")")
	}

	status := "info"
	statusMsg := "No task sources available"
	if len(sources) > 0 {
		status = "ok"
		def := resolveDefault(sources, configDefault(cfg))
		defaultName := ""
		if def != nil {
			defaultName = def.Name()
		}
		statusMsg = "Sources: " + strings.Join(sourceNames, ", ") + " (default: " + defaultName + ")"
	}

	emitResult(map[string]any{
		"name":        "tasks",
		"description": "Built-in task tracking using local files",
		"version":     "1.0.0",
		"status":      status,
		"status_msg":  statusMsg,
		"commands":    taskCommands(),
	})
}

// respondHelp returns the AIP help response.
func respondHelp() {
	emitResult(map[string]any{
		"name":  "tasks",
		"usage": "agios tasks <command> [args]",
		"description": "Built-in task tracking using local markdown files. " +
			"All commands accept `--source <name>` to target a specific source.",
		"commands": []map[string]string{
			{"name": "list", "usage": "agios tasks list [--status open|closed|ready] [--assignee <name>] [--source <name>]", "summary": "List tasks. --status ready returns open tasks not blocked by other open tasks."},
			{"name": "get", "usage": "agios tasks get <id> [--source <name>]", "summary": "Show full task details including body and comments."},
			{"name": "create", "usage": "agios tasks create --title <text> --body <text> [--status open|closed] [--assignee <name>] [--blocked-by <id,...>] [--source <name>]", "summary": "Create a new task. Defaults to open."},
			{"name": "update", "usage": "agios tasks update <id> [--title <text>] [--status open|closed] [--assignee <name>] [--body <text>] [--blocked-by <id,...>] [--source <name>]", "summary": "Update an existing task."},
			{"name": "comment", "usage": "agios tasks comment <id> <text> [--source <name>]", "summary": "Add a comment to a task."},
			{"name": "status", "usage": "agios tasks status", "summary": "Show tasks status (AIP protocol)."},
			{"name": "help", "usage": "agios tasks help", "summary": "Show this help message (AIP protocol)."},
			{"name": "peek", "usage": "agios tasks peek", "summary": "Show a snapshot of task state (AIP protocol)."},
		},
		"help": []string{
			"Run `agios tasks` to see aggregated task overview across all sources",
			"Run `agios tasks list` to see tasks from the default source",
			"Run `agios tasks list --source local` to see only local tasks",
			"Run `agios tasks create --title \"Fix bug\" --body \"...\"` to create a new task",
		},
	})
}

// PeekData returns a compact snapshot of task state for the home command.
func PeekData() map[string]any {
	cfg, projectRoot := loadConfigAndRoot()
	sources, err := resolveSources(cfg, projectRoot)
	if err != nil || len(sources) == 0 {
		return nil
	}

	var mu sync.Mutex
	totalOpen := 0
	totalClosed := 0
	var allRecent []TaskSummary

	var wg sync.WaitGroup
	for _, src := range sources {
		src := src
		wg.Add(1)
		go func() {
			defer wg.Done()

			counts, err := src.Summary()
			if err == nil {
				mu.Lock()
				totalOpen += counts["open"]
				totalClosed += counts["closed"]
				mu.Unlock()
			}

			recent, err := src.RecentActivity(time.Now().Add(-7 * 24 * time.Hour))
			if err == nil {
				mu.Lock()
				for _, t := range recent {
					allRecent = append(allRecent, TaskSummary{
						ID:     t.ID,
						Title:  t.Title,
						Status: t.Status,
						Source: t.Source,
					})
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if totalOpen == 0 && totalClosed == 0 {
		return nil
	}

	// Cap recent items
	if len(allRecent) > 5 {
		allRecent = allRecent[:5]
	}

	result := map[string]any{
		"open":   totalOpen,
		"closed": totalClosed,
	}
	if len(allRecent) > 0 {
		result["recent"] = allRecent
	}

	return result
}

// respondPeek returns a compact snapshot of task state across sources.
func respondPeek() {
	data := PeekData()
	if data == nil {
		data = map[string]any{}
	}
	emitResult(data)
}

func taskCommands() []map[string]string {
	return []map[string]string{
		{"name": "list", "usage": "agios tasks list [--status open|closed|ready] [--assignee <name>]", "summary": "List tasks. --status ready returns open tasks not blocked by other open tasks."},
		{"name": "get", "usage": "agios tasks get <id>", "summary": "Show task details"},
		{"name": "create", "usage": "agios tasks create --title <text> --body <text> [--blocked-by <id,...>]", "summary": "Create task"},
		{"name": "update", "usage": "agios tasks update <id> [--status open|closed] [--blocked-by <id,...>]", "summary": "Update task"},
		{"name": "comment", "usage": "agios tasks comment <id> <text>", "summary": "Add comment"},
		{"name": "status", "usage": "agios tasks status", "summary": "AIP status"},
		{"name": "help", "usage": "agios tasks help", "summary": "AIP help"},
	}
}
