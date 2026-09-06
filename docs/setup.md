# Setup

Veyra requires Jellyfin or Emby and two independent secrets:

- `SESSION_SECRET`: at least 32 characters.
- `ENCRYPTION_KEY`: exactly 32 characters.

Use the root README quick start, then open `http://localhost:3767/setup`.

Complete setup on a trusted local network before exposing Veyra publicly. The setup wizard is unauthenticated while required configuration is missing. Leave wizard-managed environment values empty unless you intend to override the wizard; see [Configuration](configuration.md).

Persist `/config` and back up `ENCRYPTION_KEY`. Every media-server administrator who signs in receives Veyra administrator access; this is not limited to the first account. Administrator status is rechecked after 15 minutes on subsequent requests, with access denied if verification fails.
