# Deployment

Docker Compose publishes `3767` and maps `./config` to `/config`.

Complete [Setup](setup.md) on a trusted network before enabling public access.
The current image workflow builds for Linux amd64. `latest` follows `main`;
use an exact published version tag or image digest for a reproducible deployment.
Published binaries identify their release tag or commit; local builds report `dev`.

For HTTPS:

```env
APP_BASE_URL=https://veyra.example.com
COOKIE_SECURE=true
TRUSTED_PROXY_CIDRS=YOUR_PROXY_CIDR
```

Trust only the actual proxy network and forward the original scheme and host.

For Unraid, install `templates/unraid/veyra.xml`, map `/mnt/user/appdata/veyra` to `/config`, and set the required secrets.

Follow [Operations](development/operations.md) for backup, restore, health
verification, release, and rollback procedures.
