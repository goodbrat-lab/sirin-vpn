# Sirin Server (Docker)

## Deploy (чистый VPS)
```bash
sudo ./deploy.sh
```

## Migrate (с systemd)
```bash
sudo ./migrate.sh
```

## Logs
```bash
docker compose logs -f [nginx|xray|sirind|sirinapi|sirinbot]
```

## Откат (если миграция не удалась)
```bash
docker compose down
sudo systemctl enable --now nginx xray sirind sirinapi sirinbot
```
