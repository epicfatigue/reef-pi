#!/usr/bin/env bash
set -euo pipefail

REPO="epicfatigue/reef-pi"
SERVICE="reef-pi"

log(){ echo -e "\n[reef-pi] $*\n"; }
die(){ echo "[reef-pi] ERROR: $*" >&2; exit 1; }

need(){ command -v "$1" >/dev/null 2>&1 || die "Missing dependency: $1"; }

if [[ "${EUID}" -ne 0 ]]; then
  die "Run as root: curl -fsSL <url> | sudo bash"
fi

need curl
need dpkg
need systemctl
need uname

arch="$(dpkg --print-architecture)"
case "$arch" in
  armhf)  asset="reef-pi-armhf.deb" ;;
  arm64)  asset="reef-pi-arm64.deb" ;;
  amd64)  asset="reef-pi-amd64.deb" ;;
  *)
    die "Unsupported architecture: ${arch} (expected armhf/arm64/amd64)"
    ;;
esac

url="https://github.com/${REPO}/releases/latest/download/${asset}"
tmp="$(mktemp -d)"
deb="${tmp}/${asset}"

log "Detected arch: ${arch}"
log "Downloading latest release asset: ${url}"
curl -fL --retry 5 --retry-delay 2 -o "${deb}" "${url}"

log "Installing ${deb}"
dpkg -i "${deb}" || true

# If deps missing, fix
if command -v apt-get >/dev/null 2>&1; then
  log "Fixing dependencies (apt-get -f install)"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -y
  apt-get -f install -y
fi

log "Restarting service: ${SERVICE}"
systemctl daemon-reload || true
systemctl enable "${SERVICE}.service" >/dev/null 2>&1 || true
systemctl restart "${SERVICE}.service"

log "Service status:"
systemctl --no-pager status "${SERVICE}.service" || true

log "Done ✅"