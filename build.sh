sudo tee /opt/reef-pi/pack_debs.sh >/dev/null <<'BASH'
#!/usr/bin/env bash
set -euo pipefail
trap 'echo "[pack] ERROR line $LINENO: $BASH_COMMAND" >&2' ERR

VER="v0.1.0"
PKG="reef-pi"
BASE_DIR="/opt"

RP="${BASE_DIR}/reef-pi"
DR="${BASE_DIR}/drivers"
HAL="${BASE_DIR}/hal"
RPI="${BASE_DIR}/rpi"

# Matches your installer layout (keeping as-is):
INSTALL_BIN="/usr/local/bin/reef-pi"
SERVICE_NAME="reef-pi"
APP_USER="reefpi"
APP_GROUP="reefpi"
DATA_DIR="/var/lib/reef-pi"
LOG_DIR="/var/log/reef-pi"
SUDOERS_FILE="/etc/sudoers.d/reefpi-systemctl"

# ---- NEW: UI packaging controls ----
# Default: package UI inside the .deb.
# You can disable with: BUILD_UI=false sudo ./pack_debs.sh
BUILD_UI="${BUILD_UI:-true}"

# Where the .deb will place the UI assets (recommended vs /var/lib).
# reef-pi will serve UI from WorkingDirectory.
UI_DEST="/usr/share/reef-pi/ui"

# Optional: set Node heap when building UI (small Pis).
NODE_HEAP_MB="${NODE_HEAP_MB:-768}"
# -----------------------------------

DIST="${RP}/dist"
WORK="${RP}/.pkgwork"
GO_PKG="./commands"
BIN_NAME="reef-pi"

