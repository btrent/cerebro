package app

import (
	"fmt"
	"os"
	"os/exec"

	"cerebro/internal/paths"
)

// Setup creates the dedicated cerebro Python venv and installs the MLX runtimes
// (mlx-lm for text, mlx-vlm for vision) needed to serve the default Gemma model.
// It streams the underlying command output so the user can see progress. The
// model weights themselves are not downloaded here — they come from the Hugging
// Face cache the first time the server runs.
func Setup() error {
	uv, err := exec.LookPath("uv")
	if err != nil {
		return fmt.Errorf("`uv` not found on PATH.\n" +
			"Install it from https://docs.astral.sh/uv/ (e.g. `brew install uv`), then re-run `cerebro --setup`")
	}

	if err := paths.EnsureDir(paths.DataDir()); err != nil {
		return err
	}

	venv := paths.VenvDir()
	fmt.Printf("Creating venv at %s …\n", venv)
	if err := run(uv, "venv", venv); err != nil {
		return fmt.Errorf("creating venv: %w", err)
	}

	fmt.Println("Installing mlx-lm and mlx-vlm (this can take a few minutes) …")
	if err := run(uv, "pip", "install", "--python", paths.VenvPython(), "mlx-lm", "mlx-vlm"); err != nil {
		return fmt.Errorf("installing MLX packages: %w", err)
	}

	fmt.Printf("\nSetup complete.\n  venv:   %s\n  python: %s\n\nRun `cerebro` to start chatting.\n",
		venv, paths.VenvPython())
	return nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
