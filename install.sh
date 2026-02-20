#!/usr/bin/env bash
set -euo pipefail

log() { echo -e "\n[reef-pi] $*\n"; }
die() { echo "[reef-pi] ERROR: $*" >&2; exit 1; }

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    die "Please run as root (use: sudo bash install.sh)"
  fi
}

# -----------------------------
# CONFIG
# -----------------------------
BASE_DIR_DEFAULT="/opt"

REPO_REEFPI_DEFAULT="https://github.com/epicfatigue/reef-pi.git"
REPO_DRIVERS_DEFAULT="https://github.com/epicfatigue/drivers.git"
REPO_HAL_DEFAULT="https://github.com/epicfatigue/hal.git"
REPO_RPI_DEFAULT="https://github.com/epicfatigue/rpi.git"

BRANCH_DEFAULT="main"

APP_NAME="reef-pi"
SERVICE_NAME="reef-pi"

APP_USER_DEFAULT="reefpi"
APP_GROUP_DEFAULT="reefpi"

DATA_DIR_DEFAULT="/var/lib/reef-pi"
LOG_DIR_DEFAULT="/var/log/reef-pi"

# If your UI needs building, set BUILD_UI=true when running:
#   sudo BUILD_UI=true bash install.sh
BUILD_UI_DEFAULT="true"

# Optional: allow passwordless reboot/poweroff too (default false)
#   sudo ALLOW_POWER=true bash install.sh
ALLOW_POWER_DEFAULT="false"

DEFAULT_PORT="8080"
# -----------------------------

install_packages() {
  log "Updating apt metadata and upgrading system..."
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -y
  apt-get upgrade -y

  log "Installing required packages (git, build tools, Go, Node, yarn prerequisites)..."
  apt-get install -y \
    ca-certificates \
    curl \
    git \
    jq \
    build-essential \
    pkg-config \
    systemd \
    rsync \
    sudo

  # Go
  if ! command -v go >/dev/null 2>&1; then
    log "Installing Go..."
    apt-get install -y golang || die "Failed to install golang. Install manually then re-run."
  fi

  # Node.js + npm (needed for yarn/UI)
  if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
    log "Installing Node.js + npm..."
    apt-get install -y nodejs npm || die "Failed to install nodejs/npm."
  fi

  # Yarn: Prefer corepack if available; otherwise install Debian's yarnpkg.
  if ! command -v yarn >/dev/null 2>&1; then
    if command -v corepack >/dev/null 2>&1; then
      log "Enabling Yarn via corepack..."
      corepack enable || true
      corepack prepare yarn@stable --activate || true
    fi
  fi

  if ! command -v yarn >/dev/null 2>&1; then
    log "Installing Yarn (Debian package: yarnpkg)..."
    apt-get install -y yarnpkg || die "Could not install yarn/corepack. Install yarn manually then re-run."
    # Debian calls it yarnpkg, may not provide /usr/bin/yarn everywhere; shim if needed.
    if ! command -v yarn >/dev/null 2>&1 && command -v yarnpkg >/dev/null 2>&1; then
      ln -sf "$(command -v yarnpkg)" /usr/local/bin/yarn
    fi
  fi

  log "Versions:"
  (go version || true)
  (node -v || true)
  (npm -v || true)
  (yarn -v || true)
  (git --version || true)
}

create_user_and_dirs() {
  log "Creating service user: ${APP_USER}"
  if ! id -u "${APP_USER}" >/dev/null 2>&1; then
    useradd --system --create-home --shell /usr/sbin/nologin --user-group "${APP_USER}"
  fi

  # Hardware access groups (if present)
  for g in gpio i2c spi dialout; do
    if getent group "$g" >/dev/null 2>&1; then
      usermod -aG "$g" "${APP_USER}"
    fi
  done

  log "Creating base directories..."
  mkdir -p "${BASE_DIR}" "${DATA_DIR}" "${LOG_DIR}"

  log "Setting ownership of data/log dirs..."
  chown -R "${APP_USER}:${APP_GROUP}" "${DATA_DIR}" "${LOG_DIR}"
  chmod 775 "${DATA_DIR}" "${LOG_DIR}"
}

prepare_opt_repos() {
  log "Preparing repo directories under ${BASE_DIR}..."
  mkdir -p "${BASE_DIR}/reef-pi" "${BASE_DIR}/drivers" "${BASE_DIR}/hal" "${BASE_DIR}/rpi"
  chown -R "${APP_USER}:${APP_GROUP}" "${BASE_DIR}/reef-pi" "${BASE_DIR}/drivers" "${BASE_DIR}/hal" "${BASE_DIR}/rpi"

  # Make sure base dir itself is accessible (normally already is)
  chmod 755 "${BASE_DIR}" || true

  # Sanity check: can the service user write to these paths?
  if ! sudo -u "${APP_USER}" test -w "${BASE_DIR}/drivers"; then
    die "${BASE_DIR}/drivers is not writable by ${APP_USER}. Check permissions on ${BASE_DIR}."
  fi
}

