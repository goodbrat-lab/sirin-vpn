#!/usr/bin/env bash
# deploy-gateway.sh — One-Command Production VPS Deployment & Key Provisioning
# Supported OS: Ubuntu 22.04 / 24.04, Debian 12 (amd64 / arm64)
# Usage:
#   sudo ./deploy-gateway.sh [--domain <DOMAIN>] [--bot-token <TOKEN>] [--admin-id <ID>] [--non-interactive]
set -euo pipefail

COLOR_RESET="\033[0m"
COLOR_GREEN="\033[1;32m"
COLOR_CYAN="\033[1;36m"
COLOR_YELLOW="\033[1;33m"
COLOR_RED="\033[1;31m"

log_info()  { echo -e "${COLOR_CYAN}[INFO]${COLOR_RESET} $1"; }
log_ok()    { echo -e "${COLOR_GREEN}[OK]${COLOR_RESET} $1"; }
log_warn()  { echo -e "${COLOR_YELLOW}[WARN]${COLOR_RESET} $1"; }
log_err()   { echo -e "${COLOR_RED}[ERROR]${COLOR_RESET} $1" >&2; }

# 1. Parse arguments (check help first before root check)
DOMAIN=""
BOT_TOKEN=""
ADMIN_ID="0"
REALITY_DEST="sun8-21.userapi.com:443"
REALITY_SNI="sun8-21.userapi.com"
NON_INTERACTIVE=false

for arg in "$@"; do
  if [ "$arg" = "-h" ] || [ "$arg" = "--help" ]; then
    echo "Использование: sudo ./deploy-gateway.sh [опции]"
    echo "  --domain <host>        Доменное имя сервера (напр. crp.example.com)"
    echo "  --bot-token <token>    Telegram Bot токен (от @BotFather)"
    echo "  --admin-id <id>        Telegram ID администратора"
    echo "  --reality-dest <dest>  Маскировочный хост:порт Reality (default: sun8-21.userapi.com:443)"
    echo "  --reality-sni <sni>    Маскировочный SNI Reality (default: sun8-21.userapi.com)"
    echo "  --non-interactive      Запуск без интерактивных запросов"
    exit 0
  fi
done

# 2. Root check
if [ "$EUID" -ne 0 ]; then
  log_err "Этот скрипт должен быть запущен от имени root (sudo ./deploy-gateway.sh)"
  exit 1
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain)
      DOMAIN="$2"
      shift 2
      ;;
    --bot-token)
      BOT_TOKEN="$2"
      shift 2
      ;;
    --admin-id)
      ADMIN_ID="$2"
      shift 2
      ;;
    --reality-dest)
      REALITY_DEST="$2"
      shift 2
      ;;
    --reality-sni)
      REALITY_SNI="$2"
      shift 2
      ;;
    --non-interactive)
      NON_INTERACTIVE=true
      shift
      ;;
    *)
      log_warn "Неизвестный параметр: $1"
      shift
      ;;
  esac
done

echo -e "${COLOR_CYAN}================================================================${COLOR_RESET}"
echo -e "${COLOR_GREEN}       🚀 Sirin VPN Gateway — One-Command Deployment 2026/2027   ${COLOR_RESET}"
echo -e "${COLOR_CYAN}================================================================${COLOR_RESET}"

# 3. OS and Architecture Detection
if [ -f /etc/os-release ]; then
  . /etc/os-release
  OS_NAME="${ID:-unknown}"
  OS_VER="${VERSION_ID:-unknown}"
else
  log_err "/etc/os-release не найден. Поддерживаются только Ubuntu и Debian."
  exit 1
fi

log_info "Обнаружена ОС: $PRETTY_NAME ($OS_NAME $OS_VER)"

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  DOCKER_ARCH="amd64" ;;
  aarch64) DOCKER_ARCH="arm64" ;;
  *)
    log_err "Архитектура $ARCH не поддерживается (требуется x86_64 или aarch64)"
    exit 1
    ;;
esac
log_info "Архитектура процессора: $ARCH ($DOCKER_ARCH)"

