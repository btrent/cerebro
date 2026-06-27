package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"cerebro/internal/paths"
)

func serverLogPath() string { return paths.ServerLogPath() }

// ServerConfig describes an optional, cerebro-managed inference server. When
// AutoLaunch is true and the backend is not already reachable, the Manager runs
// LaunchCommand and waits until the health endpoint responds.
type ServerConfig struct {
	AutoLaunch     bool
	BaseURL        string // e.g. http://127.0.0.1:8080/v1
	HealthPath     string // appended to BaseURL, e.g. /models
	StartupTimeout time.Duration
	LaunchCommand  string // shell command; ~ is expanded to $HOME
}

// Manager owns the lifecycle of an optional child inference server.
type Manager struct {
	cfg      ServerConfig
	cmd      *exec.Cmd
	launched bool
	logFile  *os.File
}

// NewManager creates a Manager for the given server configuration.
func NewManager(cfg ServerConfig) *Manager {
	if cfg.HealthPath == "" {
		cfg.HealthPath = "/models"
	}
	if cfg.StartupTimeout == 0 {
		cfg.StartupTimeout = 240 * time.Second
	}
	return &Manager{cfg: cfg}
}

// Reachable reports whether the backend currently answers its health endpoint.
func (m *Manager) Reachable(ctx context.Context) bool {
	return reachable(ctx, m.healthURL())
}

func (m *Manager) healthURL() string {
	return strings.TrimRight(m.cfg.BaseURL, "/") + m.cfg.HealthPath
}

// EnsureRunning makes the backend available, launching it if configured. status
// receives human-readable progress messages for display in the UI. It returns a
// helpful error (never panics) if the backend cannot be made available.
func (m *Manager) EnsureRunning(ctx context.Context, status func(string)) error {
	if status == nil {
		status = func(string) {}
	}

	if m.Reachable(ctx) {
		status("Backend already running.")
		return nil
	}

	if !m.cfg.AutoLaunch {
		return fmt.Errorf("backend not reachable at %s and auto_launch is disabled.\n"+
			"Start it manually, then relaunch cerebro", m.cfg.BaseURL)
	}
	if strings.TrimSpace(m.cfg.LaunchCommand) == "" {
		return fmt.Errorf("backend not reachable at %s and no launch_command is configured", m.cfg.BaseURL)
	}

	if err := m.launch(); err != nil {
		return err
	}

	status("Loading model (first load can take a minute)…")
	return m.waitHealthy(ctx, status)
}

func (m *Manager) launch() error {
	command := expandHome(m.cfg.LaunchCommand)

	// Route the child server's noisy output to a log file rather than the TUI.
	logPath := serverLogPath()
	if f, err := os.Create(logPath); err == nil {
		m.logFile = f
	}

	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // own process group → killable as a unit
	if m.logFile != nil {
		cmd.Stdout = m.logFile
		cmd.Stderr = m.logFile
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting backend (%s): %w", command, err)
	}
	m.cmd = cmd
	m.launched = true
	return nil
}

func (m *Manager) waitHealthy(ctx context.Context, status func(string)) error {
	deadline := time.Now().Add(m.cfg.StartupTimeout)
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Surface early child death with a pointer to the log.
		if m.cmd != nil && m.cmd.ProcessState != nil && m.cmd.ProcessState.Exited() {
			return fmt.Errorf("backend process exited during startup; see %s", serverLogPath())
		}
		if reachable(ctx, m.healthURL()) {
			status("Backend ready.")
			return nil
		}
		if time.Now().After(deadline) {
			m.Shutdown()
			return fmt.Errorf("backend did not become healthy within %s; see %s", m.cfg.StartupTimeout, serverLogPath())
		}
		select {
		case <-ctx.Done():
			m.Shutdown()
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Shutdown terminates the child server (and its process group) if cerebro
// launched it. It is safe to call multiple times and is a no-op for a backend
// that cerebro merely connected to.
func (m *Manager) Shutdown() {
	if m.logFile != nil {
		_ = m.logFile.Close()
		m.logFile = nil
	}
	if !m.launched || m.cmd == nil || m.cmd.Process == nil {
		return
	}
	pgid := m.cmd.Process.Pid
	// Negative pid signals the whole process group.
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	done := make(chan struct{})
	go func() { _, _ = m.cmd.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	m.launched = false
	m.cmd = nil
}

// Launched reports whether cerebro started (and therefore owns) the backend.
func (m *Manager) Launched() bool { return m.launched }

func reachable(ctx context.Context, url string) bool {
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

func expandHome(s string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return s
	}
	if strings.HasPrefix(s, "~/") {
		s = home + "/" + strings.TrimPrefix(s, "~/")
	}
	return strings.ReplaceAll(s, " ~/", " "+home+"/")
}
