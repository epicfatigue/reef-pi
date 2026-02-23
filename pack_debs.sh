sudo tee /opt/reef-pi/pack_debs.sh >/dev/null <<'BASH'
#!/usr/bin/env bash
set -euo pipefail
trap 'echo "[pack] ERROR line $LINENO: $BASH_COMMAND" >&2' ERR

VER="v0.1.0"
PKG="reef-pi"

# Project layout to ship
OPT_BASE="/opt"
OPT_RP="${OPT_BASE}/reef-pi"
OPT_DR="${OPT_BASE}/drivers"
OPT_HAL="${OPT_BASE}/hal"
OPT_RPI="${OPT_BASE}/rpi"

# Runtime install locations
INSTALL_BIN="/usr/local/bin/reef-pi"
SERVICE_NAME="reef-pi"
APP_USER="reefpi"
APP_GROUP="reefpi"

DATA_DIR="/var/lib/reef-pi"
LOG_DIR="/var/log/reef-pi"
SUDOERS_FILE="/etc/sudoers.d/reefpi-systemctl"

# reef-pi will run from here (key fix)
WORKDIR="/opt/reef-pi"

# DB lives in /var/lib, but app finds it via symlink in WORKDIR
DB_FILE="${DATA_DIR}/reef-pi.db"
DB_LINK="${WORKDIR}/reef-pi.db"

# Binary source: prefer the built one in /opt/reef-pi
BIN_SRC=""
if [[ -x "${OPT_RP}/reef-pi" ]]; then
  BIN_SRC="${OPT_RP}/reef-pi"
elif [[ -x "/var/lib/reef-pi/reef-pi" ]]; then
  BIN_SRC="/var/lib/reef-pi/reef-pi"
else
  echo "[pack] ERROR: Could not find built binary at /opt/reef-pi/reef-pi or /var/lib/reef-pi/reef-pi" >&2
  exit 1
fi

DIST="${OPT_RP}/dist"
WORK="${OPT_RP}/.pkgwork"

log(){ echo -e "\n[pack] $*\n"; }
die(){ echo "[pack] ERROR: $*" >&2; exit 1; }
need(){ command -v "$1" >/dev/null 2>&1 || die "Missing dependency: $1"; }

need dpkg-deb
need rsync
need install
need dpkg

# Sanity
[[ -d "${OPT_RP}"  ]] || die "Missing ${OPT_RP}"
[[ -d "${OPT_DR}"  ]] || die "Missing ${OPT_DR}"
[[ -d "${OPT_HAL}" ]] || die "Missing ${OPT_HAL}"
[[ -d "${OPT_RPI}" ]] || die "Missing ${OPT_RPI}"

rm -rf "$DIST" "$WORK"
mkdir -p "$DIST" "$WORK"

# ---- Excludes: don’t ship build-time junk ----
# Keep runtime essentials: home.html, favicon.ico, assets/, ui/ (built), etc.
RSYNC_RP_EXCLUDES=(
  --exclude '.git/'
  --exclude '.github/'
  --exclude '.vscode/'
  --exclude '.idea/'
  --exclude '.pkgwork/'
  --exclude 'dist/'
  --exclude 'node_modules/'
  --exclude 'front-end/node_modules/'
  --exclude 'front-end/.cache/'
  --exclude 'front-end/build/'
  --exclude 'front-end/dist/'
  --exclude 'front-end/src/'
  --exclude 'front-end/public/'
  --exclude 'front-end/test/'
  --exclude 'front-end/tests/'
  --exclude 'front-end/__tests__/'
  --exclude 'scripts/'
  --exclude 'build/'
  --exclude 'commands/'
  --exclude 'controller/'
  --exclude '*.go'
  --exclude '*.md'
  --exclude 'Makefile'
  --exclude 'Dockerfile'
  --exclude 'Gemfile'
  --exclude 'Gemfile.lock'
  --exclude 'package.json'
  --exclude 'yarn.lock'
  --exclude 'webpack.config.js'
  --exclude 'swagger.yml'
  --exclude 'swagger.json'
  --exclude 'openapi.yaml'
  --exclude 'sass-lint.yml'
  --exclude 'install.sh'
  --exclude 'Compile_and_install.sh'
)

RSYNC_COMMON_EXCLUDES=(
  --exclude '.git/'
  --exclude '.github/'
  --exclude '.pkgwork/'
  --exclude 'dist/'
  --exclude 'node_modules/'
)

