# Changelog

## [1.0.0] - 2026-10-17

First public release of Sirin VPN.

### Features
- Android client with Sirin Feather visual element
- Own server: UDP Noise NK + TCP VLESS Reality (XTLS Vision)
- Automatic fallback if UDP is blocked (392ms)
- Split tunneling (RU direct, rest via VPN)
- IPv6 leak protection (fd00::1/128 on tun)
- API with HTTPS (Let's Encrypt)
- Telegram bot for key management (@sirin_crp_bot)
- SNI masking via VK CDN (sun8-21.userapi.com)
- Docker deployment (docker-compose.yml)

### Fixed
- IPv6 leak (critical: was leaking through Wi-Fi)
- MTU mismatch (1280 vs 1100, caused crashes)
- Split tunneling trap (ONLY mode with 1 app blocked everything)

### Tech Stack
- Go 1.27 (engine, server, API, bot)
- Kotlin 2.x + Jetpack Compose (Android)
- Docker + docker-compose (deployment)
- Xray-core (VLESS Reality)

### Based on
- Meridian VPN (JinComputers) - MIT License
- See NOTICE for full credits
