package browser

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// doTabs handles tab management: list, create, close, switch.
func doTabs(sess *Session, sub string, args []string) {
	ctx, cancel := context.WithTimeout(sess.Ctx, 10*time.Second)
	defer cancel()

	switch sub {
	case "", "list":
		listTabs(ctx)
	case "create":
		url := "about:blank"
		if len(args) > 0 {
			url = args[0]
		}
		createTab(ctx, url)
	case "close":
		if len(args) < 1 {
			emitError("Tab index required", "INVALID_ARGS",
				"Usage: `agios browser tabs close <index>`",
				"Run `agios browser tabs` to see tab indices",
			)
			os.Exit(1)
		}
		closeTab(ctx, args[0])
	case "switch":
		if len(args) < 1 {
			emitError("Tab index required", "INVALID_ARGS",
				"Usage: `agios browser tabs switch <index>`",
				"Run `agios browser tabs` to see tab indices",
			)
			os.Exit(1)
		}
		switchTab(ctx, args[0])
	default:
		emitError("Unknown tabs subcommand: "+sub, "INVALID_ARGS",
			"Usage: `agios browser tabs [create|close|switch]`",
		)
		os.Exit(1)
	}
}

// listTabs lists all open page targets.
func listTabs(ctx context.Context) {
	targets, err := chromedp.Targets(ctx)
	if err != nil {
		emitError(fmt.Sprintf("Failed to list tabs: %v", err), "TABS_ERROR")
		os.Exit(1)
	}

	type tabInfo struct {
		Index int    `json:"index"`
		URL   string `json:"url"`
		Title string `json:"title"`
		ID    string `json:"id"`
	}

	var tabs []tabInfo
	idx := 0
	for _, t := range targets {
		if t.Type != "page" {
			continue
		}
		tabs = append(tabs, tabInfo{
			Index: idx,
			URL:   t.URL,
			Title: t.Title,
			ID:    string(t.TargetID),
		})
		idx++
	}

	emitResult(map[string]any{
		"tabs":  tabs,
		"count": len(tabs),
		"help": []string{
			"Run `agios browser tabs create [url]` to open a new tab",
			"Run `agios browser tabs switch <index>` to switch to a tab",
		},
	})
}

// createTab opens a new tab.
func createTab(ctx context.Context, url string) {
	_, err := chromedp.RunResponse(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := target.CreateTarget(url).Do(ctx)
		return err
	}))
	if err != nil {
		emitError(fmt.Sprintf("Failed to create tab: %v", err), "TABS_ERROR")
		os.Exit(1)
	}

	emitResult(map[string]any{
		"action": "create",
		"url":    url,
		"help": []string{
			"Run `agios browser tabs` to see all tabs",
			"Run `agios browser tabs switch <index>` to switch to the new tab",
		},
	})
}

// closeTab closes a tab by index.
func closeTab(ctx context.Context, indexStr string) {
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		emitError("Invalid tab index: "+indexStr, "INVALID_ARGS",
			"Run `agios browser tabs` to see tab indices",
		)
		os.Exit(1)
	}

	targets, err := chromedp.Targets(ctx)
	if err != nil {
		emitError(fmt.Sprintf("Failed to list tabs: %v", err), "TABS_ERROR")
		os.Exit(1)
	}

	var pageTargets []*target.Info
	for _, t := range targets {
		if t.Type == "page" {
			pageTargets = append(pageTargets, t)
		}
	}

	if index < 0 || index >= len(pageTargets) {
		emitError(fmt.Sprintf("Tab index %d out of range (0-%d)", index, len(pageTargets)-1), "INVALID_ARGS",
			"Run `agios browser tabs` to see tab indices",
		)
		os.Exit(1)
	}

	if len(pageTargets) <= 1 {
		emitError("Cannot close the last tab", "TABS_ERROR",
			"Run `agios browser quit` to stop the browser instead",
		)
		os.Exit(1)
	}

	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return target.CloseTarget(pageTargets[index].TargetID).Do(ctx)
	}))
	if err != nil {
		emitError(fmt.Sprintf("Failed to close tab: %v", err), "TABS_ERROR")
		os.Exit(1)
	}

	emitResult(map[string]any{
		"action": "close",
		"index":  index,
		"help": []string{
			"Run `agios browser tabs` to see remaining tabs",
		},
	})
}

// switchTab activates a tab by index.
func switchTab(ctx context.Context, indexStr string) {
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		emitError("Invalid tab index: "+indexStr, "INVALID_ARGS",
			"Run `agios browser tabs` to see tab indices",
		)
		os.Exit(1)
	}

	targets, err := chromedp.Targets(ctx)
	if err != nil {
		emitError(fmt.Sprintf("Failed to list tabs: %v", err), "TABS_ERROR")
		os.Exit(1)
	}

	var pageTargets []*target.Info
	for _, t := range targets {
		if t.Type == "page" {
			pageTargets = append(pageTargets, t)
		}
	}

	if index < 0 || index >= len(pageTargets) {
		emitError(fmt.Sprintf("Tab index %d out of range (0-%d)", index, len(pageTargets)-1), "INVALID_ARGS",
			"Run `agios browser tabs` to see tab indices",
		)
		os.Exit(1)
	}

	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return target.ActivateTarget(pageTargets[index].TargetID).Do(ctx)
	}))
	if err != nil {
		emitError(fmt.Sprintf("Failed to switch tab: %v", err), "TABS_ERROR")
		os.Exit(1)
	}

	emitResult(map[string]any{
		"action": "switch",
		"index":  index,
		"url":    pageTargets[index].URL,
		"title":  pageTargets[index].Title,
		"help": []string{
			"Run `agios browser page` to see the page structure",
		},
	})
}
