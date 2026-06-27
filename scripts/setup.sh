#!/usr/bin/env bash
# Create the dedicated cerebro Python venv and install the MLX runtimes used to
# serve the default Gemma model. Equivalent to `cerebro --setup`.
#
# Requires `uv` (https://docs.astral.sh/uv/). The Gemma weights are downloaded
# lazily by the server on first run (or reused from your Hugging Face cache).
set -euo pipefail

DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/cerebro"
VENV_DIR="$DATA_DIR/venv"

if ! command -v uv >/dev/null 2>&1; then
  echo "error: 'uv' not found on PATH. Install it first: https://docs.astral.sh/uv/" >&2
  exit 1
fi

mkdir -p "$DATA_DIR"

echo "Creating venv at $VENV_DIR ..."
uv venv "$VENV_DIR"

echo "Installing mlx-lm and mlx-vlm (this can take a few minutes) ..."
uv pip install --python "$VENV_DIR/bin/python" mlx-lm mlx-vlm

echo
echo "Setup complete."
echo "  python: $VENV_DIR/bin/python"
echo "Run 'cerebro' (or 'go run ./cmd/cerebro') to start."