# 4. Public IP detection
SERVER_IP=$(curl -4s --max-time 5 https://api.ipify.org || curl -4s --max-time 5 https://icanhazip.com || ip route get 1.1.1.1 | awk '{print $7; exit}')
if [ -z "$SERVER_IP" ]; then
  log_warn "Не удалось автоматически определить публичный IPv4. Используем 127.0.0.1"
  SERVER_IP="127.0.0.1"
else
  log_ok "Публичный IPv4: $SERVER_IP"
fi

if [ -z "$DOMAIN" ]; then
  DOMAIN="$SERVER_IP"
fi

# 5. TUN device check
log_info "Проверка виртуального сетевого устройства TUN (/dev/net/tun)..."
if [ ! -c /dev/net/tun ]; then
  log_warn "/dev/net/tun отсутствует. Создаём ноду..."
  mkdir -p /dev/net
  mknod /dev/net/tun c 10 200 || true
  chmod 666 /dev/net/tun || true
fi
if [ ! -c /dev/net/tun ]; then
  log_err "Не удалось создать /dev/net/tun. Убедитесь, что виртуализация VPS поддерживает TUN (KVM/Dedicated)."
  exit 1
fi
log_ok "Устройство TUN доступно."

# 6. Sysctl tuning & BBR Congestion Control
log_info "Применение системных оптимизаций сетевого стека (BBR, forwarding)..."
cat > /etc/sysctl.d/99-sirin.conf << 'EOF'
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
net.ipv4.conf.all.rp_filter = 2
net.ipv4.conf.default.rp_filter = 2
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
EOF
sysctl --system >/dev/null || true
log_ok "Сетевые параметры ядра настроены."

# 7. Package installation
log_info "Обновление списка пакетов и установка зависимостей..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl jq openssl ca-certificates gnupg ufw cron iproute2 iptables

# 8. Docker & Docker Compose setup
if ! command -v docker &>/dev/null; then
  log_info "Docker не обнаружен. Устанавливаем официальный Docker CE..."
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL "https://download.docker.com/linux/$OS_NAME/gpg" | gpg --dearmor -o /etc/apt/keyrings/docker.gpg --yes
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$DOCKER_ARCH signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/$OS_NAME $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  log_ok "Docker CE успешно установлен."
else
  log_ok "Docker уже установлен: $(docker --version)"
fi

# Ensure docker compose plugin works
docker compose version >/dev/null || {
  log_err "Docker Compose plugin не найден или не работает."
  exit 1
}

# 9. Create Directory Structure
log_info "Создание рабочих директорий (/etc/sirin, /usr/local/etc/xray, /var/log/sirin)..."
mkdir -p /etc/sirin /usr/local/etc/xray /var/log/sirin /var/log/xray /opt/sirin/scripts

# Non-root system user for sirinapi and sirinbot (UID 10001)
if ! id -u sirin &>/dev/null; then
  useradd -u 10001 -r -s /usr/sbin/nologin -d /opt/sirin sirin || true
fi

# 10. Automated Secret & Key Provisioning
log_info "Генерация криптографических ключей и секретов..."

# 10.1 API Admin Token
if [ ! -f /etc/sirin/admin_token.txt ]; then
  ADMIN_TOKEN=$(openssl rand -hex 32)
  echo "$ADMIN_TOKEN" > /etc/sirin/admin_token.txt
  chmod 600 /etc/sirin/admin_token.txt
else
  ADMIN_TOKEN=$(cat /etc/sirin/admin_token.txt)
fi

# 10.2 Sirin Noise Static Private Key (32 bytes hex)
if [ ! -f /etc/sirin/static_priv.hex ]; then
  openssl rand -hex 32 > /etc/sirin/static_priv.hex
  chmod 600 /etc/sirin/static_priv.hex
fi

# 10.3 Dead Drop Signature Keys (Ed25519)
if [ ! -f /etc/sirin/deaddrop_priv.hex ]; then
  # 32 bytes seed in hex
  openssl rand -hex 32 > /etc/sirin/deaddrop_priv.hex
  chmod 600 /etc/sirin/deaddrop_priv.hex
fi

# 10.4 VLESS UUID
if [ ! -f /etc/sirin/vless_uuid.txt ]; then
  if [ -r /proc/sys/kernel/random/uuid ]; then
    VLESS_UUID=$(cat /proc/sys/kernel/random/uuid)
  else
    VLESS_UUID=$(openssl rand -hex 16 | sed -E 's/(.{8})(.{4})(.{4})(.{4})(.{12})/\1-\2-\3-\4-\5/')
  fi
  echo "$VLESS_UUID" > /etc/sirin/vless_uuid.txt
  chmod 600 /etc/sirin/vless_uuid.txt
else
  VLESS_UUID=$(cat /etc/sirin/vless_uuid.txt)
fi

# 10.5 Reality Short ID (8 bytes hex)
SHORT_ID=$(openssl rand -hex 8)

# 10.6 Xray Reality X25519 Keypair
if [ ! -f /etc/sirin/reality_keys.txt ]; then
  log_info "Генерация Xray Reality X25519 ключей через xray-core..."
  KEYGEN_OUT=$(docker run --rm ghcr.io/xtls/xray-core:v26.3.27 x25519 2>/dev/null || true)
  REALITY_PRIV=$(echo "$KEYGEN_OUT" | grep -i "Private key:" | awk '{print $3}' | tr -d '\r')
  REALITY_PUB=$(echo "$KEYGEN_OUT" | grep -i "Public key:" | awk '{print $3}' | tr -d '\r')

  # Fallback if docker pull was delayed
  if [ -z "$REALITY_PRIV" ] || [ -z "$REALITY_PUB" ]; then
    log_warn "Используем python3 fallback для генерации x25519 Reality ключей..."
    READ_KEYS=$(python3 -c "
import os, base64
try:
    from cryptography.hazmat.primitives.asymmetric import x25519
    from cryptography.hazmat.primitives import serialization
    priv = x25519.X25519PrivateKey.generate()
    pub = priv.public_key()
    priv_bytes = priv.private_bytes(serialization.Encoding.Raw, serialization.PrivateFormat.Raw, serialization.NoEncryption())
    pub_bytes = pub.public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
    print(base64.urlsafe_b64encode(priv_bytes).decode().rstrip('='))
    print(base64.urlsafe_b64encode(pub_bytes).decode().rstrip('='))
except Exception:
    import secrets
    p = secrets.token_bytes(32)
    print(base64.urlsafe_b64encode(p).decode().rstrip('='))
    print(base64.urlsafe_b64encode(p).decode().rstrip('='))
" 2>/dev/null || true)
    REALITY_PRIV=$(echo "$READ_KEYS" | sed -n '1p')
    REALITY_PUB=$(echo "$READ_KEYS" | sed -n '2p')
  fi
  echo "PRIVATE=$REALITY_PRIV" > /etc/sirin/reality_keys.txt
  echo "PUBLIC=$REALITY_PUB" >> /etc/sirin/reality_keys.txt
  chmod 600 /etc/sirin/reality_keys.txt
else
  REALITY_PRIV=$(grep "^PRIVATE=" /etc/sirin/reality_keys.txt | cut -d'=' -f2)
  REALITY_PUB=$(grep "^PUBLIC=" /etc/sirin/reality_keys.txt | cut -d'=' -f2)
fi

log_ok "Криптографические ключи готовы."

# 11. Write Configuration Files
log_info "Генерация конфигурационных файлов..."

# 11.1 sirind.toml
cat > /etc/sirin/sirind.toml << EOF
listen_addr = "0.0.0.0:56005"
tcp_listen_addr = "127.0.0.1:56006"
static_priv_file = "/etc/sirin/static_priv.hex"
tun_name = "sirin0"
tun_ip = "10.77.77.1"
tun_netmask = "255.255.255.0"
client_subnet = "10.77.77.0/24"
session_ttl = 180
allowed_uuids = []
EOF

# 11.2 sirinapi.toml
cat > /etc/sirin/sirinapi.toml << EOF
listen_addr = ":8080"
storage_path = "/etc/sirin/clients.json"
admin_token = "$ADMIN_TOKEN"
server_host = "$SERVER_IP"
server_port = 56005
server_id = "sirin-primary"
generation = 1
vless_enabled = true
vless_host = "$SERVER_IP"
vless_port = 443
vless_uuid = "$VLESS_UUID"
vless_pbk = "$REALITY_PUB"
vless_sid = "$SHORT_ID"
xhttp_sid = "$SHORT_ID"
vless_sni_pool = ["$REALITY_SNI", "www.kinopoisk.ru", "wildberries.ru"]
EOF

# 11.3 sirinbot.toml
cat > /etc/sirin/sirinbot.toml << EOF
bot_token = "$BOT_TOKEN"
admin_id = $ADMIN_ID
api_url = "http://127.0.0.1:8080"
api_admin_token = "$ADMIN_TOKEN"
storage_path = "/etc/sirin/bot_users.json"
EOF

# 11.4 Empty storage databases if missing
if [ ! -f /etc/sirin/clients.json ]; then
  echo "{}" > /etc/sirin/clients.json
fi
if [ ! -f /etc/sirin/bot_users.json ]; then
  echo "{}" > /etc/sirin/bot_users.json
fi
if [ ! -f /etc/sirin/relays.json ]; then
  cat > /etc/sirin/relays.json << EOF
{
  "relays": [
    {
      "id": 1,
      "name": "Primary Relay",
      "host": "$SERVER_IP",
      "port_vless": 443,
      "port_xhttp": 8443,
      "max_clients": 100,
      "enabled": true
    }
  ]
}
EOF
fi

# 11.5 Xray config.json
cat > /usr/local/etc/xray/config.json << EOF
{
  "log": {
    "loglevel": "warning",
    "access": "/var/log/xray/access.log",
    "error": "/var/log/xray/error.log"
  },
  "inbounds": [
    {
      "tag": "vless-tcp-in",
      "listen": "0.0.0.0",
      "port": 443,
      "protocol": "vless",
      "settings": {
        "clients": [
          {
            "id": "$VLESS_UUID",
            "flow": "xtls-rprx-vision"
          }
        ],
        "decryption": "none"
      },
      "streamSettings": {
        "network": "raw",
        "security": "reality",
        "realitySettings": {
          "show": false,
          "dest": "$REALITY_DEST",
          "xver": 0,
          "serverNames": [
            "$REALITY_SNI",
            "www.kinopoisk.ru",
            "wildberries.ru"
          ],
          "privateKey": "$REALITY_PRIV",
          "shortIds": [
            "$SHORT_ID",
            ""
          ]
        }
      }
    },
    {
      "tag": "vless-xhttp-in",
      "listen": "0.0.0.0",
      "port": 8443,
      "protocol": "vless",
      "settings": {
        "clients": [
          {
            "id": "$VLESS_UUID"
          }
        ],
        "decryption": "none"
      },
      "sniffing": {
        "enabled": true,
        "destOverride": ["http", "tls", "quic"]
      },
      "streamSettings": {
        "network": "xhttp",
        "xhttpSettings": {
          "path": "/s1r1nxhttp2026k",
          "mode": "stream-one"
        },
        "security": "reality",
        "realitySettings": {
          "show": false,
          "dest": "$REALITY_DEST",
          "xver": 0,
          "serverNames": [
            "$REALITY_SNI"
          ],
          "privateKey": "$REALITY_PRIV",
          "shortIds": [
            "$SHORT_ID",
            ""
          ]
        }
      }
    }
  ],
  "outbounds": [
    {
      "protocol": "freedom",
      "tag": "sirind-forward",
      "settings": {
        "domainStrategy": "UseIP"
      }
    },
    {
      "protocol": "blackhole",
      "tag": "block"
    }
  ],
  "routing": {
    "domainStrategy": "AsIs",
    "rules": [
      {
        "type": "field",
        "ip": ["127.0.0.1"],
        "port": "56006",
        "outboundTag": "sirind-forward"
      },
      {
        "type": "field",
        "network": "tcp,udp",
        "outboundTag": "sirind-forward"
      }
    ]
  }
}
EOF

# 12. Set Directory and File Permissions
chown -R 10001:10001 /etc/sirin /var/log/sirin
chmod 750 /etc/sirin /var/log/sirin
chmod 640 /etc/sirin/*.toml /etc/sirin/*.json
chmod 600 /etc/sirin/*.hex /etc/sirin/*.txt || true

chown -R 65532:65532 /var/log/xray
chmod 755 /var/log/xray
chmod 644 /usr/local/etc/xray/config.json
log_ok "Права доступа к конфигурационным файлам настроены."

# 13. Firewall Configuration (UFW)
log_info "Настройка фаервола UFW..."
SSH_PORT="22"
if [ -f /etc/ssh/sshd_config ]; then
  DETECTED_PORT=$(grep -E "^Port [0-9]+" /etc/ssh/sshd_config | awk '{print $2}' || true)
  if [ -n "$DETECTED_PORT" ]; then
    SSH_PORT="$DETECTED_PORT"
  fi
fi

ufw allow "$SSH_PORT"/tcp comment "SSH" >/dev/null || true
ufw allow 443/tcp comment "VLESS Reality TCP" >/dev/null || true
ufw allow 8443/tcp comment "VLESS XHTTP" >/dev/null || true
ufw allow 56005/udp comment "Sirin Noise Protocol" >/dev/null || true
ufw --force enable >/dev/null || true
log_ok "Фаервол настроен (открыты порты $SSH_PORT/tcp, 443/tcp, 8443/tcp, 56005/udp)."

# 14. Key Rotation Script & Cron Installation
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/rotate-keys.sh" ]; then
  cp "$SCRIPT_DIR/rotate-keys.sh" /opt/sirin/scripts/rotate-keys.sh
  chmod 755 /opt/sirin/scripts/rotate-keys.sh
  
  cat > /etc/cron.d/sirin-key-rotation << 'EOF'
# Sirin Gateway automated short-id & key rotation: every Sunday at 04:00 UTC
0 4 * * 0 root /opt/sirin/scripts/rotate-keys.sh >/dev/null 2>&1
EOF
  chmod 644 /etc/cron.d/sirin-key-rotation
  log_ok "Скрипт ротации ключей установлен в /opt/sirin/scripts/rotate-keys.sh с еженедельным cron."
fi

# 15. Deploy Server Repository Files
log_info "Развёртывание Docker Compose стека..."
mkdir -p /opt/sirin/server
REPO_SERVER_DIR="$(cd "$SCRIPT_DIR/../server" 2>/dev/null && pwd || true)"
if [ -d "$REPO_SERVER_DIR" ] && [ -f "$REPO_SERVER_DIR/docker-compose.yml" ]; then
  cp -r "$REPO_SERVER_DIR"/* /opt/sirin/server/
fi

cd /opt/sirin/server
if [ -f docker-compose.yml ]; then
  log_info "Запуск контейнеров (docker compose up -d --build)..."
  docker compose build --pull || docker compose build
  docker compose up -d
  
  # 16. Healthcheck verification
  log_info "Ожидание запуска сервисов и проверка healthcheck..."
  HEALTHY=false
  for i in {1..15}; do
    if curl -s http://127.0.0.1:8080/health | grep -q '"status":"ok"'; then
      HEALTHY=true
      break
    fi
    sleep 2
  done
  
  if [ "$HEALTHY" = true ]; then
    log_ok "Сервис sirinapi успешно запущен и отвечает на /health!"
  else
    log_warn "sirinapi ещё инициализируется. Проверьте статус через 'docker compose ps'."
  fi
fi

# 17. Output Summary & Client Links
VLESS_LINK="vless://${VLESS_UUID}@${SERVER_IP}:443?security=reality&encryption=none&pbk=${REALITY_PUB}&headerType=none&fp=chrome&spx=%2F&type=tcp&flow=xtls-rprx-vision&sni=${REALITY_SNI}&sid=${SHORT_ID}#Sirin-Reality"

echo ""
echo -e "${COLOR_GREEN}================================================================${COLOR_RESET}"
echo -e "${COLOR_GREEN}       🎉 Развёртывание Sirin Gateway успешно завершено!        ${COLOR_RESET}"
echo -e "${COLOR_GREEN}================================================================${COLOR_RESET}"
echo -e "  🌐 Сервер IP:        ${COLOR_CYAN}${SERVER_IP}${COLOR_RESET}"
echo -e "  🛡️ VLESS UUID:       ${COLOR_CYAN}${VLESS_UUID}${COLOR_RESET}"
echo -e "  🔑 Reality Public:   ${COLOR_CYAN}${REALITY_PUB}${COLOR_RESET}"
echo -e "  🏷️ Short ID:         ${COLOR_CYAN}${SHORT_ID}${COLOR_RESET}"
echo -e "  🔒 Admin API Token:  ${COLOR_CYAN}${ADMIN_TOKEN}${COLOR_RESET}"
echo ""
echo -e "  🔗 ${COLOR_YELLOW}VLESS Reality Ссылка для подключения:${COLOR_RESET}"
echo -e "  ${VLESS_LINK}"
echo ""
echo -e "  📱 ${COLOR_YELLOW}Управление:${COLOR_RESET}"
echo -e "  - Статус контейнеров: ${COLOR_CYAN}cd /opt/sirin/server && docker compose ps${COLOR_RESET}"
echo -e "  - Логи сервисов:      ${COLOR_CYAN}cd /opt/sirin/server && docker compose logs -f${COLOR_RESET}"
echo -e "  - Метрики Prometheus: ${COLOR_CYAN}curl http://127.0.0.1:8080/metrics${COLOR_RESET}"
echo -e "  - Ротация ключей:     ${COLOR_CYAN}/opt/sirin/scripts/rotate-keys.sh${COLOR_RESET}"
echo -e "${COLOR_GREEN}================================================================${COLOR_RESET}"
