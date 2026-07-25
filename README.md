# Veyra

[![CI](https://github.com/mayvqt/Veyra/actions/workflows/ci.yml/badge.svg)](https://github.com/mayvqt/Veyra/actions/workflows/ci.yml)
[![Container](https://img.shields.io/badge/ghcr.io-mayvqt%2Fveyra-0f766e)](https://github.com/mayvqt/Veyra/pkgs/container/veyra)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

Veyra is a self-hosted Jellyfin or Emby portal with media login, dashboard widgets, Seerr requests, Arr data, and admin health views.

```bash
cp .env.example .env
openssl rand -hex 32 # SESSION_SECRET
openssl rand -hex 16 # ENCRYPTION_KEY
docker compose up -d
```

Replace the two placeholder secrets in `.env`, then open `http://localhost:3767/setup`.

Documentation:
- [Setup](docs/setup.md)
- [Configuration](docs/configuration.md)
- [Integrations](docs/integrations.md)
- [Deployment](docs/deployment.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Development](docs/development.md)
- [Architecture](docs/architecture.md)
- [Security](docs/security.md)
