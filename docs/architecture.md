# Architecture

- Handlers own HTTP orchestration.
- Integrations own upstream protocols.
- Stores own SQL and durable state.
- Templates and static assets own presentation.

Jellyfin and Emby use explicit provider implementations behind one shared bounded media-server client.
