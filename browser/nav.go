package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// doGo navigates to a URL and waits for the page to load.
func doGo(sess *Session, url string) {
	ctx, cancel := context.WithTimeout(sess.Ctx, 30*time.Second)
	defer cancel()

	// Emit progress at 3s if still loading
	done := make(chan struct{})
	go func() {
		select {
		case <-time.After(3 * time.Second):
			progress := map[string]any{"progress": "Loading " + url + "..."}
			data, _ := json.Marshal(progress)
			os.Stdout.Write(data)
			os.Stdout.Write([]byte("\n"))
		case <-done:
		}
	}()

	err := chromedp.Run(ctx, chromedp.Navigate(url))
	close(done)
	if err != nil {
		emitError(fmt.Sprintf("Navigation failed: %v", err), "NAV_ERROR",
			"Check the URL and try again",
			"Run `agios browser status` to check browser state",
		)
		os.Exit(1)
	}

	// Wait for DOM content to be ready
	chromedp.Run(ctx, chromedp.WaitReady("body"))

	var title, location string
	chromedp.Run(ctx,
		chromedp.Title(&title),
		chromedp.Location(&location),
	)

	emitResult(map[string]any{
		"url":   location,
		"title": title,
		"help": []string{
			"Run `agios browser page` to see the page structure",
			"Run `agios browser content` to extract page text",
		},
	})
}
