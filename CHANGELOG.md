# Changelog

## [1.2.0] - 2026-10-18

### Added
- SNI rotation: 4 domains (sun8-21.userapi.com, kinopoisk.ru, wildberries.ru, ikea.com). Auto-fallback if one blocked.
- Testbench scenario: exact SNI blocking via NFQUEUE

### Resilience
- Before: 1 SNI (sun8-21) + UDP = 2 points of failure
- After: 4 SNI + UDP = 5 points of failure
- Verified: block sun8-21 -> switch to kinopoisk in 4.5s

### Infrastructure
- Xray serverNames: 4 domains
- API /v1/params: SNIs[] array
- Android client: parses snis, passes to engine

## [1.1.1] - 2026-10-17

### Added
- Deep-link support: https://crp.guseynovia.ru/key?uuid=... opens app
- "Change key" button in Settings with immediate reconnect
- Telegram bot: HTTPS link + inline button "Open in Sirin"
- KeyIcon in brand style

### Fixed
- APK v1.1.0 was missing deep-link and change-key features (not committed)
- v1.1.1 includes all previous features + new ones

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
