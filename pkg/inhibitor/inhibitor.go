package inhibitor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
	"wails-cast/pkg/execresolver"
	_logger "wails-cast/pkg/logger"
)

var InhibitorInstance = &Inhibitor{}
var logger = _logger.Logger

// Inhibitor manages brief system sleep inhibition during streaming
type Inhibitor struct {
	cmd      *exec.Cmd
	mu       sync.Mutex
	timer    *time.Timer
	autoStop bool
}

// Refresh starts or refreshes sleep inhibition for a brief period (auto-stops after duration)
func Refresh() {
	inhibitor, err := startInhibitor()
	if err != nil {
		logger.Error("Failed to start inhibitor", "err", err)
	}

	inhibitor.Process.Kill()

}

func startInhibitor() (*exec.Cmd, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// macOS: use caffeinate
		caffeinatePath := execresolver.Find("caffeinate")
		cmd = exec.Command(caffeinatePath)
	case "linux":
		// Linux: try systemd-inhibit (systemd), then fallback to xdg-screensaver
		if systemdPath, exists := execresolver.FindWithCheck("systemd-inhibit"); exists {
			cmd = exec.Command(systemdPath, "--what=idle:sleep", "--who=wails-cast", "--why=Streaming media", "--mode=block", "sleep", "1")
		} else if xdgPath, exists := execresolver.FindWithCheck("xdg-screensaver"); exists {
			cmd = exec.Command(xdgPath, "suspend", fmt.Sprintf("%d", os.Getpid()))
		}
	case "windows":
		// Windows: reset sleep timer via PowerShell
		powershellPath := execresolver.Find("powershell")
		cmd = exec.Command(powershellPath, "-Command", "[System.Threading.Thread]::CurrentThread.SetThreadExecutionState(1)")
	}

	if cmd == nil {
		return nil, fmt.Errorf("sleep inhibition not supported on %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start sleep inhibitor command: %v", err)
	}

	return cmd, nil
}

// Stop allows system sleep again
func (i *Inhibitor) Stop() {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Cancel timer if exists
	if i.timer != nil {
		i.timer.Stop()
		i.timer = nil
	}

	if i.cmd != nil && i.cmd.Process != nil {
		i.cmd.Process.Kill()
		i.cmd = nil
		if logger != nil {
			logger.Info("Sleep inhibition disabled")
		}
	}
}

// IsActive returns whether sleep inhibition is currently active
func (i *Inhibitor) IsActive() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.cmd != nil && i.cmd.Process != nil
}
