# Codebase map

| Path | What lives here |
| --- | --- |
| `cmd/server` | CLI commands, process startup, signals, backup dispatch, and logging. |
| `internal/app` | Database initialization, HTTP lifecycle, maintenance, graceful shutdown, and backup coordination. |
| `internal/config` | Environment and setup settings, defaults, URL normalization, and validation. |
| `internal/auth` | Media-server login sessions and administrator refresh policy. |
| `internal/http/router.go` | Public, authenticated, and administrator route boundaries. |
| `internal/http/handlers` | Page/API orchestration, dashboard composition, setup, settings, and cache policy. |
| `internal/http/middleware` | Authentication, CSRF, proxy trust, limits, request logging, headers, and static caching. |
| `internal/integrations/mediaserver` | Jellyfin/Emby identity, media, playback, and admin adapters. |
| `internal/integrations/seerr` | Search, requests, quotas, history, and account linking. |
| `internal/integrations/arr` | Sonarr, Radarr, and Prowlarr queue, calendar, health, and storage clients. |
| `internal/store` | SQLite schema, queries, sessions, settings, audit logs, cache, and maintenance. |
| `internal/security`, `internal/logging` | Encryption and final diagnostic redaction. |
| `internal/http/templates`, `web/static`, `assets.go` | Embedded HTML, CSS, JavaScript, images, and asset fingerprints. |
| `scripts`, `Dockerfile`, `docker-entrypoint.sh`, `docker-compose.yml` | Local bundles, checks, container build/startup, and example deployment. |
| `templates/unraid`, `.github/workflows` | Unraid metadata, CI, image publishing, and announcements. |

Tests live beside the packages they cover. Update this map when ownership moves
or a new top-level area is added.
