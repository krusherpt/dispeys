#!/bin/bash
set -euo pipefail

INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="dispeys"
SERVICE_FILE="dispeys.service"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[+]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
error() { echo -e "${RED}[-]${NC} $*"; exit 1; }

# Check root
if [ "$(id -u)" -ne 0 ]; then
    error "This script must be run as root (use sudo)"
fi

# Check we're in the project root
if [ ! -f "go.mod" ]; then
    error "go.mod not found. Run this script from the project root."
fi

# Check dependencies
for cmd in go gcc xdotool xprop wmctrl; do
    if ! command -v "$cmd" &>/dev/null; then
        warn "Missing dependency: $cmd — install it before running this script"
    fi
done

info "Building dispeys..."
CGO_ENABLED=1 go build -o dispeysController -a -gcflags="all=-l -B" -ldflags="-s -w" cmd/controller/main.go

info "Installing binary to ${INSTALL_DIR}/..."
cp dispeysController "${INSTALL_DIR}/${SERVICE_NAME}"
chmod 755 "${INSTALL_DIR}/${SERVICE_NAME}"
rm -f dispeysController

USER="${SUDO_USER:-$USER}"
if ! id "$USER" &>/dev/null; then
    error "User $USER not found"
fi

info "Installing systemd user service (running as $USER)..."
mkdir -p "/home/${USER}/.config/systemd/user"
cat > "/home/${USER}/.config/systemd/user/${SERVICE_FILE}" <<EOF
[Unit]
Description=Dispeys - Stream Deck Tray Controller
After=graphical.target
Wants=graphical.target

[Service]
Type=simple
Environment=DISPLAY=:0
ExecStart=${INSTALL_DIR}/${SERVICE_NAME}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF
chown -R "${USER}:${USER}" "/home/${USER}/.config/systemd"

echo ""
echo "Enable with:"
echo "  loginctl enable-linger ${USER}"
echo "  systemctl --user enable dispeys"
echo "  systemctl --user start dispeys"
echo ""
echo "Check status with:"
echo "  systemctl --user status dispeys"

systemctl daemon-reload

info "Enabling and starting service..."
loginctl enable-linger "${USER}"
sudo -u "$USER" bash -c 'export XDG_RUNTIME_DIR=/run/user/$(id -u); systemctl --user enable --now dispeys'
info "Service installed and started."
info "Check status with:"
echo "  systemctl --user status dispeys"
