package x11

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// StartApp starts an application on the X display
func (c *Client) StartApp(app string, args []string) (int, error) {
	return c.StartAppWithEnv(app, args, nil)
}

// StartAppWithEnv starts an application with custom environment variables
func (c *Client) StartAppWithEnv(app string, args []string, env map[string]string) (int, error) {
	// Check if the app exists
	appPath, err := exec.LookPath(app)
	if err != nil {
		return 0, fmt.Errorf("application not found: %w", err)
	}
	
	// Create command
	cmd := exec.Command(appPath, args...)
	
	// Set up environment
	cmd.Env = os.Environ()
	
	// Ensure DISPLAY is set to our display
	cmd.Env = setEnv(cmd.Env, "DISPLAY", c.display)
	
	// Add custom environment variables
	for k, v := range env {
		cmd.Env = setEnv(cmd.Env, k, v)
	}
	
	// Set up process attributes to put it in its own process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	
	// Start the process
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start application: %w", err)
	}
	
	return cmd.Process.Pid, nil
}

// StopApp stops an application by PID
func (c *Client) StopApp(pid int) error {
	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}
	
	// Try graceful termination first
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// If SIGTERM fails, try SIGKILL
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}
	
	// Wait for process to exit (non-blocking)
	process.Wait()
	
	return nil
}

// setEnv sets or updates an environment variable in a slice
func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if len(e) > len(prefix) && e[:len(prefix)] == prefix {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}