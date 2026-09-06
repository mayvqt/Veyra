# Veyra

[![CI](https://github.com/mayvqt/Veyra/actions/workflows/ci.yml/badge.svg)](https://github.com/mayvqt/Veyra/actions/workflows/ci.yml)
[![Container](https://img.shields.io/badge/ghcr.io-mayvqt%2Fveyra-0f766e)](https://github.com/mayvqt/Veyra/pkgs/container/veyra)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

## Overview

Veyra is a self-hosted Jellyfin or Emby portal with media login, dashboard widgets, Seerr requests, Arr data, and admin health views.

## Quick start

```bash
cp .env.example .env
openssl rand -hex 32 # SESSION_SECRET
openssl rand -hex 16 # ENCRYPTION_KEY
docker compose up -d
```

Replace the two placeholder secrets in `.env`, start Veyra, then open `http://localhost:3767/setup`. Complete setup on a
trusted local network before exposing Veyra publicly. Leave the other wizard-managed values blank unless you intend to
manage them through the environment.

## Documentation

- [Setup](docs/setup.md)
- [Configuration](docs/configuration.md)
- [Integrations](docs/integrations.md)
- [Deployment](docs/deployment.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Security](docs/security.md)
- [Development](docs/development.md)
- [Architecture](docs/architecture.md)
- [License](LICENSE)
- [Issues](https://github.com/mayvqt/Veyra/issues)

## Related projects

These are separate deployments in the same media-server and Seerr ecosystem:

- [Augur](https://github.com/mayvqt/Augur) — a Discord bot for requesting movies and TV shows through Seerr.
- [Aperture](https://github.com/mayvqt/Aperture) — controlled invite links and account provisioning for Jellyfin or Emby.
