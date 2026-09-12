# Architecture

`cmd/server` loads configuration and hands process lifecycle to
`internal/app`. The app opens SQLite, applies migrations, starts maintenance,
builds the router, and shuts the server down cleanly. The router creates
authentication and integration clients, then gives handlers narrow dependencies.
Templates and static files are embedded by `assets.go`.

## Sources of truth and public contracts

- Jellyfin or Emby owns identity, passwords, administrator status, access tokens,
  libraries, media, and playback sessions.
- Seerr owns account links, request permissions and quota, request history, and
  request state.
- Sonarr, Radarr, and Prowlarr own their queue, calendar, storage, and health data.
- SQLite owns Veyra users, sessions, setup settings, audit logs, and response cache.
- Environment variables override stored setup settings. [Configuration](../configuration.md)
  is the public configuration contract.
- Routes in `internal/http/router.go` are the HTTP contract. Setup is public only
  before configuration exists; dashboard/API routes require a session; `/admin`
  requires refreshed administrator status.

Upstream endpoints and authorization headers are external contracts. Check
changes against official provider documentation and cover them with deterministic
HTTP fixtures before any dedicated, opt-in live test.

Recently added media uses each provider's user-scoped, ungrouped Latest endpoint
to preserve its filtering preferences and episode creation dates. The TV query
requests episodes, excluding series folder creation dates, and the adapter groups
them by series ID before limiting the combined movie/TV list. It expands the TV
prefix from 100 records only when more history can affect
the visible list, doubling up to 4,096 records within a ten-second overall
deadline and the existing 2 MiB response limit. An incomplete traversal returns
an error so it cannot overwrite a complete cached list. Native grouping is not a
shared identity/timestamp contract: see the [Jellyfin implementation](https://github.com/jellyfin/jellyfin/blob/v12.0/Emby.Server.Implementations/Library/UserViewManager.cs)
and [Emby Latest API](https://dev.emby.media/reference/RestAPI/UserLibraryService/getUsersByUseridItemsLatest.html).

## Security and concurrency

Session cookies, media-server tokens, API keys, the encryption key, account links,
audit data, and the database are sensitive. Preserve CSRF enforcement, request
limits, login throttling, proxy validation, response headers, URL restrictions,
redaction, and the rule that a name or email match cannot link a Seerr account.

SQLite runs in WAL mode with one open connection and a busy timeout. Maintenance
expires sessions and cache entries and checkpoints the WAL. Dashboard handlers
fetch configured independent providers concurrently and label partial results. Cache loads coalesce concurrent failures as well as successes; bounded stale data never authorizes an unlinked identity. Auth
refreshes administrator status on an interval and fails closed for admin access.
Keep concurrent results deterministic, bound fan-out and timeouts, isolate
provider failure, and honor request cancellation.
