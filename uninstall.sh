#!/usr/bin/env bash
set -euo pipefail
trap 'echo "[uninstall] ERROR on line $LINENO: $BASH_COMMAND" >&2' ERR

APP_NAME="reef-pi"
SERVICE_NAME="reef-pi"
APP_USER="reefpi"
DATA_DIR="/var/lib/reef-pi"
LOG_DIR="/var/log/reef-pi"
BASE_DIR="/opt"

die() { echo "[uninstall] ERROR: $*" >&2; exit 1; }
log() { echo -e "\n[uninstall] $*\n"; }

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    die "Please run as root: sudo bash uninstall_reefpi.sh"
  fi
}

require_root

log "Stopping and disabling systemd service (if present)..."
systemctl stop "${SERVICE_NAME}.service" 2>/dev/null || true
systemctl disable "${SERVICE_NAME}.service" 2>/dev/null || true

log "Removing systemd unit file (if present)..."
rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
rm -f "/lib/systemd/system/${SERVICE_NAME}.service"
systemctl daemon-reload
systemctl reset-failed "${SERVICE_NAME}.service" 2>/dev/null || true

log "Removing installed binary (if present)..."
rm -f "/usr/local/bin/${APP_NAME}"

log "Removing sudoers rule (if present)..."
rm -f "/etc/sudoers.d/${APP_USER}-systemctl"
rm -f "/etc/sudoers.d/reefpi-systemctl"

log "Removing app directories..."
rm -rf "${BASE_DIR}/reef-pi" \
       "${BASE_DIR}/drivers" \
       "${BASE_DIR}/hal" \
       "${BASE_DIR}/rpi"

rm -rf "${DATA_DIR}" "${LOG_DIR}"

log "Removing user/group (${APP_USER}) if present..."
if id -u "${APP_USER}" >/dev/null 2>&1; then
  userdel -r "${APP_USER}" 2>/dev/null || userdel "${APP_USER}" || true
fi
getent group "${APP_USER}" >/dev/null 2>&1 && groupdel "${APP_USER}" 2>/dev/null || true

log "Done ✅"
echo "[uninstall] Remaining checks:"
echo "  - which reef-pi        (should be empty)"
echo "  - systemctl status reef-pi (should be not found/inactive)"
echo "  - ls -la /opt          (should not contain reef-pi/drivers/hal/rpi)"
echo "  - ls -la /var/lib      (reef-pi dir should be gone)"