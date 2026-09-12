# Operations

Give Veyra one writable `/config` mount, its listening port, and only the
outbound access needed for configured services. The container drops privileges
and enables `no-new-privileges`; match `PUID` and `PGID` to the config
directory owner. Trust forwarded headers only from the directly connected proxy.
Keep `.env`, `ENCRYPTION_KEY`, `SESSION_SECRET`, API keys, databases, logs,
and backups private, and redact diagnostics before sharing them.

## Back up and restore

Create a consistent online snapshot without overwriting an existing file:

```bash
docker exec veyra veyra backup /config/veyra-$(date +%Y%m%d-%H%M%S).db
```

The destination must support hard links. Store the snapshot off-host with the
original encryption key and deployment configuration. Regularly restore it into
a separate instance running the matching version. Do not mix a restored database
with stale WAL/SHM files. Check ownership, `/healthz`, login, settings, and each
configured integration, and record when the restore last succeeded.

## Release and rollback

The complete CI workflow must pass for the exact revision before image
publication. Deploy a version tag or digest and record it with the pre-upgrade
backup; `latest` is a moving tag. After deployment, check `/healthz`, login,
administrator access, dashboard partial-failure behavior, and configured
integrations without putting secrets in logs.

Migrations are forward-only. To roll back across a schema change, stop Veyra,
preserve the failed state for diagnosis, restore the matching pre-upgrade database
without stale WAL/SHM files, and run the previous immutable image. Repairs that
change users, settings, sessions, audit records, or cache state must be explicit
operator actions.

The first upgrade that records authentication origin requires users to sign in again. Later media-server type or internal URL changes also revoke sessions; Seerr-only and presentation changes preserve them. User and audit history remain intact.
