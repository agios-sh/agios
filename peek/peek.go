// Package peek fetches state snapshots from apps for the dock view.
package peek

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/agios-sh/agios/runner"
	"golang.org/x/sync/errgroup"
)

type AppEntry struct {
	Name    string         `json:"name"`
	Summary string         `json:"summary,omitempty"`
	Peek    map[string]any `json:"peek,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type FetchResult struct {
	AppName     string
	Description string
	Peek        map[string]any
	Error       string
}

// FetchAll concurrently fetches peek + status data from all apps.
func FetchAll(apps []string) []FetchResult {
	var mu sync.Mutex
	results := make([]FetchResult, len(apps))

	g := new(errgroup.Group)

	for i, app := range apps {
		i, app := i, app
		g.Go(func() error {
			result := fetchAppPeek(app)
			mu.Lock()
			results[i] = result
			mu.Unlock()
			return nil
		})
	}

	g.Wait()
	return results
}

func fetchAppPeek(appName string) FetchResult {
	result := FetchResult{AppName: appName}

	binPath, err := runner.Resolve(appName)
	if err != nil {
		result.Error = fmt.Sprintf("Binary %q not found on PATH", appName)
		return result
	}

	var description string
	var peekData map[string]any
	var peekErr error

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		statusResult, statusErr := runner.Exec(binPath, []string{"status"}, runner.DefaultTimeout)
		if statusErr == nil && statusResult != nil && len(statusResult.Stdout) > 0 {
			var obj map[string]any
			if err := json.Unmarshal(statusResult.Stdout, &obj); err == nil {
				if d, ok := obj["description"].(string); ok {
					description = d
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		peekResult, execErr := runner.Exec(binPath, []string{"peek"}, runner.DefaultTimeout)

		if peekResult == nil || len(peekResult.Stdout) == 0 {
			if execErr != nil {
				peekErr = fmt.Errorf("failed to fetch peek from %s: %s", appName, execErr.Error())
			}
			return
		}

		parsed, err := parsePeek(peekResult.Stdout)
		if err != nil {
			peekErr = fmt.Errorf("invalid peek output from %s: %s", appName, err.Error())
			return
		}
		peekData = parsed
	}()

	wg.Wait()

	result.Description = description
	result.Peek = peekData
	if peekErr != nil {
		result.Error = peekErr.Error()
	}
	return result
}

func parsePeek(data []byte) (map[string]any, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		return obj, nil
	}

	parsed, err := runner.ParseJSONL(data)
	if err != nil {
		return nil, fmt.Errorf("invalid peek output: %w", err)
	}

	if parsed.Result != nil {
		return parsed.Result, nil
	}

	return nil, nil
}
