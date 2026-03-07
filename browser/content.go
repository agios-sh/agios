package browser

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// doContent extracts visible text from the page.
func doContent(sess *Session) {
	ctx, cancel := context.WithTimeout(sess.Ctx, 10*time.Second)
	defer cancel()

	var text string
	err := chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText`, &text))
	if err != nil {
		emitError(fmt.Sprintf("Failed to extract text: %v", err), "CONTENT_ERROR",
			"Run `agios browser go <url>` to load a page first",
		)
		os.Exit(1)
	}

	var title, location string
	chromedp.Run(ctx,
		chromedp.Title(&title),
		chromedp.Location(&location),
	)

	emitResult(map[string]any{
		"url":     location,
		"title":   title,
		"content": text,
		"help": []string{
			"Run `agios browser page` to see the page structure with handles",
		},
	})
}
