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

POLKIT_RULE="/etc/polkit-1/rules.d/49-reefpi.rules"

log "Installing polkit rule for reef-pi service control"

mkdir -p /etc/polkit-1/rules.d

cat > "${POLKIT_RULE}" <<'EOF'
polkit.addRule(function(action, subject) {
    if (subject.user == "reefpi") {
        if (action.id == "org.freedesktop.systemd1.manage-units") {
            var unit = action.lookup("unit");
            var verb = action.lookup("verb");
            if (unit == "reef-pi.service" && verb == "restart") {
                return polkit.Result.YES;
            }
        }

        if (action.id == "org.freedesktop.login1.reboot" ||
            action.id == "org.freedesktop.login1.reboot-multiple-sessions" ||
            action.id == "org.freedesktop.login1.power-off" ||
            action.id == "org.freedesktop.login1.power-off-multiple-sessions") {
            return polkit.Result.YES;
        }
    }
});
EOF

chmod 0644 "${POLKIT_RULE}"

log "Reloading polkit"
systemctl restart polkit >/dev/null 2>&1 || true

log "Restarting service: ${SERVICE}"
systemctl daemon-reload || true
systemctl enable "${SERVICE}.service" >/dev/null 2>&1 || true
systemctl restart "${SERVICE}.service"

log "Service status:"
systemctl --no-pager status "${SERVICE}.service" || true

log "Done ✅"
