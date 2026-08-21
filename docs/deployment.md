# Deployment

Docker Compose publishes `3767` and maps `./config` to `/config`.

For HTTPS:

```env
APP_BASE_URL=https://veyra.example.com
COOKIE_SECURE=true
TRUSTED_PROXY_CIDRS=YOUR_PROXY_CIDR
```

Trust only the actual proxy network and forward the original scheme and host.

For Unraid, install `templates/unraid/veyra.xml`, map `/mnt/user/appdata/veyra` to `/config`, and set the required secrets.

## Database backups

Create a consistent online SQLite backup without stopping Veyra:

```bash
docker exec veyra veyra backup /config/veyra-$(date +%Y%m%d-%H%M%S).db
```

The command validates the source database, uses SQLite's online backup operation,
sets restrictive permissions, and refuses to overwrite an existing file. Copy the
result out of the container or include `/config` in your normal backup rotation.