log(){ echo -e "\n[pack] $*\n"; }
die(){ echo "[pack] ERROR: $*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "Missing dependency: $1"; }

need go
need dpkg-deb
need awk
need install
need rsync
need find

[[ -d "$RP/.git" ]] || die "Expected repo at $RP"
[[ -f "$RP/go.mod" ]] || die "No go.mod in $RP"
[[ -f "$DR/go.mod" && -f "$HAL/go.mod" && -f "$RPI/go.mod" ]] || die "Missing /opt drivers/hal/rpi go.mod files"

rm -rf "$DIST" "$WORK"
mkdir -p "$DIST" "$WORK/bin"

wire_replaces() {
  log "Wiring local replaces in $RP/go.mod -> ../drivers ../hal ../rpi"
  (
    cd "$RP"
    MOD_DRIVERS=$(awk '/^module /{print $2; exit}' "$DR/go.mod")
    MOD_HAL=$(awk '/^module /{print $2; exit}' "$HAL/go.mod")
    MOD_RPI=$(awk '/^module /{print $2; exit}' "$RPI/go.mod")

    [[ -n "$MOD_DRIVERS" && -n "$MOD_HAL" && -n "$MOD_RPI" ]] || die "Could not detect module paths"
    go mod edit -replace "$MOD_DRIVERS=../drivers"
    go mod edit -replace "$MOD_HAL=../hal"
    go mod edit -replace "$MOD_RPI=../rpi"
    go mod tidy
  )
}

detect_ui_index() {
  # Search for typical build outputs (build/ or dist/) containing index.html
  find "$RP" -maxdepth 8 -type f -name index.html \
    \( -path "*/build/*" -o -path "*/dist/*" \) 2>/dev/null | head -n 1 || true
}

pick_yarn() {
  if command -v yarn >/dev/null 2>&1; then
    echo "yarn"
  elif command -v yarnpkg >/dev/null 2>&1; then
    echo "yarnpkg"
  else
    echo ""
  fi
}

build_ui_if_needed() {
  if [[ "$BUILD_UI" != "true" ]]; then
    log "BUILD_UI=false; skipping UI build and packaging"
    return 0
  fi

  # Node is needed for UI build. We don't auto-install deps in a pack script.
  need node
  need npm

  local yarn_cmd
  yarn_cmd="$(pick_yarn)"
  [[ -n "$yarn_cmd" ]] || die "Missing yarn/yarnpkg. Install it or run with BUILD_UI=false."

  log "Building UI with ${yarn_cmd} (NODE_HEAP_MB=${NODE_HEAP_MB})..."
  (
    cd "$RP"
    export NODE_OPTIONS="--max-old-space-size=${NODE_HEAP_MB}"

    # If your repo requires install first, do it. If already installed, this is fast.
    "${yarn_cmd}" install --frozen-lockfile 2>/dev/null || "${yarn_cmd}" install
    "${yarn_cmd}" build
  )

  local ui_index
  ui_index="$(detect_ui_index)"
  [[ -n "$ui_index" ]] || die "UI build finished but index.html not found under */build/* or */dist/*."
  log "UI build output detected at: $(dirname "$ui_index")"
}

stage_ui_into_pkgroot() {
  local pkgroot="$1"

  if [[ "$BUILD_UI" != "true" ]]; then
    return 0
  fi

  local ui_index ui_src
  ui_index="$(detect_ui_index)"
  [[ -n "$ui_index" ]] || die "UI not found. Run with BUILD_UI=true (default) to build it, or BUILD_UI=false to skip."
  ui_src="$(dirname "$ui_index")"

  log "Staging UI into package: ${UI_DEST}"
  mkdir -p "${pkgroot}${UI_DEST}"
  rsync -a --delete "${ui_src}/" "${pkgroot}${UI_DEST}/"
}

build_go() {
  local arch="$1"       # armhf or amd64 label for our outputs
  local goarch="$2"     # arm or amd64
  local goarm="${3:-}"  # 7 for armhf
  local out="$4"

  log "Building binary for ${arch} -> ${out}"
  (
    cd "$RP"
    export GOOS=linux
    export GOARCH="$goarch"
    [[ -n "$goarm" ]] && export GOARM="$goarm"
    export CGO_ENABLED=0

    # Clean is optional, but keeps builds reproducible
    go clean -cache -testcache
    go build -buildvcs=false -trimpath -ldflags "-s -w" -o "$out" "$GO_PKG"
  )
  chmod 0755 "$out"
}

make_deb() {
  local deb_arch="$1"     # Debian arch: armhf / amd64
  local binpath="$2"
  local outname="$3"

  local pkgroot="${WORK}/${PKG}-${deb_arch}"
  rm -rf "$pkgroot"
  mkdir -p \
    "${pkgroot}/DEBIAN" \
    "$(dirname "${pkgroot}${INSTALL_BIN}")" \
    "${pkgroot}/etc/systemd/system" \
    "${pkgroot}${DATA_DIR}" \
    "${pkgroot}${LOG_DIR}" \
    "${pkgroot}/etc/sudoers.d" \
    "${pkgroot}${UI_DEST}"

  # control
  cat > "${pkgroot}/DEBIAN/control" <<EOF
Package: ${PKG}
Version: ${VER#v}
Section: admin
Priority: optional
Architecture: ${deb_arch}
Maintainer: Epicfatigue
Description: reef-pi fork packaged release
Depends: systemd
EOF

  # payload: binary
  install -m 0755 "$binpath" "${pkgroot}${INSTALL_BIN}"

  # payload: UI (optional)
  stage_ui_into_pkgroot "$pkgroot"

  # Decide WorkingDirectory based on whether UI is packaged
  local working_dir
  if [[ "$BUILD_UI" == "true" ]]; then
    working_dir="${UI_DEST}"
  else
    working_dir="${DATA_DIR}"
  fi

  # payload: systemd service
  cat > "${pkgroot}/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Reef-Pi Controller
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}

WorkingDirectory=${working_dir}

ExecStart=${INSTALL_BIN}
Environment=REEFPI_TELEMETRY_DEBUG=0
Environment=REEFPI_TELEMETRY_SLOW_LOCK_MS=10
Environment=REEFPI_TELEMETRY_SLOW_HOLD_MS=10

Restart=on-failure
RestartSec=5

StandardOutput=journal
StandardError=journal

NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

  # payload: sudoers rule (same as your installer when ALLOW_POWER=false)
  cat > "${pkgroot}${SUDOERS_FILE}" <<EOF
# Installed by reef-pi .deb.
# Allow ${APP_USER} to restart reef-pi without a password.
${APP_USER} ALL=(root) NOPASSWD: /bin/systemctl restart ${SERVICE_NAME}.service
EOF
  chmod 0440 "${pkgroot}${SUDOERS_FILE}"

  # maint scripts
  cat > "${pkgroot}/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e

APP_USER="reefpi"
APP_GROUP="reefpi"
DATA_DIR="/var/lib/reef-pi"
LOG_DIR="/var/log/reef-pi"
SERVICE_NAME="reef-pi"
SUDOERS_FILE="/etc/sudoers.d/reefpi-systemctl"

# Create system user if missing
if ! id -u "$APP_USER" >/dev/null 2>&1; then
  useradd --system --create-home --shell /usr/sbin/nologin --user-group "$APP_USER"
fi

# Add hardware access groups if they exist
for g in gpio i2c spi dialout; do
  getent group "$g" >/dev/null 2>&1 && usermod -aG "$g" "$APP_USER" || true
done

mkdir -p "$DATA_DIR" "$LOG_DIR"
chown -R "$APP_USER:$APP_GROUP" "$DATA_DIR" "$LOG_DIR"
chmod 775 "$DATA_DIR" "$LOG_DIR"

# sudoers validate if visudo exists (best effort)
if command -v visudo >/dev/null 2>&1; then
  if ! visudo -cf "$SUDOERS_FILE"; then
    echo "[reef-pi] WARNING: sudoers file invalid: $SUDOERS_FILE" >&2
  fi
fi

systemctl daemon-reload || true
systemctl enable "${SERVICE_NAME}.service" >/dev/null 2>&1 || true
systemctl restart "${SERVICE_NAME}.service" >/dev/null 2>&1 || true
exit 0
EOF
  chmod 0755 "${pkgroot}/DEBIAN/postinst"

  cat > "${pkgroot}/DEBIAN/prerm" <<'EOF'
#!/bin/sh
set -e
systemctl stop reef-pi.service >/dev/null 2>&1 || true
exit 0
EOF
  chmod 0755 "${pkgroot}/DEBIAN/prerm"

  # build deb
  log "Building ${outname} (Architecture=${deb_arch})"
  dpkg-deb --build "$pkgroot" "${DIST}/${outname}"
}

# ---- do it ----
wire_replaces

# NEW: build UI once (if enabled) so it can be packaged
build_ui_if_needed

# Build both binaries on this machine
build_go "armhf" "arm" "7" "${WORK}/bin/${BIN_NAME}-armhf"
build_go "amd64" "amd64" "" "${WORK}/bin/${BIN_NAME}-amd64"

# Package with your requested names
make_deb "armhf" "${WORK}/bin/${BIN_NAME}-armhf" "${PKG}-${VER}.deb"
make_deb "amd64" "${WORK}/bin/${BIN_NAME}-amd64" "${PKG}-${VER}-x86.deb"

log "Done. Outputs:"
ls -lh "$DIST"
BASH