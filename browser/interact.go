package browser

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// doClick clicks the element identified by a handle.
func doClick(sess *Session, handle string) {
	nodeID, err := resolveHandle(handle)
	if err != nil {
		emitError(err.Error(), "HANDLE_ERROR",
			"Run `agios browser page` to see available handles",
		)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(sess.Ctx, 10*time.Second)
	defer cancel()

	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		remoteObj, err := dom.ResolveNode().WithBackendNodeID(cdp.BackendNodeID(nodeID)).Do(ctx)
		if err != nil {
			return fmt.Errorf("resolving node: %w", err)
		}

		// Scroll into view
		_, err = dom.GetContentQuads().WithObjectID(remoteObj.ObjectID).Do(ctx)
		if err != nil {
			callJSFunction(ctx, remoteObj.ObjectID, "function() { this.scrollIntoView({block: 'center'}); }")
		}

		// Get coordinates for click
		x, y, err := getElementCenter(ctx, remoteObj.ObjectID)
		if err != nil {
			// Fallback: JS click
			callJSFunction(ctx, remoteObj.ObjectID, "function() { this.click(); }")
			return nil
		}

		if err := input.DispatchMouseEvent(input.MousePressed, x, y).
			WithButton(input.Left).WithClickCount(1).Do(ctx); err != nil {
			return err
		}
		return input.DispatchMouseEvent(input.MouseReleased, x, y).
			WithButton(input.Left).WithClickCount(1).Do(ctx)
	}))

	if err != nil {
		emitError(fmt.Sprintf("Click failed: %v", err), "INTERACT_ERROR",
			"Run `agios browser page` to refresh handles",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"action": "click",
		"handle": handle,
		"help":   []string{"Run `agios browser page` to see the updated page"},
	})
}

// doInput focuses the element and types keystrokes.
func doInput(sess *Session, handle, text string) {
	nodeID, err := resolveHandle(handle)
	if err != nil {
		emitError(err.Error(), "HANDLE_ERROR",
			"Run `agios browser page` to see available handles",
		)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(sess.Ctx, 10*time.Second)
	defer cancel()

	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		remoteObj, err := dom.ResolveNode().WithBackendNodeID(cdp.BackendNodeID(nodeID)).Do(ctx)
		if err != nil {
			return fmt.Errorf("resolving node: %w", err)
		}

		callJSFunction(ctx, remoteObj.ObjectID, "function() { this.focus(); }")

		for _, ch := range text {
			if err := input.DispatchKeyEvent(input.KeyDown).
				WithText(string(ch)).Do(ctx); err != nil {
				return err
			}
		}
		return nil
	}))

	if err != nil {
		emitError(fmt.Sprintf("Input failed: %v", err), "INTERACT_ERROR",
			"Run `agios browser page` to refresh handles",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"action": "input",
		"handle": handle,
		"text":   text,
		"help":   []string{"Run `agios browser page` to see the updated page"},
	})
}

// doSet sets an input element's value directly.
func doSet(sess *Session, handle, value string) {
	nodeID, err := resolveHandle(handle)
	if err != nil {
		emitError(err.Error(), "HANDLE_ERROR",
			"Run `agios browser page` to see available handles",
		)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(sess.Ctx, 10*time.Second)
	defer cancel()

	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		remoteObj, err := dom.ResolveNode().WithBackendNodeID(cdp.BackendNodeID(nodeID)).Do(ctx)
		if err != nil {
			return fmt.Errorf("resolving node: %w", err)
		}

		escaped := escapeJS(value)
		js := fmt.Sprintf(`function() {
			this.focus();
			this.value = '%s';
			this.dispatchEvent(new Event('input', {bubbles: true}));
			this.dispatchEvent(new Event('change', {bubbles: true}));
		}`, escaped)
		callJSFunction(ctx, remoteObj.ObjectID, js)
		return nil
	}))

	if err != nil {
		emitError(fmt.Sprintf("Set failed: %v", err), "INTERACT_ERROR",
			"Run `agios browser page` to refresh handles",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"action": "set",
		"handle": handle,
		"value":  value,
		"help":   []string{"Run `agios browser page` to see the updated page"},
	})
}

