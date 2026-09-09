# Architecture

- Handlers own HTTP orchestration.
- Integrations own upstream protocols.
- Stores own SQL and durable state.
- Templates and static assets own presentation.

Jellyfin and Emby use explicit provider implementations behind one shared bounded media-server client.

## Frontend and UI

Go templates in `internal/http/templates/` define page structure and server-rendered
content. Static files in `web/static/` own presentation and progressive browser
interactions; `assets.go` embeds the resulting bundle in the binary. The frontend
does not make trusted authorization, integration, or persistence decisions.

Shared CSS is split by responsibility: design tokens and element defaults live in
`css/base/foundation.css`, the application shell and shared layout live in
`css/base/shell.css`, reusable controls live in `css/components/`, and route-specific
composition lives in `css/pages/`. Extend those existing layers instead of adding a
second styling system or duplicating page markup in JavaScript.

Dashboard JavaScript is intentionally dependency-free and split by feature under
`web/static/js/dashboard/`. Preserve server-rendered fallbacks and progressively
enhance existing controls. Keep public routes, form contracts, CSRF handling, and
permission-dependent rendering unchanged unless the feature explicitly requires a
contract change.
