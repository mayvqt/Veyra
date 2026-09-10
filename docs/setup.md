# Setup

Veyra requires Jellyfin or Emby and two independent secrets:

- `SESSION_SECRET`: at least 32 characters.
- `ENCRYPTION_KEY`: exactly 32 characters.

Copy the example environment and generate both secrets:

```bash
cp .env.example .env
openssl rand -hex 32 # use this for SESSION_SECRET
openssl rand -hex 16 # use this for ENCRYPTION_KEY
```

Replace the placeholders in `.env`, then start Veyra:

```bash
docker compose up -d
```

Open `http://localhost:3767/setup`.

Complete setup on a trusted local network before exposing Veyra publicly. The setup wizard is unauthenticated only before any setup settings or users have been saved. Leave wizard-managed environment values empty unless you intend to override the wizard; see [Configuration](configuration.md).

Persist `/config` and back up `ENCRYPTION_KEY`. Every media-server administrator who signs in receives Veyra administrator access; this is not limited to the first account. Administrator status is rechecked after 15 minutes on subsequent requests, with access denied if verification fails.