// doKey presses a key or key combination.
func doKey(sess *Session, key string) {
	ctx, cancel := context.WithTimeout(sess.Ctx, 10*time.Second)
	defer cancel()

	keyDef := mapKey(key)

	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		if err := input.DispatchKeyEvent(input.KeyDown).
			WithKey(keyDef.key).
			WithCode(keyDef.code).
			WithWindowsVirtualKeyCode(keyDef.keyCode).
			WithNativeVirtualKeyCode(keyDef.keyCode).
			Do(ctx); err != nil {
			return err
		}
		return input.DispatchKeyEvent(input.KeyUp).
			WithKey(keyDef.key).
			WithCode(keyDef.code).
			WithWindowsVirtualKeyCode(keyDef.keyCode).
			WithNativeVirtualKeyCode(keyDef.keyCode).
			Do(ctx)
	}))

	if err != nil {
		emitError(fmt.Sprintf("Key press failed: %v", err), "INTERACT_ERROR",
			"Run `agios browser page` to see the page",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"action": "key",
		"key":    key,
		"help":   []string{"Run `agios browser page` to see the updated page"},
	})
}

// doHover hovers over the element identified by a handle.
func doHover(sess *Session, handle string) {
	nodeID, err := resolveHandle(handle)
	if err != nil {
		emitError(err.Error(), "HANDLE_ERROR",
			"Run `agios browser page` to see available handles",
		)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(sess.Ctx, 10*time.Second)
	defer cancel()

	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		remoteObj, err := dom.ResolveNode().WithBackendNodeID(cdp.BackendNodeID(nodeID)).Do(ctx)
		if err != nil {
			return fmt.Errorf("resolving node: %w", err)
		}

		x, y, err := getElementCenter(ctx, remoteObj.ObjectID)
		if err != nil {
			return fmt.Errorf("getting element position: %w", err)
		}

		return input.DispatchMouseEvent(input.MouseMoved, x, y).Do(ctx)
	}))

	if err != nil {
		emitError(fmt.Sprintf("Hover failed: %v", err), "INTERACT_ERROR",
			"Run `agios browser page` to refresh handles",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"action": "hover",
		"handle": handle,
		"help":   []string{"Run `agios browser page` to see the updated page"},
	})
}

// doScroll scrolls the page or to a specific element.
func doScroll(sess *Session, scrollTarget string) {
	ctx, cancel := context.WithTimeout(sess.Ctx, 10*time.Second)
	defer cancel()

	if scrollTarget == "" {
		err := chromedp.Run(ctx, chromedp.Evaluate(`window.scrollBy(0, window.innerHeight)`, nil))
		if err != nil {
			emitError(fmt.Sprintf("Scroll failed: %v", err), "INTERACT_ERROR")
			os.Exit(1)
		}
		emitResult(map[string]any{
			"action": "scroll",
			"target": "viewport",
			"help":   []string{"Run `agios browser page` to see the updated page"},
		})
		return
	}

	if pixels, err := strconv.Atoi(scrollTarget); err == nil {
		err := chromedp.Run(ctx, chromedp.Evaluate(
			fmt.Sprintf(`window.scrollBy(0, %d)`, pixels), nil))
		if err != nil {
			emitError(fmt.Sprintf("Scroll failed: %v", err), "INTERACT_ERROR")
			os.Exit(1)
		}
		emitResult(map[string]any{
			"action": "scroll",
			"pixels": pixels,
			"help":   []string{"Run `agios browser page` to see the updated page"},
		})
		return
	}

	nodeID, err := resolveHandle(scrollTarget)
	if err != nil {
		emitError(err.Error(), "HANDLE_ERROR",
			"Run `agios browser page` to see available handles",
		)
		os.Exit(1)
	}

	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		remoteObj, err := dom.ResolveNode().WithBackendNodeID(cdp.BackendNodeID(nodeID)).Do(ctx)
		if err != nil {
			return fmt.Errorf("resolving node: %w", err)
		}
		callJSFunction(ctx, remoteObj.ObjectID, "function() { this.scrollIntoView({block: 'center', behavior: 'smooth'}); }")
		return nil
	}))

	if err != nil {
		emitError(fmt.Sprintf("Scroll failed: %v", err), "INTERACT_ERROR",
			"Run `agios browser page` to refresh handles",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"action": "scroll",
		"handle": scrollTarget,
		"help":   []string{"Run `agios browser page` to see the updated page"},
	})
}

