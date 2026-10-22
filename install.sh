#!/usr/bin/env bash

set -euo pipefail

MANIFEST_URL="${RABBIT_INSTALL_MANIFEST_URL:-https://reisen7.github.io/rabbit-panel/latest.json}"
MANIFEST_FALLBACK_URL="${RABBIT_INSTALL_MANIFEST_FALLBACK_URL:-https://raw.githubusercontent.com/reisen7/rabbit-panel/gh-pages/latest.json}"
INSTALL_DIR="${RABBIT_INSTALL_DIR:-/opt/rabbit-panel}"
BIN_PATH="${RABBIT_BIN_PATH:-/usr/local/bin/rabbit-panel}"
SERVICE_NAME="${RABBIT_SERVICE_NAME:-rabbit-panel}"
ENV_FILE="${RABBIT_ENV_FILE:-/etc/${SERVICE_NAME}.env}"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

require_root() {
  if [ "${EUID:-$(id -u)}" -ne 0 ]; then
    echo "[ERROR] Please run as root."
    exit 1
  fi
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "linux-amd64" ;;
    aarch64|arm64) echo "linux-arm64" ;;
    armv7l|armv7) echo "linux-armv7" ;;
    *)
      echo "[ERROR] Unsupported architecture: $(uname -m)"
      exit 1
      ;;
  esac
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "[ERROR] Missing required command: $cmd"
    exit 1
  fi
}

try_download() {
  local url="$1"
  local output="$2"

  if command -v curl >/dev/null 2>&1; then
    curl --retry 3 --connect-timeout 10 -fsSL "$url" -o "$output"
    return $?
  fi

  if command -v wget >/dev/null 2>&1; then
    wget -t 3 -T 10 -qO "$output" "$url"
    return $?
  fi

  echo "[ERROR] curl or wget is required." >&2
  exit 1
}

fetch_manifest() {
  local manifest_path="$TMP_DIR/latest.json"

  if try_download "$MANIFEST_URL" "$manifest_path"; then
    :
  elif [ "$MANIFEST_FALLBACK_URL" != "$MANIFEST_URL" ] && try_download "$MANIFEST_FALLBACK_URL" "$manifest_path"; then
    echo "[WARN] Failed to fetch manifest from primary URL, switched to fallback URL." >&2
  else
    echo "[ERROR] Failed to fetch update manifest." >&2
    echo "[ERROR] Primary:  $MANIFEST_URL" >&2
    echo "[ERROR] Fallback: $MANIFEST_FALLBACK_URL" >&2
    exit 1
  fi

  if [ ! -s "$manifest_path" ]; then
    echo "[ERROR] Manifest file is empty: $manifest_path" >&2
    exit 1
  fi

  echo "$manifest_path"
}

read_json() {
  local file="$1"
  local key="$2"

  if [ ! -f "$file" ]; then
    echo "[ERROR] JSON file not found: $file" >&2
    exit 1
  fi

  python3 - "$file" "$key" <<'PY'
import json, sys
file_path, key = sys.argv[1], sys.argv[2]
try:
    with open(file_path, 'r', encoding='utf-8') as f:
        data = json.load(f)
    value = data
    for part in key.split('.'):
        value = value[part]
except FileNotFoundError:
    print(f"[ERROR] JSON file not found: {file_path}", file=sys.stderr)
    sys.exit(1)
except KeyError:
    print(f"[ERROR] Missing key in manifest: {key}", file=sys.stderr)
    sys.exit(1)
except json.JSONDecodeError as exc:
    print(f"[ERROR] Invalid JSON in {file_path}: {exc}", file=sys.stderr)
    sys.exit(1)

print(value)
PY
}

download_file() {
  local url="$1"
  local output="$2"

  if ! try_download "$url" "$output"; then
    echo "[ERROR] Failed to download file: $url" >&2
    exit 1
  fi
}

verify_sha256() {
  local file="$1"
  local expected="$2"
  if [ -z "$expected" ]; then
    return 0
  fi
  local actual=""
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$file" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$file" | awk '{print $1}')"
  elif command -v openssl >/dev/null 2>&1; then
    actual="$(openssl dgst -sha256 "$file" | awk '{print $NF}')"
  else
    echo "[ERROR] No SHA256 verification tool found."
    exit 1
  fi

  if [ "$actual" != "$expected" ]; then
    echo "[ERROR] SHA256 mismatch."
    echo "Expected: $expected"
    echo "Actual:   $actual"
    exit 1
  fi
}

