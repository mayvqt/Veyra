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
