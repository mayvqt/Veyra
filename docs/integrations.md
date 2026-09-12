# Integrations

- Jellyfin or Emby supplies identity and administrator policy. The API key enables recently added media, posters, playback, and admin data. Veyra uses Jellyfin's current `MediaBrowser` authorization scheme for Jellyfin 12 compatibility while retaining Emby's provider-specific token authorization.
- Seerr supplies search, requests, quota, and request history. Link users to their media-server identity. Matching usernames, display names, or email addresses do not grant access to a Seerr account; unlinked users cannot submit requests or read private history/quota through Veyra. TV requests offer only seasons still eligible at standard quality.
- Sonarr and Radarr supply queue, calendar, health, and storage data.
- Prowlarr supplies indexer and admin health data.

Configure Seerr's Radarr/Sonarr server, profile, root, and language defaults inside Seerr.

Calendar day headings and times use UTC. Movie entries distinguish theatrical, digital, and physical releases within the selected week. Optional unconfigured services are excluded from loading. Outages are labelled as unavailable or incomplete; complete cached data may be shown for up to 15 minutes.
