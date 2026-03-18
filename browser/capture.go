package browser

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

// doCapture takes a screenshot and saves it.
func doCapture(sess *Session, outPath string) {
	ctx, cancel := context.WithTimeout(sess.Ctx, 15*time.Second)
	defer cancel()

	var buf []byte
	err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf))
	if err != nil {
		emitError(fmt.Sprintf("Screenshot failed: %v", err), "CAPTURE_ERROR",
			"Run `agios browser go <url>` to load a page first",
		)
		os.Exit(1)
	}

	if outPath == "" {
		// Save to ~/.agios/tmp/
		home, err := os.UserHomeDir()
		if err != nil {
			emitError("Failed to find home directory", "INTERNAL_ERROR")
			os.Exit(1)
		}
		tmpDir := filepath.Join(home, ".agios", "tmp")
		os.MkdirAll(tmpDir, 0o755)
		outPath = filepath.Join(tmpDir, fmt.Sprintf("capture_%s.png", randomID()))
	}

	if err := os.WriteFile(outPath, buf, 0o644); err != nil {
		emitError(fmt.Sprintf("Failed to save screenshot: %v", err), "CAPTURE_ERROR")
		os.Exit(1)
	}

	emitResult(map[string]any{
		"path": outPath,
		"size": len(buf),
		"help": []string{
			"Screenshot saved to the path above",
		},
	})
}

// randomID generates a short random hex string for file names.
func randomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use timestamp-based name if crypto/rand fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
