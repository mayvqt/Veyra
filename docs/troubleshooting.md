# Troubleshooting

```bash
docker compose logs -f veyra
docker compose restart veyra
```

Verify:

- container-reachable service URLs and matching API keys;
- linked Seerr users and Seerr's Radarr/Sonarr defaults;
- persistent writable `/config`;
- `APP_BASE_URL`, HTTPS, cookie security, and proxy trust.

Redact secrets, tokens, cookies, private URLs, and response bodies before sharing logs.

## Setup returns after saving

Nonempty environment values override saved wizard values. If you copied an older `.env.example`, remove the sample integration URLs and `change-me` keys from `.env`, leaving those variables empty, then recreate the container with `docker compose up -d`. A restart alone does not reload the container environment. Complete setup again if needed. Keep intentional environment overrides and your existing session and encryption secrets.
