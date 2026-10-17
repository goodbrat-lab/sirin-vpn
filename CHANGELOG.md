# Changelog

## [1.1.0] - 2026-10-17

Automatic client key synchronization between clients.json and sirind.

### Added
- Auto-sync: `sirind` reads allowed UUIDs directly from `/etc/sirin/clients.json` with fallback to `sirind.toml`
- Background sync loop: checks `clients.json` mtime every 30 seconds
- Thread-safe key handling via `sync.RWMutex`
- Key caching: avoids re-deriving X25519 wrap keys for unchanged UUIDs
- Unit test suite for client synchronization (`sync_test.go`)
- Support for generating and using keys from Telegram bot without restarting `sirind`

### Changed
- `sirind` container mounts `/etc/sirin` directory as read-only (`ro`)
- Configuration accepts optional `clients_path` parameter

## [1.0.1] - 2026-10-17

Working release with production configuration via GitHub Secrets.

### Changed
- CI: use `SIRIN_CONFIG_KT` secret for release builds to produce production-ready APK
- Fixed `version.properties` formatting (versionCode: 2, versionName: 1.0.1)

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
