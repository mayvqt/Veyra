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

Online database backup (existing files are not overwritten):

```bash
docker exec veyra veyra backup /config/veyra-$(date +%Y%m%d-%H%M%S).db
```
