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

Online database backup (existing files are not overwritten):

```bash
docker exec veyra veyra backup /config/veyra-$(date +%Y%m%d-%H%M%S).db
```

The backup destination must support hard links; unsupported filesystems fail
without replacing an existing backup.

Keep backups off the application host together with a securely stored copy of
the original `ENCRYPTION_KEY` and deployment configuration. Test restoring a
backup into a separate instance before upgrading. Stop Veyra before replacing
its database, preserve the old database and its WAL/SHM files together, and
restore the snapshot as `/config/veyra.db` with ownership matching `PUID`/`PGID`.
Do not reuse stale WAL/SHM files with the restored snapshot. Start the matching
application version and check `/healthz`, login, settings, and integrations.
Schema migrations run forward at startup; rollback requires the matching
pre-upgrade backup rather than opening a newer schema with an older binary.

## Production maintenance

Only perform maintenance on an explicitly authorized instance. Confirm the target,
take and verify a current backup, preserve the encryption key and deployment
configuration, and avoid exposing credentials or private URLs in commands and
logs. Prefer reversible changes and check `/healthz` plus the affected user flow
afterward.

## Release and deploy

Release work requires explicit authorization beyond a local commit. Run the full
development validation suite, build the local binary and container path, and review
the exact version and image tag before publishing. Deploy immutable version tags or
digests, keep the pre-upgrade database backup, and verify health, login, settings,
and configured integrations after rollout. Do not treat a successful push as
deployment approval.