make_deb() {
  local deb_arch="$1"
  local outname="$2"

  local pkgroot="${WORK}/${PKG}-${deb_arch}"
  rm -rf "$pkgroot"

  mkdir -p \
    "${pkgroot}/DEBIAN" \
    "$(dirname "${pkgroot}${INSTALL_BIN}")" \
    "${pkgroot}/etc/systemd/system" \
    "${pkgroot}${DATA_DIR}" \
    "${pkgroot}${LOG_DIR}" \
    "${pkgroot}/etc/sudoers.d" \
    "${pkgroot}${OPT_BASE}"

  # control
  cat > "${pkgroot}/DEBIAN/control" <<EOF
Package: ${PKG}
Version: ${VER#v}
Section: admin
Priority: optional
Architecture: ${deb_arch}
Maintainer: Epicfatigue
Description: reef-pi fork packaged release (runtime under /opt, runs from /opt/reef-pi)
Depends: systemd, rsync
EOF

  # binary
  log "Packaging binary ${BIN_SRC} -> ${INSTALL_BIN}"
  install -m 0755 "${BIN_SRC}" "${pkgroot}${INSTALL_BIN}"

  # project trees under /opt (but exclude build junk from reef-pi tree)
  log "Packaging runtime trees into /opt (reef-pi + drivers + hal + rpi)"
  mkdir -p "${pkgroot}${OPT_RP}" "${pkgroot}${OPT_DR}" "${pkgroot}${OPT_HAL}" "${pkgroot}${OPT_RPI}"

  # reef-pi tree: exclude build-time junk
  rsync -a --delete "${RSYNC_RP_EXCLUDES[@]}" "${OPT_RP}/"  "${pkgroot}${OPT_RP}/"

  # drivers/hal/rpi: exclude VCS etc
  rsync -a --delete "${RSYNC_COMMON_EXCLUDES[@]}" "${OPT_DR}/"  "${pkgroot}${OPT_DR}/"
  rsync -a --delete "${RSYNC_COMMON_EXCLUDES[@]}" "${OPT_HAL}/" "${pkgroot}${OPT_HAL}/"
  rsync -a --delete "${RSYNC_COMMON_EXCLUDES[@]}" "${OPT_RPI}/" "${pkgroot}${OPT_RPI}/"

  # Don't ship DB inside /opt/reef-pi
  rm -f "${pkgroot}${OPT_RP}/reef-pi.db" 2>/dev/null || true

  # systemd service
  cat > "${pkgroot}/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Reef-Pi Controller
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}

WorkingDirectory=${WORKDIR}
ExecStart=${INSTALL_BIN} daemon

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

  # sudoers
  cat > "${pkgroot}${SUDOERS_FILE}" <<EOF
# Installed by reef-pi .deb.
${APP_USER} ALL=(root) NOPASSWD: /bin/systemctl restart ${SERVICE_NAME}.service
EOF
  chmod 0440 "${pkgroot}${SUDOERS_FILE}"

  # postinst
  cat > "${pkgroot}/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e

APP_USER="reefpi"
APP_GROUP="reefpi"
DATA_DIR="/var/lib/reef-pi"
LOG_DIR="/var/log/reef-pi"
SERVICE_NAME="reef-pi"
SUDOERS_FILE="/etc/sudoers.d/reefpi-systemctl"
WORKDIR="/opt/reef-pi"
DB_FILE="${DATA_DIR}/reef-pi.db"
DB_LINK="${WORKDIR}/reef-pi.db"

if ! id -u "$APP_USER" >/dev/null 2>&1; then
  useradd --system --create-home --shell /usr/sbin/nologin --user-group "$APP_USER"
fi

for g in gpio i2c spi dialout; do
  getent group "$g" >/dev/null 2>&1 && usermod -aG "$g" "$APP_USER" || true
done

mkdir -p "$DATA_DIR" "$LOG_DIR"
chown -R "$APP_USER:$APP_GROUP" "$DATA_DIR" "$LOG_DIR"
chmod 775 "$DATA_DIR" "$LOG_DIR"

# Allow reefpi to read/write under /opt/reef-pi if needed (matches your current working setup)
chown -R "$APP_USER:$APP_GROUP" "$WORKDIR" || true

# DB symlink
if [ -e "$DB_LINK" ] && [ ! -L "$DB_LINK" ]; then
  if [ ! -e "$DB_FILE" ]; then
    mv "$DB_LINK" "$DB_FILE" || true
  else
    rm -f "$DB_LINK" || true
  fi
fi
ln -sf "$DB_FILE" "$DB_LINK" || true
chown -h "$APP_USER:$APP_GROUP" "$DB_LINK" 2>/dev/null || true

if command -v visudo >/dev/null 2>&1; then
  visudo -cf "$SUDOERS_FILE" >/dev/null 2>&1 || echo "[reef-pi] WARNING: sudoers invalid: $SUDOERS_FILE" >&2
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

  log "Building ${outname} (Architecture=${deb_arch})"
  dpkg-deb --build "$pkgroot" "${DIST}/${outname}"
}

DEB_ARCH="$(dpkg --print-architecture)"
log "Detected build arch: ${DEB_ARCH}"
make_deb "${DEB_ARCH}" "${PKG}-${VER}-${DEB_ARCH}.deb"

log "Done. Outputs:"
ls -lh "$DIST"
BASH