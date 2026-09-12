# Capabilities

Veyra gives Jellyfin or Emby users one place to:

- sign in with their media-server account;
- see recently added movies and TV shows, with episodes consolidated into one card per show, and open them on the media server;
- search Seerr, choose seasons, submit requests, and review request status;
- view Sonarr and Radarr queues and upcoming releases; and
- follow links to configured services.

Media-server administrators also get user, playback, integration-health, storage,
and audit views, plus browser-based configuration. Optional Prowlarr data appears
in the integration health view. Veyra keeps short-lived cached responses so a
temporary upstream problem can degrade individual widgets without taking down the
whole dashboard.

See [Integrations](integrations.md) for ownership and linking rules, and
[Configuration](configuration.md) for the settings behind each feature.
