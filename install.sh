#!/usr/bin/env bash
#
# install.sh — Install dispeys as a system-wide binary with systemd user service.
#
# Usage: sudo ./install.sh [--help]
#
# This script installs the dispeysController binary to /usr/bin and places
# a systemd user service in /usr/lib/systemd/user/ (matching the PKGBUILD).
# After installation, enable the service per-user with:
#   loginctl enable-linger $USER
#   systemctl --user enable --now dispeys

set -euo pipefail

# ── Colors & helpers ──────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[+]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
error() { echo -e "${RED}[-]${NC} $*"; exit 1; }

# ── Constants (must match PKGBUILD) ──────────────────────────────────
BIN_NAME="dispeysController"
BIN_DIR="/usr/bin"
SERVICE_NAME="dispeys.service"
SERVICE_DIR="/usr/lib/systemd/user"

# ── Usage ─────────────────────────────────────────────────────────────
usage() {
    echo "Usage: sudo $0 [--help]"
    echo ""
    echo "Install dispeys system-wide and set up the systemd user service."
    echo "Run as root (sudo)."
    exit 0
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    usage
fi

# ── Root check ────────────────────────────────────────────────────────
if [[ "$(id -u)" -ne 0 ]]; then
    error "This script must be run as root (use sudo)"
fi

# ── Project root check ────────────────────────────────────────────────
if [[ ! -f "go.mod" ]]; then
    error "go.mod not found. Run this script from the project root."
fi

# ── Dependency check (fail hard if missing) ──────────────────────────
missing=()
for cmd in go gcc; do
    if ! command -v "$cmd" &>/dev/null; then
        missing+=("$cmd")
    fi
done

if [[ ${#missing[@]} -gt 0 ]]; then
    error "Missing required build dependencies: ${missing[*]}"
fi

for cmd in xdotool xprop wmctrl; do
    if ! command -v "$cmd" &>/dev/null; then
        warn "Optional dependency missing: $cmd — install it for full functionality"
    fi
done

# ── Build ─────────────────────────────────────────────────────────────
info "Building ${BIN_NAME}..."
CGO_ENABLED=1 go build -o "${BIN_NAME}" -a -gcflags="all=-l -B" -ldflags="-s -w" cmd/controller/main.go

if [[ ! -f "${BIN_NAME}" ]]; then
    error "Build failed — binary not produced"
fi

# ── Install binary ───────────────────────────────────────────────────
info "Installing binary to ${BIN_DIR}/..."
install -Dm755 "${BIN_NAME}" "${BIN_DIR}/${BIN_NAME}"
rm -f "${BIN_NAME}"

# ── Install systemd service ──────────────────────────────────────────
if [[ ! -f "${SERVICE_NAME}" ]]; then
    error "${SERVICE_NAME} not found. This file must exist alongside install.sh"
fi

info "Installing systemd service to ${SERVICE_DIR}/..."
install -Dm644 "${SERVICE_NAME}" "${SERVICE_DIR}/${SERVICE_NAME}"

# ── Post-install setup ───────────────────────────────────────────────
# Determine the invoking user (works with sudo)
USER="${SUDO_USER:-$(logname 2>/dev/null || whoami)}"
if ! id "$USER" &>/dev/null; then
    error "Cannot determine user. Run with sudo or set SUDO_USER."
fi

info "Service installed. Enable and start with:"
echo ""
echo "  loginctl enable-linger ${USER}"
echo "  systemctl --user enable --now dispeys"
echo ""
echo "Check status with:"
echo "  systemctl --user status dispeys"
echo "  journalctl --user -u dispeys -f"
echo ""
echo "To uninstall:"
echo "  sudo rm ${BIN_DIR}/${BIN_NAME} ${SERVICE_DIR}/${SERVICE_NAME}"
