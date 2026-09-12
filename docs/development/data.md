# Data and migrations

`internal/store/db.go` owns the schema and ordered migrations.
`PRAGMA user_version` records the current version. SQLite stores users, hashed
session IDs with encrypted media-server tokens, setup settings, audit logs, and
expiring cache entries. The media server and other integrations remain the source
of truth for upstream data.

`internal/store` is the only SQL owner. Connections use WAL, foreign keys,
`synchronous=NORMAL`, a five-second busy timeout, in-memory temporary storage,
one open connection, private directories, and mode `0600` database files.
Secret setup values and access tokens are encrypted with `ENCRYPTION_KEY`;
`SESSION_SECRET` protects session identifiers. Losing or changing either secret
can make persisted state unusable.

Migrations are forward-only and transactional. Append a migration rather than
changing one that may already have run. Prefer expand/contract changes when old
and new code must overlap, and reject schemas newer than the binary.

Every schema change needs a fresh-install test, upgrades from each affected
supported version, preservation checks for rows and encrypted values, repeat
startup coverage, and a failed-migration test. Update [Operations](operations.md)
whenever an upgrade changes backup or rollback requirements.

Integration cache keys include a SHA-256 namespace derived from both media-server
and Seerr configuration and the other upstream settings. Numeric upstream user IDs
are never reused across configuration changes. Failed concurrent refreshes share
one result with a five-second cooldown; canceled waiters return independently.

`settings.auth.origin` stores the normalized media-server type and internal URL.
Router startup compares it before accepting sessions and transactionally revokes
sessions for a different or unknown legacy origin. Users and audit history remain.
This does not require a schema revision.
