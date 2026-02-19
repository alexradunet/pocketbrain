#!/usr/bin/env bash
set -euo pipefail

REPAIR=false
FORCE=false
YES=false
NON_INTERACTIVE=false
DEEP=false

for arg in "$@"; do
  case "$arg" in
    --repair) REPAIR=true ;;
    --force) FORCE=true ;;
    --yes) YES=true ;;
    --non-interactive) NON_INTERACTIVE=true ;;
    --deep) DEEP=true ;;
    -h|--help)
      cat <<'EOF'
PocketBrain Doctor

Usage:
  make doctor
  make doctor ARGS="--yes"
  make doctor ARGS="--repair"
  make doctor ARGS="--repair --force"
  make doctor ARGS="--non-interactive"
  make doctor ARGS="--deep"

Flags:
  --yes              auto-confirm prompts
  --repair           apply safe repairs
  --force            allow aggressive repairs (with --repair)
  --non-interactive  never prompt
  --deep             run extra checks (typecheck/service diagnostics)
EOF
      exit 0
      ;;
    *)
      printf 'Unknown flag: %s\n' "$arg" >&2
      exit 2
      ;;
  esac
done

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
cd "$REPO_ROOT"

PASS_COUNT=0
WARN_COUNT=0
FAIL_COUNT=0
FIXED_COUNT=0

resolve_bin() {
  local cmd="$1"
  if command -v "$cmd" >/dev/null 2>&1; then
    command -v "$cmd"
    return 0
  fi
  if [[ -x "/usr/sbin/$cmd" ]]; then
    printf '/usr/sbin/%s\n' "$cmd"
    return 0
  fi
  if [[ -x "/sbin/$cmd" ]]; then
    printf '/sbin/%s\n' "$cmd"
    return 0
  fi
  return 1
}

pass() { PASS_COUNT=$((PASS_COUNT + 1)); printf 'PASS  %s\n' "$1"; }
warn() { WARN_COUNT=$((WARN_COUNT + 1)); printf 'WARN  %s\n' "$1"; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); printf 'FAIL  %s\n' "$1"; }
fixed() { FIXED_COUNT=$((FIXED_COUNT + 1)); printf 'FIXED %s\n' "$1"; }

confirm() {
  local prompt="$1"
  if [[ "$NON_INTERACTIVE" == "true" ]]; then
    return 1
  fi
  if [[ "$YES" == "true" ]]; then
    return 0
  fi
  read -r -p "$prompt [y/N]: " answer
  [[ "${answer,,}" == "y" || "${answer,,}" == "yes" ]]
}

safe_repair_enabled() {
  [[ "$REPAIR" == "true" ]]
}

aggressive_repair_enabled() {
  [[ "$REPAIR" == "true" && "$FORCE" == "true" ]]
}

if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
  pass ".env file present"
else
  warn ".env missing (copy .env.example to .env)"
fi

