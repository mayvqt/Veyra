# Architecture

Veyra is a single Go server backed by SQLite. It renders server-side HTML and
serves embedded CSS and JavaScript, so the deployed binary contains the complete
web application.

Handlers own HTTP orchestration, integrations own upstream protocols, the store
owns durable state, and templates and static assets own presentation. Jellyfin
and Emby have explicit provider implementations behind one bounded media-server
client.

Contributors can find the full request flow, sources of truth, and security and
concurrency boundaries in the [development architecture guide](development/architecture.md).
