package execresolver

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	cache = make(map[string]string)
	mu    sync.RWMutex
)

// Find locates an executable in common system paths
// Returns the full path if found, or just the name as fallback
func Find(name string) string {
	// Check cache first
	mu.RLock()
	if path, ok := cache[name]; ok {
		mu.RUnlock()
		return path
	}
	mu.RUnlock()

	// Search for executable
	path := findExecutable(name)

	// Cache the result
	mu.Lock()
	cache[name] = path
	mu.Unlock()

	return path
}

// FindWithCheck locates an executable and returns whether it exists
// Returns the path (or name as fallback) and true if the file exists and is accessible
func FindWithCheck(name string) (string, bool) {
	path := Find(name)
	_, err := os.Stat(path)
	return path, err == nil
}

// FindRefresh forces a fresh search for an executable, bypassing cache
func FindRefresh(name string) string {
	path := findExecutable(name)

	// Update cache
	mu.Lock()
	cache[name] = path
	mu.Unlock()

	return path
}

// FindRefreshWithCheck forces a fresh search and returns whether it exists
func FindRefreshWithCheck(name string) (string, bool) {
	path := FindRefresh(name)
	_, err := os.Stat(path)
	return path, err == nil
}

// Exists checks if an executable exists and is accessible
func Exists(name string) bool {
	path := Find(name)
	_, err := os.Stat(path)
	return err == nil
}

// ExistsRefresh checks if an executable exists with a fresh search
func ExistsRefresh(name string) bool {
	path := FindRefresh(name)
	_, err := os.Stat(path)
	return err == nil
}

// ClearCache clears the executable path cache
func ClearCache() {
	mu.Lock()
	cache = make(map[string]string)
	mu.Unlock()
}

// findExecutable locates an executable in common system paths
func findExecutable(name string) string {
	// First try exec.LookPath with current PATH
	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	// Get platform-specific common paths
	commonPaths := getCommonPaths()

	// Check common installation directories
	for _, dir := range commonPaths {
		fullPath := filepath.Join(dir, name)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}

	// Fall back to just the name (will fail if not in PATH)
	return name
}

// getCommonPaths returns common executable paths for the current platform
func getCommonPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/usr/local/bin",
			"/opt/homebrew/bin",
			"/usr/bin",
			"/opt/local/bin",
			"/bin",
		}
	case "linux":
		return []string{
			"/usr/local/bin",
			"/usr/bin",
			"/bin",
			"/snap/bin",
			"/usr/local/sbin",
			"/usr/sbin",
			"/sbin",
		}
	case "windows":
		return []string{
			"C:\\Windows\\System32",
			"C:\\Windows",
			"C:\\Program Files\\ffmpeg\\bin",
		}
	default:
		return []string{"/usr/local/bin", "/usr/bin", "/bin"}
	}
}