write_sudoers_rules() {
  log "Installing sudoers rules for ${APP_USER} (passwordless systemctl restart)..."

  local sudoers_file="/etc/sudoers.d/${APP_USER}-systemctl"

  if [[ "${ALLOW_POWER}" == "true" ]]; then
    cat > "${sudoers_file}" <<EOF
# Installed by reef-pi installer.
# Allow ${APP_USER} to restart reef-pi, and optionally reboot/poweroff, without a password.
${APP_USER} ALL=(root) NOPASSWD: /bin/systemctl restart ${SERVICE_NAME}.service, /bin/systemctl reboot, /bin/systemctl poweroff
EOF
  else
    cat > "${sudoers_file}" <<EOF
# Installed by reef-pi installer.
# Allow ${APP_USER} to restart reef-pi without a password.
${APP_USER} ALL=(root) NOPASSWD: /bin/systemctl restart ${SERVICE_NAME}.service
EOF
  fi

  chmod 0440 "${sudoers_file}"

  # Validate sudoers syntax; remove file if invalid
  if ! visudo -cf "${sudoers_file}"; then
    rm -f "${sudoers_file}"
    die "Invalid sudoers file generated; removed ${sudoers_file}"
  fi

  log "Sudoers installed: ${sudoers_file} (ALLOW_POWER=${ALLOW_POWER})"
}

clone_or_update_repo() {
  local url="$1"
  local dest="$2"
  local branch="$3"

  if [[ -d "${dest}/.git" ]]; then
    log "Updating ${dest}..."
    sudo -u "${APP_USER}" git -C "${dest}" fetch --all
    sudo -u "${APP_USER}" git -C "${dest}" checkout "${branch}"
    sudo -u "${APP_USER}" git -C "${dest}" pull --ff-only
  else
    log "Cloning ${url} -> ${dest}..."
    mkdir -p "${dest}"
    chown -R "${APP_USER}:${APP_GROUP}" "${dest}"
    # Clone directly into destination
    sudo -u "${APP_USER}" git clone --branch "${branch}" "${url}" "${dest}"
  fi
}

clone_all() {
  log "Cloning all repositories under ${BASE_DIR}..."
  clone_or_update_repo "${REPO_DRIVERS}" "${BASE_DIR}/drivers" "${BRANCH}"
  clone_or_update_repo "${REPO_HAL}"     "${BASE_DIR}/hal"     "${BRANCH}"
  clone_or_update_repo "${REPO_RPI}"     "${BASE_DIR}/rpi"     "${BRANCH}"
  clone_or_update_repo "${REPO_REEFPI}"  "${BASE_DIR}/reef-pi" "${BRANCH}"

  # Ensure ownership (in case /opt existed with different perms)
  chown -R "${APP_USER}:${APP_GROUP}" "${BASE_DIR}/drivers" "${BASE_DIR}/hal" "${BASE_DIR}/rpi" "${BASE_DIR}/reef-pi"
}

