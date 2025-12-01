package inhibitor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
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
func (i *Inhibitor) Refresh(duration time.Duration) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// If already running, just reset the timer
	if i.cmd != nil && i.cmd.Process != nil {
		if i.timer != nil {
			i.timer.Reset(duration)
		} else {
			i.timer = time.AfterFunc(duration, func() {
				i.Stop()
			})
		}
		return nil
	}

	// Start new inhibition
	if err := i.startLocked(); err != nil {
		return err
	}

	// Set up auto-stop timer
	i.timer = time.AfterFunc(duration, func() {
		i.Stop()
	})

	return nil
}

func (i *Inhibitor) startLocked() error {
	// Kill existing process if running
	if i.cmd != nil && i.cmd.Process != nil {
		i.cmd.Process.Kill()
		i.cmd = nil
	}

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// macOS: use caffeinate
		cmd = exec.Command("caffeinate", "-d") // -d prevents display sleep
	case "linux":
		// Linux: try systemd-inhibit (systemd), then fallback to xdg-screensaver
		if _, err := exec.LookPath("systemd-inhibit"); err == nil {
			cmd = exec.Command("systemd-inhibit", "--what=idle:sleep", "--who=wails-cast", "--why=Streaming media", "--mode=block", "sleep", "infinity")
		} else if _, err := exec.LookPath("xdg-screensaver"); err == nil {
			cmd = exec.Command("xdg-screensaver", "suspend", fmt.Sprintf("%d", os.Getpid()))
		}
	case "windows":
		// Windows: use powercfg or SetThreadExecutionState via PowerShell
		// Note: This uses a PowerShell command to prevent sleep
		cmd = exec.Command("powershell", "-Command", "$null = [System.Threading.Thread]::CurrentThread.SetThreadExecutionState(3); while($true){Start-Sleep -Seconds 30}")
	}

	if cmd == nil {
		if logger != nil {
			logger.Warn("Sleep inhibition not supported on this platform", "os", runtime.GOOS)
		}
		return fmt.Errorf("sleep inhibition not supported on %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		if logger != nil {
			logger.Warn("Failed to start sleep inhibition", "error", err)
		}
		return err
	}

	i.cmd = cmd
	if logger != nil {
		logger.Info("Sleep inhibition enabled", "os", runtime.GOOS)
	}
	return nil
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