// doPick selects an option from a dropdown/select element.
func doPick(sess *Session, handle, value string) {
	nodeID, err := resolveHandle(handle)
	if err != nil {
		emitError(err.Error(), "HANDLE_ERROR",
			"Run `agios browser page` to see available handles",
		)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(sess.Ctx, 10*time.Second)
	defer cancel()

	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		remoteObj, err := dom.ResolveNode().WithBackendNodeID(cdp.BackendNodeID(nodeID)).Do(ctx)
		if err != nil {
			return fmt.Errorf("resolving node: %w", err)
		}

		escaped := escapeJS(value)
		js := fmt.Sprintf(`function() {
			var opts = this.options;
			for (var i = 0; i < opts.length; i++) {
				if (opts[i].value === '%s' || opts[i].text === '%s') {
					this.selectedIndex = i;
					this.dispatchEvent(new Event('change', {bubbles: true}));
					return true;
				}
			}
			return false;
		}`, escaped, escaped)
		callJSFunction(ctx, remoteObj.ObjectID, js)
		return nil
	}))

	if err != nil {
		emitError(fmt.Sprintf("Pick failed: %v", err), "INTERACT_ERROR",
			"Run `agios browser page` to refresh handles",
		)
		os.Exit(1)
	}

	emitResult(map[string]any{
		"action": "pick",
		"handle": handle,
		"value":  value,
		"help":   []string{"Run `agios browser page` to see the updated page"},
	})
}

// getElementCenter returns the center coordinates of an element.
func getElementCenter(ctx context.Context, objectID runtime.RemoteObjectID) (float64, float64, error) {
	quads, err := dom.GetContentQuads().WithObjectID(objectID).Do(ctx)
	if err != nil {
		return 0, 0, err
	}
	if len(quads) == 0 || len(quads[0]) < 8 {
		return 0, 0, fmt.Errorf("no quads returned")
	}

	q := quads[0]
	x := (q[0] + q[2] + q[4] + q[6]) / 4
	y := (q[1] + q[3] + q[5] + q[7]) / 4
	return math.Round(x), math.Round(y), nil
}

// callJSFunction calls a JavaScript function on a remote object.
func callJSFunction(ctx context.Context, objectID runtime.RemoteObjectID, fn string) {
	runtime.CallFunctionOn(fn).
		WithObjectID(objectID).
		Do(ctx)
}

// escapeJS escapes a string for safe embedding in a JS single-quoted literal.
func escapeJS(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\n", `\n`, "\r", `\r`)
	return r.Replace(s)
}

// keyDef holds CDP key information.
type keyDef struct {
	key     string
	code    string
	keyCode int64
}

// mapKey maps common key names to CDP key definitions.
func mapKey(name string) keyDef {
	switch name {
	case "Enter":
		return keyDef{"Enter", "Enter", 13}
	case "Tab":
		return keyDef{"Tab", "Tab", 9}
	case "Escape", "Esc":
		return keyDef{"Escape", "Escape", 27}
	case "Backspace":
		return keyDef{"Backspace", "Backspace", 8}
	case "Delete":
		return keyDef{"Delete", "Delete", 46}
	case "ArrowUp", "Up":
		return keyDef{"ArrowUp", "ArrowUp", 38}
	case "ArrowDown", "Down":
		return keyDef{"ArrowDown", "ArrowDown", 40}
	case "ArrowLeft", "Left":
		return keyDef{"ArrowLeft", "ArrowLeft", 37}
	case "ArrowRight", "Right":
		return keyDef{"ArrowRight", "ArrowRight", 39}
	case "Home":
		return keyDef{"Home", "Home", 36}
	case "End":
		return keyDef{"End", "End", 35}
	case "PageUp":
		return keyDef{"PageUp", "PageUp", 33}
	case "PageDown":
		return keyDef{"PageDown", "PageDown", 34}
	case "Space":
		return keyDef{" ", "Space", 32}
	default:
		if len(name) == 1 {
			return keyDef{name, "Key" + name, int64(name[0])}
		}
		return keyDef{name, name, 0}
	}
}