wire_local_replaces() {
  local rp="${BASE_DIR}/reef-pi"
  [[ -f "${rp}/go.mod" ]] || die "No go.mod found in ${rp}. Is this the correct repo?"

  log "Wiring local Go module replacements in reef-pi (../drivers, ../hal, ../rpi)..."

  sudo -u "${APP_USER}" bash -lc "
    set -e
    cd '${rp}'

    MOD_DRIVERS=\$(awk '/^module /{print \$2; exit}' '${BASE_DIR}/drivers/go.mod' || true)
    MOD_HAL=\$(awk '/^module /{print \$2; exit}' '${BASE_DIR}/hal/go.mod' || true)
    MOD_RPI=\$(awk '/^module /{print \$2; exit}' '${BASE_DIR}/rpi/go.mod' || true)

    [[ -n \"\$MOD_DRIVERS\" ]] || { echo 'Could not detect drivers module path'; exit 1; }
    [[ -n \"\$MOD_HAL\" ]]     || { echo 'Could not detect hal module path'; exit 1; }
    [[ -n \"\$MOD_RPI\" ]]     || { echo 'Could not detect rpi module path'; exit 1; }

    go mod edit -replace \"\$MOD_DRIVERS=../drivers\"
    go mod edit -replace \"\$MOD_HAL=../hal\"
    go mod edit -replace \"\$MOD_RPI=../rpi\"

    go mod tidy
  "
}

build_ui_if_present() {
  local rp="${BASE_DIR}/reef-pi"

  if [[ "${BUILD_UI}" != "true" ]]; then
    log "BUILD_UI is false; skipping UI build."
    return 0
  fi

  local ui_dir=""
  if [[ -f "${rp}/ui/package.json" ]]; then ui_dir="${rp}/ui"; fi
  if [[ -z "${ui_dir}" && -f "${rp}/frontend/package.json" ]]; then ui_dir="${rp}/frontend"; fi
  if [[ -z "${ui_dir}" && -f "${rp}/front-end/package.json" ]]; then ui_dir="${rp}/front-end"; fi

  if [[ -z "${ui_dir}" ]]; then
    log "No UI directory with package.json found; skipping UI build."
    return 0
  fi

  log "Building UI in ${ui_dir} using yarn..."
  sudo -u "${APP_USER}" bash -lc "
    set -e
    cd '${ui_dir}'
    yarn install --frozen-lockfile || yarn install
    yarn build
  "
}

build_backend_and_install() {
  local rp="${BASE_DIR}/reef-pi"

  log "Building reef-pi backend..."
  if [[ -f "${rp}/Makefile" ]]; then
    if grep -qE '(^|\s)build(\s|:|$)' "${rp}/Makefile"; then
      sudo -u "${APP_USER}" make -C "${rp}" build
    else
      sudo -u "${APP_USER}" bash -lc "cd '${rp}' && go build -o '${APP_NAME}' ./"
    fi
  else
    sudo -u "${APP_USER}" bash -lc "cd '${rp}' && go build -o '${APP_NAME}' ./"
  fi

  local bin_src=""
  if [[ -f "${rp}/${APP_NAME}" ]]; then
    bin_src="${rp}/${APP_NAME}"
  elif [[ -f "${rp}/bin/${APP_NAME}" ]]; then
    bin_src="${rp}/bin/${APP_NAME}"
  else
    die "Build finished but ${APP_NAME} binary not found in ${rp} or ${rp}/bin"
  fi

  log "Installing binary to /usr/local/bin/${APP_NAME}..."
  install -m 0755 "${bin_src}" "/usr/local/bin/${APP_NAME}"
  chown root:root "/usr/local/bin/${APP_NAME}"
}

write_service_file() {
  log "Creating systemd service: ${SERVICE_NAME}.service"

  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Reef-Pi Controller
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}

WorkingDirectory=${DATA_DIR}

# If your fork needs config flags, add them here.
# Example:
# ExecStart=/usr/local/bin/${APP_NAME} --config ${DATA_DIR}/config.yml
ExecStart=/usr/local/bin/${APP_NAME}
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

  systemctl daemon-reload
}

enable_and_start() {
  log "Enabling ${SERVICE_NAME} at boot and starting it now..."
  systemctl enable "${SERVICE_NAME}.service"
  systemctl restart "${SERVICE_NAME}.service"

  log "Service status:"
  systemctl --no-pager status "${SERVICE_NAME}.service" || true

  log "Follow logs with:"
  echo "  sudo journalctl -u ${SERVICE_NAME} -f"
}

print_summary() {
  log "Install complete ✅"
  echo "Repos installed under: ${BASE_DIR}"
  echo "  - ${BASE_DIR}/reef-pi"
  echo "  - ${BASE_DIR}/drivers"
  echo "  - ${BASE_DIR}/hal"
  echo "  - ${BASE_DIR}/rpi"
  echo
  echo "Binary:   /usr/local/bin/${APP_NAME}"
  echo "Data dir: ${DATA_DIR}"
  echo "Logs:     journalctl -u ${SERVICE_NAME} -f"
  echo
  echo "Sudoers:  /etc/sudoers.d/${APP_USER}-systemctl (ALLOW_POWER=${ALLOW_POWER})"
  echo
  echo "If reef-pi listens on port ${DEFAULT_PORT}:"
  echo "  http://<your-pi-ip>:${DEFAULT_PORT}/"
}

# -----------------------------
# Main
# -----------------------------
require_root

BASE_DIR="${BASE_DIR:-$BASE_DIR_DEFAULT}"

REPO_REEFPI="${REPO_REEFPI:-$REPO_REEFPI_DEFAULT}"
REPO_DRIVERS="${REPO_DRIVERS:-$REPO_DRIVERS_DEFAULT}"
REPO_HAL="${REPO_HAL:-$REPO_HAL_DEFAULT}"
REPO_RPI="${REPO_RPI:-$REPO_RPI_DEFAULT}"
BRANCH="${BRANCH:-$BRANCH_DEFAULT}"

APP_USER="${APP_USER:-$APP_USER_DEFAULT}"
APP_GROUP="${APP_GROUP:-$APP_GROUP_DEFAULT}"

DATA_DIR="${DATA_DIR:-$DATA_DIR_DEFAULT}"
LOG_DIR="${LOG_DIR:-$LOG_DIR_DEFAULT}"

BUILD_UI="${BUILD_UI:-$BUILD_UI_DEFAULT}"
ALLOW_POWER="${ALLOW_POWER:-$ALLOW_POWER_DEFAULT}"

log "Starting installer"
log "BASE_DIR=${BASE_DIR}"
log "BRANCH=${BRANCH}"
log "BUILD_UI=${BUILD_UI}"
log "ALLOW_POWER=${ALLOW_POWER}"

install_packages
create_user_and_dirs
prepare_opt_repos
write_sudoers_rules
clone_all
wire_local_replaces
build_ui_if_present
build_backend_and_install
write_service_file
enable_and_start
print_summary