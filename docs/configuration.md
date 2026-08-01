# Configuration

Environment variables override wizard values. Internal URLs must be container-reachable; public URLs are browser links.

| Variable | Purpose |
| --- | --- |
| `APP_NAME`, `APP_BASE_URL`, `APP_BIND_ADDR` | Name, public URL, and listen address. |
| `DATABASE_PATH` | SQLite path; use `/config/veyra.db` in containers. |
| `LOG_LEVEL` | `debug`, `info`, `warn`, or `error`. |
| `COOKIE_SECURE` | Keep `true` for HTTPS; use `false` only for local HTTP. |
| `TRUSTED_PROXY_CIDRS` | Networks trusted to supply forwarded headers. |
| `PUID`, `PGID` | Container file ownership. |
| `MEDIA_SERVER_TYPE` | `jellyfin` or `emby`. |
| `MEDIA_SERVER_URL`, `MEDIA_SERVER_PUBLIC_URL`, `MEDIA_SERVER_API_KEY` | Required identity URL; optional public link and server-level media data. |
| `SEERR_URL`, `SEERR_PUBLIC_URL`, `SEERR_API_KEY` | Optional requests integration. |
| `SONARR_URL`, `SONARR_API_KEY` | Optional queue, calendar, and admin data. |
| `RADARR_URL`, `RADARR_API_KEY` | Optional queue, calendar, and admin data. |
| `PROWLARR_URL`, `PROWLARR_API_KEY` | Optional indexer and admin data. |

Every configured optional service URL requires its API key.

Service URLs must use HTTP or HTTPS and cannot contain URL userinfo, query parameters, or fragments. Keep credentials in the dedicated API key variables; this prevents them from being copied into request logs, diagnostics, browser pages, or `Referer` headers.