ensure_env_file() {
  mkdir -p "$(dirname "$ENV_FILE")"
  if [ ! -f "$ENV_FILE" ]; then
    local jwt_secret
    local node_secret
    jwt_secret="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32)"
    node_secret="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32)"
    cat >"$ENV_FILE" <<EOF
MODE=master
HOST=0.0.0.0
PORT=3958
JWT_SECRET=${jwt_secret}
NODE_SECRET=${node_secret}
RABBIT_DEPLOY_MODE=binary
RABBIT_SERVICE_NAME=${SERVICE_NAME}
# --- 原生 HTTPS（默认开启，证书缺失时自动生成自签证书到 data/certs）---
TLS_ENABLED=true
TLS_SELF_SIGNED=true
TLS_HOSTS=
# 用已有证书时改为指向自己的证书并关掉自签：
# TLS_ENABLED=true
# TLS_SELF_SIGNED=false
# TLS_CERT_FILE=${INSTALL_DIR}/data/certs/server.crt
# TLS_KEY_FILE=${INSTALL_DIR}/data/certs/server.key
# 可选：额外开一个 HTTP 端口，自动 301 到 HTTPS
# HTTP_PORT=3959
# TLS_REDIRECT_PORT=3958
# 想用纯 HTTP 访问（例如前面有 Nginx 做 TLS 终止）：TLS_ENABLED=false
EOF
    chmod 600 "$ENV_FILE"
  fi
}

env_value() {
  # env_value KEY DEFAULT —— 从 env 文件读取配置值，缺失或空值时回退默认
  local key="$1"
  local fallback="$2"
  local value=""
  if [ -f "$ENV_FILE" ]; then
    value="$(sed -n "s/^${key}=//p" "$ENV_FILE" | tail -n 1)"
  fi
  if [ -z "$value" ]; then
    echo "$fallback"
  else
    echo "$value"
  fi
}

install_service() {
  cat >"$SERVICE_FILE" <<EOF
[Unit]
Description=Rabbit Panel
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
EnvironmentFile=${ENV_FILE}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BIN_PATH}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
}

main() {
  require_root
  require_cmd python3
  require_cmd systemctl

  local arch
  arch="$(detect_arch)"
  local manifest
  manifest="$(fetch_manifest)"

  local version
  local download_url
  local checksum
  version="$(read_json "$manifest" version)"
  download_url="$(read_json "$manifest" "assets.${arch}")"
  checksum="$(python3 - "$manifest" "$arch" <<'PY'
import json, sys
path, arch = sys.argv[1], sys.argv[2]
with open(path, 'r', encoding='utf-8') as f:
    data = json.load(f)
print(data.get('sha256', {}).get(arch, ''))
PY
)"

  echo "[INFO] Installing Rabbit Panel ${version} for ${arch}"

  mkdir -p "$INSTALL_DIR"
  local download_path="$TMP_DIR/rabbit-panel"
  download_file "$download_url" "$download_path"
  chmod +x "$download_path"
  verify_sha256 "$download_path" "$checksum"

  ensure_env_file

  if [ -f "$BIN_PATH" ]; then
    cp "$BIN_PATH" "${BIN_PATH}.bak"
  fi

  cp "$download_path" "$BIN_PATH"
  chmod +x "$BIN_PATH"

  install_service
  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"

  local access_host
  access_host="$(hostname -I 2>/dev/null | awk '{print $1}')"
  if [ -z "$access_host" ]; then
    access_host="127.0.0.1"
  fi

  # 从 env 文件读取实际监听端口与 TLS 状态，避免打印与实际不符的地址
  local access_port access_scheme
  access_port="$(env_value PORT 3958)"
  if [ "$(env_value TLS_ENABLED true)" = "false" ] || [ "$(env_value TLS_ENABLED true)" = "0" ]; then
    access_scheme="http"
  else
    access_scheme="https"
  fi

  echo "[OK] Rabbit Panel installed."
  echo "[INFO] Service name: ${SERVICE_NAME}"
  echo "[INFO] Binary path:  ${BIN_PATH}"
  echo "[INFO] Env file:     ${ENV_FILE}"
  echo "[INFO] Access URL:   ${access_scheme}://${access_host}:${access_port}"
  if [ "$access_scheme" = "https" ]; then
    echo "[INFO] 默认使用自签证书，浏览器首次访问会提示证书不受信任（属正常现象）。"
    echo "[INFO] 想改用纯 HTTP：编辑 ${ENV_FILE} 设置 TLS_ENABLED=false 后 systemctl restart ${SERVICE_NAME}"
  fi
  echo
  echo "[INFO] Service management commands:"
  echo "  systemctl start ${SERVICE_NAME}"
  echo "  systemctl stop ${SERVICE_NAME}"
  echo "  systemctl restart ${SERVICE_NAME}"
  echo "  systemctl status ${SERVICE_NAME}"
  echo "  journalctl -u ${SERVICE_NAME} -f"
}

main "$@"