DATA_DIR="${DATA_DIR:-.data}"
if [[ "$DATA_DIR" != /* ]]; then
  DATA_DIR="${REPO_ROOT}/${DATA_DIR}"
fi

VAULT_PATH="${VAULT_PATH:-${DATA_DIR}/vault}"
if [[ "$VAULT_PATH" != /* ]]; then
  VAULT_PATH="${REPO_ROOT}/${VAULT_PATH}"
fi

OPENCODE_CONFIG_DIR="${OPENCODE_CONFIG_DIR:-${VAULT_PATH}/99-system/99-pocketbrain}"
if [[ "$OPENCODE_CONFIG_DIR" != /* ]]; then
  OPENCODE_CONFIG_DIR="${REPO_ROOT}/${OPENCODE_CONFIG_DIR}"
fi

POCKETBRAIN_SKILLS_DIR="${OPENCODE_CONFIG_DIR}/.agents/skills"

WHATSAPP_AUTH_DIR="${WHATSAPP_AUTH_DIR:-${DATA_DIR}/whatsapp-auth}"
if [[ "$WHATSAPP_AUTH_DIR" != /* ]]; then
  WHATSAPP_AUTH_DIR="${REPO_ROOT}/${WHATSAPP_AUTH_DIR}"
fi

ensure_dir() {
  local dir="$1"
  local label="$2"
  if [[ -d "$dir" ]]; then
    if [[ -w "$dir" ]]; then
      pass "$label exists and is writable ($dir)"
    else
      fail "$label exists but is not writable ($dir)"
    fi
    return
  fi

  if safe_repair_enabled && (confirm "Create missing $label at $dir?" || [[ "$YES" == "true" || "$NON_INTERACTIVE" == "true" ]]); then
    mkdir -p "$dir"
    fixed "created $label ($dir)"
  else
    fail "$label missing ($dir)"
  fi
}

ensure_dir "$DATA_DIR" "DATA_DIR"
ensure_dir "$VAULT_PATH" "vault path"
ensure_dir "$OPENCODE_CONFIG_DIR" "PocketBrain vault home"
ensure_dir "$POCKETBRAIN_SKILLS_DIR" "PocketBrain runtime skills dir"

ENABLE_WHATSAPP="${ENABLE_WHATSAPP:-false}"
if [[ "${ENABLE_WHATSAPP,,}" == "true" || "${ENABLE_WHATSAPP}" == "1" ]]; then
  ensure_dir "$WHATSAPP_AUTH_DIR" "WhatsApp auth dir"
else
  warn "WhatsApp disabled (ENABLE_WHATSAPP=false)"
fi

STATE_DB="${DATA_DIR}/state.db"
if [[ -f "$STATE_DB" ]]; then
  pass "SQLite state DB present ($STATE_DB)"
else
  warn "SQLite state DB missing ($STATE_DB)"
fi

if command -v systemctl >/dev/null 2>&1; then
  for svc in fail2ban unattended-upgrades tailscaled; do
    if systemctl is-active --quiet "$svc"; then
      pass "service active: $svc"
    else
      if safe_repair_enabled && (confirm "Start service $svc?" || [[ "$YES" == "true" || "$NON_INTERACTIVE" == "true" ]]); then
        sudo systemctl enable --now "$svc" >/dev/null
        fixed "enabled and started $svc"
      else
        warn "service not active: $svc"
      fi
    fi
  done

  if systemctl cat pocketbrain >/dev/null 2>&1; then
    if systemctl is-active --quiet pocketbrain; then
      pass "pocketbrain service active"
    else
      warn "pocketbrain service installed but not active"
    fi
  else
    warn "pocketbrain service not installed"
  fi
else
  warn "systemctl not found; skipping service checks"
fi

if UFW_BIN="$(resolve_bin ufw)"; then
  UFW_STATUS="$(sudo "$UFW_BIN" status verbose || true)"
  if printf '%s' "$UFW_STATUS" | grep -q '^Status: active'; then
    pass "UFW active"
  else
    if safe_repair_enabled && (confirm "Enable UFW with baseline rules?" || [[ "$YES" == "true" || "$NON_INTERACTIVE" == "true" ]]); then
      sudo "$UFW_BIN" default deny incoming >/dev/null
      sudo "$UFW_BIN" default allow outgoing >/dev/null
      sudo "$UFW_BIN" allow OpenSSH >/dev/null
      sudo "$UFW_BIN" allow 41641/udp >/dev/null
      sudo "$UFW_BIN" --force enable >/dev/null
      fixed "enabled UFW with baseline rules"
      UFW_STATUS="$(sudo "$UFW_BIN" status verbose || true)"
    else
      fail "UFW inactive"
    fi
  fi

  if printf '%s' "$UFW_STATUS" | grep -q 'Default: deny (incoming)'; then
    pass "UFW default incoming deny"
  else
    warn "UFW default incoming policy is not deny"
  fi

  if printf '%s' "$UFW_STATUS" | grep -q 'OpenSSH'; then
    pass "UFW allows OpenSSH"
  else
    fail "UFW missing OpenSSH allow rule"
  fi
else
  warn "ufw not installed"
fi

if SSHD_BIN="$(resolve_bin sshd)"; then
  SSHD_EFFECTIVE="$(sudo "$SSHD_BIN" -T || true)"
  if printf '%s' "$SSHD_EFFECTIVE" | grep -q '^passwordauthentication no$'; then
    pass "SSH password authentication disabled"
  else
    fail "SSH password authentication is not disabled"
  fi

  if printf '%s' "$SSHD_EFFECTIVE" | grep -q '^permitrootlogin no$'; then
    pass "SSH root login disabled"
  else
    fail "SSH root login is not disabled"
  fi

  if printf '%s' "$SSHD_EFFECTIVE" | grep -q '^kbdinteractiveauthentication no$'; then
    pass "SSH keyboard-interactive auth disabled"
  else
    warn "SSH keyboard-interactive auth is not disabled"
  fi

  if aggressive_repair_enabled && (confirm "Apply canonical SSH hardening drop-in?" || [[ "$YES" == "true" || "$NON_INTERACTIVE" == "true" ]]); then
    sudo sh -c 'cat > /etc/ssh/sshd_config.d/99-pocketbrain-hardening.conf <<"EOF"
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
PubkeyAuthentication yes
MaxAuthTries 3
X11Forwarding no
AllowAgentForwarding no
AllowTcpForwarding no
EOF'
    sudo "$SSHD_BIN" -t
    sudo systemctl reload ssh
    fixed "applied canonical SSH hardening drop-in"
  fi
else
  warn "sshd not found; skipping SSH checks"
fi

if command -v tailscale >/dev/null 2>&1; then
  TS_STATUS="$(sudo tailscale status 2>&1 || true)"
  if printf '%s' "$TS_STATUS" | grep -q 'Logged out\|Needs login\|Log in at:'; then
    warn "Tailscale installed but not logged in"
  else
    pass "Tailscale logged in"
  fi
else
  warn "tailscale CLI not found"
fi

SYNCTHING_ENABLED_VALUE="${SYNCTHING_ENABLED:-false}"
if [[ "${SYNCTHING_ENABLED_VALUE,,}" == "true" || "$SYNCTHING_ENABLED_VALUE" == "1" ]]; then
  SYNCTHING_BASE_URL_VALUE="${SYNCTHING_BASE_URL:-http://127.0.0.1:8384}"
  SYNCTHING_API_KEY_VALUE="${SYNCTHING_API_KEY:-}"
  SYNCTHING_TIMEOUT_MS_VALUE="${SYNCTHING_TIMEOUT_MS:-5000}"
  SYNCTHING_VAULT_FOLDER_ID_VALUE="${SYNCTHING_VAULT_FOLDER_ID:-vault}"
  SYNCTHING_MUTATION_TOOLS_ENABLED_VALUE="${SYNCTHING_MUTATION_TOOLS_ENABLED:-false}"
  SYNCTHING_ALLOWED_FOLDER_IDS_VALUE="${SYNCTHING_ALLOWED_FOLDER_IDS:-}"

  CURL_TIMEOUT_SEC=6
  if [[ "$SYNCTHING_TIMEOUT_MS_VALUE" =~ ^[0-9]+$ ]] && [[ "$SYNCTHING_TIMEOUT_MS_VALUE" -gt 0 ]]; then
    CURL_TIMEOUT_SEC=$((SYNCTHING_TIMEOUT_MS_VALUE / 1000 + 1))
  else
    warn "SYNCTHING_TIMEOUT_MS is invalid (${SYNCTHING_TIMEOUT_MS_VALUE}); using 6s for probes"
  fi

  if [[ -z "$SYNCTHING_API_KEY_VALUE" ]]; then
    fail "Syncthing enabled but SYNCTHING_API_KEY is missing"
  else
    pass "Syncthing API key configured"
  fi

  if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet syncthing; then
      pass "Syncthing system service active"
    elif systemctl --user is-active --quiet syncthing; then
      pass "Syncthing user service active"
    else
      warn "Syncthing service is not active (system or user scope)"
    fi
  fi

  if [[ "$SYNCTHING_BASE_URL_VALUE" =~ ^https?://(127\.0\.0\.1|localhost)(:[0-9]+)?$ ]]; then
    pass "Syncthing base URL uses local interface"
  else
    warn "Syncthing base URL is non-local (${SYNCTHING_BASE_URL_VALUE})"
  fi

  if command -v curl >/dev/null 2>&1 && [[ -n "$SYNCTHING_API_KEY_VALUE" ]]; then
    if curl -fsS -m "$CURL_TIMEOUT_SEC" \
      -H "X-API-Key: ${SYNCTHING_API_KEY_VALUE}" \
      "${SYNCTHING_BASE_URL_VALUE}/rest/system/ping" >/dev/null 2>&1; then
      pass "Syncthing API ping succeeded"
    else
      fail "Syncthing API ping failed (check base URL/API key/service)"
    fi

    if curl -fsS -m "$CURL_TIMEOUT_SEC" \
      -H "X-API-Key: ${SYNCTHING_API_KEY_VALUE}" \
      "${SYNCTHING_BASE_URL_VALUE}/rest/db/status?folder=${SYNCTHING_VAULT_FOLDER_ID_VALUE}" >/dev/null 2>&1; then
      if [[ -n "${SYNCTHING_VAULT_FOLDER_ID:-}" ]]; then
        pass "Syncthing vault folder ID is valid (${SYNCTHING_VAULT_FOLDER_ID_VALUE})"
      else
        pass "Syncthing vault folder ID defaulted to '${SYNCTHING_VAULT_FOLDER_ID_VALUE}' and is valid"
      fi
    else
      warn "Syncthing vault folder ID check failed (${SYNCTHING_VAULT_FOLDER_ID_VALUE})"
    fi
  else
    warn "curl missing or Syncthing API key missing; skipping Syncthing API probes"
  fi

  if [[ "${SYNCTHING_MUTATION_TOOLS_ENABLED_VALUE,,}" == "true" || "$SYNCTHING_MUTATION_TOOLS_ENABLED_VALUE" == "1" ]]; then
    if [[ -n "$SYNCTHING_ALLOWED_FOLDER_IDS_VALUE" ]]; then
      pass "Syncthing mutation policy configured with allowed folder IDs"
    else
      fail "SYNCTHING_MUTATION_TOOLS_ENABLED=true but SYNCTHING_ALLOWED_FOLDER_IDS is empty"
    fi
  else
    pass "Syncthing mutation tools disabled by policy"
  fi
else
  pass "Syncthing integration disabled"
fi

if [[ "$DEEP" == "true" ]]; then
  if command -v bun >/dev/null 2>&1; then
    if bun run typecheck >/dev/null 2>&1; then
      pass "deep check: typecheck passed"
    else
      warn "deep check: typecheck failed"
    fi
  else
    warn "deep check skipped: bun not installed"
  fi
fi

printf '\nDoctor summary: %s pass, %s warn, %s fail, %s fixed\n' "$PASS_COUNT" "$WARN_COUNT" "$FAIL_COUNT" "$FIXED_COUNT"

if [[ "$FAIL_COUNT" -gt 0 ]]; then
  exit 2
fi
exit 0
