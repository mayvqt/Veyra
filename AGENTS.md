# AGENTS.md

Engineering guidance for contributors and coding agents working on Veyra.

## Priorities

Optimize for security, correctness, maintainability, observability, and operability, in that order. Prefer straightforward Go, explicit ownership, and small cohesive changes. Preserve behavior unless the change intentionally updates it and includes matching tests and docs.

## Before Changing Code

1. Read the relevant handler, store, integration client, tests, and docs.
2. Check `git status` and preserve unrelated work.
3. Identify security, persistence, compatibility, and deployment impact.
4. Make the smallest cohesive change that fully solves the problem.

## Package Boundaries

| Path | Responsibility |
| --- | --- |
| `cmd/server` | Process startup and signal handling. |
| `internal/app` | Dependency wiring, lifecycle, background jobs. |
| `internal/config` | Environment parsing and validation. |
| `internal/http` | Router and shared HTTP concerns. |
| `internal/http/handlers` | Thin request orchestration and view models. |
| `internal/http/middleware` | Auth, CSRF, proxy, logging, limits, headers. |
| `internal/integrations/*` | Context-aware upstream clients. |
| `internal/store` | SQLite, migrations, transactions, audit, cache. |
| `internal/security` | Cryptography and redaction helpers. |
| `internal/http/templates`, `web/static` | Presentation only. |

Dependencies should point inward. HTTP may call store and integrations; store and integrations must not depend on HTTP presentation.

## Code Style

- Keep files focused. Split files that mix workflows or become hard to scan.
- Keep handlers small. Move SQL to `internal/store`, upstream protocol details to `internal/integrations`, and reusable policy to the owning package.
- Prefer typed structs, clear names, early returns, explicit data flow, and narrow APIs.
- Add interfaces only at consumer boundaries when they provide a real test seam.
- Avoid global mutable state, catch-all helpers, speculative abstractions, duplicated parsing, and dead compatibility code.
- Wrap errors with useful context while preserving the cause. Never expose raw internal errors to users.
- Use request contexts for database and network work. Do not replace a request context with `context.Background()` in request paths.

## Persistence

- Enable SQLite foreign keys on every connection.
- Preserve WAL mode, busy timeout, and bounded connection counts.
- Put schema changes in ordered, transactional, idempotent migrations.
- Test fresh databases and upgrades from the previous schema.
- Use transactions and constraints for durable invariants.
- Prefer schema introspection over driver error-string matching.

## HTTP And Integrations

- Configure bounded header/body sizes and read-header, read, write, and idle timeouts.
- Require authentication for protected routes and current admin authorization for admin routes.
- Require CSRF for state-changing browser requests.
- Keep cookies `HttpOnly`, `SameSite`, and `Secure` by default.
- Trust forwarded headers only from configured proxy CIDRs; validate derived IP, scheme, and host.
- Integration clients need timeouts, context propagation, bounded response bodies, status checks, and safe error redaction.
- JavaScript should progressively enhance working HTML and use stable `data-*` hooks.

## Security

- Validate configuration strictly and fail fast on invalid values.
- Document minimum entropy/length for secrets.
- Never log or render passwords, raw session IDs, CSRF values, API keys, access tokens, encryption keys, or secret-bearing URLs.
- Use cryptographically secure randomness and standard-library primitives.
- Treat auth, proxy handling, URL fetching, encryption, file access, and logging as security-sensitive.
- Bound user-controlled data before parsing, storing, logging, or forwarding it.

## Tests And Tooling

Behavior changes need tests at the lowest useful layer. Cover success, validation, authorization, upstream failure, and state-transition cases when relevant. Use fake transports and narrow fakes instead of real external services.

Before finishing, run:

```bash
gofmt -w <touched-go-files>
go test ./...
go vet ./...
go test -race ./...
git diff --check
```

CI also runs static and vulnerability analysis. Do not weaken checks to make a change pass.

## Docs And Deployment

- Document changed environment variables, routes, migrations, operational behavior, and security assumptions.
- Keep `.env.example`, Compose, Docker defaults, README, docs, and Unraid template values consistent.
- Containers must run unprivileged after the entrypoint prepares writable paths.
- Pin automation and image dependencies according to repository policy.
- Do not commit secrets, generated databases, local config, logs, build artifacts, or tool caches.

## Definition Of Done

A change is done when code, migrations, tests, docs, and deployment configuration agree; required checks pass; failure paths are safe; and the diff contains no unrelated churn.
