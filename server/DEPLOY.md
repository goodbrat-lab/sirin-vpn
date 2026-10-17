# Развёртывание Sirin VPN

## Требования
- VPS: Ubuntu 24.04, 2+ ГБ RAM, 20 ГБ диска
- Домен: crp.example.com (A-запись → IP VPS)
- Доступ: root (или sudo)

## Установка Docker
```bash
apt update
apt install -y ca-certificates curl gnupg
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu noble stable" > /etc/apt/sources.list.d/docker.list
apt update
apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

## Генерация ключей
```bash
# Статический ключ сервера (sirind)
sirind --genkey
# Private Key (hex) → /etc/sirin/static_priv.hex
# Public Key (hex) → в клиент Config.kt

# Xray Reality
xray x25519
# Private Key → xray/config.json (privateKey)
# Public Key → API (vless_pbk) и клиент

xray uuid
# UUID → xray/config.json (clients[0].id)

openssl rand -hex 8
# Short ID → xray/config.json (shortIds)
```

## Создание конфигов
```bash
mkdir -p /etc/sirin /usr/local/etc/xray /var/log/sirin /var/log/xray

# sirind.toml
cat > /etc/sirin/sirind.toml << 'EOF'
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

# sirinapi.toml
cat > /etc/sirin/sirinapi.toml << 'EOF'
listen_addr = ":8080"
storage_path = "/etc/sirin/clients.json"
admin_token = "REPLACE_WITH_LONG_RANDOM_TOKEN"
server_host = "YOUR_SERVER_IP"
server_port = 56005
server_id = "sirin-main"
generation = 1
vless_enabled = true
vless_host = "YOUR_SERVER_IP"
vless_port = 443
EOF

# sirinbot.toml
cat > /etc/sirin/sirinbot.toml << 'EOF'
bot_token = "REPLACE_WITH_BOT_TOKEN"
admin_id = 0
api_url = "http://127.0.0.1:8080"
api_admin_token = "REPLACE_WITH_API_ADMIN_TOKEN"
storage_path = "/etc/sirin/bot_users.json"
EOF

# xray/config.json
cat > /usr/local/etc/xray/config.json << 'EOF'
{
  "log": {"loglevel": "warning"},
  "inbounds": [{
    "listen": "127.0.0.1",
    "port": 4430,
    "protocol": "vless",
    "settings": {
      "clients": [{"id": "REPLACE_WITH_VLESS_UUID", "flow": "xtls-rprx-vision"}],
      "decryption": "none"
    },
    "streamSettings": {
      "network": "raw",
      "security": "reality",
      "realitySettings": {
        "dest": "sun8-21.userapi.com:443",
        "serverNames": ["sun8-21.userapi.com"],
        "privateKey": "REPLACE_WITH_XRAY_PRIVATE_KEY",
        "shortIds": ["REPLACE_WITH_SHORT_ID", ""]
      }
    }
  }],
  "outbounds": [
    {"protocol": "freedom", "tag": "sirind-forward"},
    {"protocol": "blackhole", "tag": "block"}
  ],
  "routing": {"rules": [
    {"type": "field", "ip": ["127.0.0.1"], "port": "56006", "outboundTag": "sirind-forward"},
    {"type": "field", "network": "tcp,udp", "outboundTag": "block"}
  ]}
}
EOF

# Права
chmod 600 /etc/sirin/*.toml /etc/sirin/static_priv.hex
chmod 644 /usr/local/etc/xray/config.json
```

## SSL-сертификат
```bash
apt install -y nginx certbot python3-certbot-nginx
# Настроить /etc/nginx/sites-available/crp.example.com (HTTP-заглушка)
# Получить сертификат
certbot certonly --nginx -d crp.example.com
```

## nftables
```bash
./configs/setup-nftables.sh
```

## Запуск
```bash
docker compose up -d
docker compose ps
curl -sf https://crp.example.com/health
```

## Миграция с systemd
```bash
# ОСТОРОЖНО: останавливает systemd-сервисы
sudo ./migrate.sh
```

## Откат
```bash
docker compose down
sudo systemctl enable --now nginx xray sirind sirinapi sirinbot
```

## Troubleshooting
- **TUN не создаётся** → проверить `/dev/net/tun`, `cap_add: NET_ADMIN`
- **nginx в контейнере не видит сертификаты** → проверить volume `/etc/letsencrypt`
- **sirind не может писать в `/var/log`** → проверить `user: root` и volume
- **xray падает: permission denied /var/log/xray/access.log** → 
  xray в контейнере работает под UID 65532. Исправить:
  `chown -R 65532:65532 /var/log/xray && chmod 755 /var/log/xray`
