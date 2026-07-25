# Setup

Veyra requires Jellyfin or Emby and two independent secrets:

- `SESSION_SECRET`: at least 32 characters.
- `ENCRYPTION_KEY`: exactly 32 characters.

Use the root README quick start, then open `http://localhost:3767/setup`.

Persist `/config` and back up `ENCRYPTION_KEY`. The first media-server administrator who signs in becomes a Veyra administrator.
