package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGitHubAPIURL = "https://api.github.com/repos/agios-sh/agios/releases/latest"
	cacheFileName       = "update-check.json"
	cacheTTL            = 24 * time.Hour
	httpTimeout         = 30 * time.Second
	agiosDir            = ".agios"
)

// githubAPIURL is the endpoint for fetching the latest release.
// Overridden in tests to point at a local HTTP server.
var githubAPIURL = defaultGitHubAPIURL

// CheckResult holds the outcome of a version check.
type CheckResult struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	DownloadURL     string `json:"download_url,omitempty"`
}

// CacheEntry is persisted to ~/.agios/update-check.json.
type CacheEntry struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

// githubRelease is the subset of the GitHub API response we need.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckLatest hits the GitHub API, compares versions, and writes the cache.
func CheckLatest(currentVersion string) (*CheckResult, error) {
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(githubAPIURL)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}

	// Write cache
	entry := CacheEntry{
		CheckedAt:     time.Now(),
		LatestVersion: release.TagName,
	}
	_ = writeCache(entry)

	result := &CheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   release.TagName,
		UpdateAvailable: CompareVersions(release.TagName, currentVersion) > 0,
	}

	// Find download URL for this platform
	assetName := AssetName()
	for _, a := range release.Assets {
		if a.Name == assetName {
			result.DownloadURL = a.BrowserDownloadURL
			break
		}
	}

	return result, nil
}

// ReadCache reads ~/.agios/update-check.json and returns a CheckResult
// if the cache exists. Returns nil if missing or corrupt.
func ReadCache(currentVersion string) *CheckResult {
	path := cachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}

	if entry.LatestVersion == "" {
		return nil
	}

	return &CheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   entry.LatestVersion,
		UpdateAvailable: CompareVersions(entry.LatestVersion, currentVersion) > 0,
	}
}

// IsCacheStale returns true if the cache is >24h old or missing.
func IsCacheStale() bool {
	path := cachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return true
	}

	return time.Since(entry.CheckedAt) > cacheTTL
}

// SpawnBackgroundCheck spawns a detached child process to check for updates.
// The child process inherits AGIOS_NO_UPDATE_CHECK=1 to prevent recursive spawning.
func SpawnBackgroundCheck(currentVersion string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	cmd := exec.Command(self, "--update-check", currentVersion)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), "AGIOS_NO_UPDATE_CHECK=1")

	devNull, err := os.Open(os.DevNull)
	if err == nil {
		cmd.Stdout = devNull
		cmd.Stderr = devNull
		defer devNull.Close()
	}

	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawning update check: %w", err)
	}

	// Fire and forget — don't wait
	go func() {
		cmd.Wait()
	}()

	return nil
}

// RunBackgroundCheck is the entry point for the --update-check child process.
// It checks for updates, writes the cache, and exits silently.
func RunBackgroundCheck(args []string) {
	if len(args) == 0 {
		os.Exit(1)
	}
	currentVersion := args[0]
	// Best-effort: ignore errors
	CheckLatest(currentVersion)
}

// Apply downloads the archive, extracts the binary, and atomically replaces
// the current executable.
func Apply(result *CheckResult) error {
	if result.DownloadURL == "" {
		return fmt.Errorf("no download URL for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Download to temp file
	archivePath, err := downloadToTemp(result.DownloadURL)
	if err != nil {
		return fmt.Errorf("downloading update: %w", err)
	}
	defer os.Remove(archivePath)

	// Extract binary
	newBinaryPath, err := extractBinary(archivePath)
	if err != nil {
		return fmt.Errorf("extracting update: %w", err)
	}
	defer os.Remove(newBinaryPath)

	// Remove macOS quarantine
	removeQuarantine(newBinaryPath)

	// Resolve current executable path
	target, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving current executable: %w", err)
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	// Atomically replace
	if err := atomicReplace(target, newBinaryPath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}

// CompareVersions compares two semver strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareVersions(a, b string) int {
	av := parseVersion(a)
	bv := parseVersion(b)

	for i := 0; i < 3; i++ {
		if av[i] < bv[i] {
			return -1
		}
		if av[i] > bv[i] {
			return 1
		}
	}
	return 0
}

// AssetName returns the expected archive file name for the current platform.
func AssetName() string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("agios_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

// parseVersion extracts [major, minor, patch] from a version string.
func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		// Strip any pre-release suffix (e.g., "1-beta")
		num := strings.SplitN(parts[i], "-", 2)[0]
		result[i], _ = strconv.Atoi(num)
	}
	return result
}

func cachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, agiosDir, cacheFileName)
}

func writeCache(entry CacheEntry) error {
	path := cachePath()
	if path == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func downloadToTemp(url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "agios-update-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}

	return tmp.Name(), nil
}

func extractBinary(archivePath string) (string, error) {
	if runtime.GOOS == "windows" {
		return extractZip(archivePath)
	}
	return extractTarGz(archivePath)
}

func extractTarGz(archivePath string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	binaryName := "agios"

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		name := filepath.Base(header.Name)
		if name == binaryName && header.Typeflag == tar.TypeReg {
			tmp, err := os.CreateTemp("", "agios-new-*")
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(tmp, tr); err != nil {
				tmp.Close()
				os.Remove(tmp.Name())
				return "", err
			}
			tmp.Close()
			if err := os.Chmod(tmp.Name(), 0o755); err != nil {
				os.Remove(tmp.Name())
				return "", err
			}
			return tmp.Name(), nil
		}
	}

	return "", fmt.Errorf("binary %q not found in archive", binaryName)
}

func extractZip(archivePath string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	binaryName := "agios.exe"

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name == binaryName {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			tmp, err := os.CreateTemp("", "agios-new-*.exe")
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(tmp, rc); err != nil {
				tmp.Close()
				os.Remove(tmp.Name())
				return "", err
			}
			tmp.Close()
			if err := os.Chmod(tmp.Name(), 0o755); err != nil {
				os.Remove(tmp.Name())
				return "", err
			}
			return tmp.Name(), nil
		}
	}

	return "", fmt.Errorf("binary %q not found in archive", binaryName)
}

func atomicReplace(target, newBinary string) error {
	// Get permissions from original
	info, err := os.Stat(target)
	if err != nil {
		return err
	}

	// Create temp file in same directory as target (ensures same filesystem for rename)
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".agios-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	// Copy new binary to temp location
	src, err := os.Open(newBinary)
	if err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}

	if _, err := io.Copy(tmp, src); err != nil {
		src.Close()
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	src.Close()
	tmp.Close()

	// Set permissions to match original
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Atomic rename
	if err := os.Rename(tmpPath, target); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}
