# Security

Report vulnerabilities through GitHub private vulnerability reporting.

Include affected versions, impact, reproduction, and mitigation without credentials, tokens, private URLs, or sensitive logs.

Security fixes target `main` until stable release branches exist.

## Secret handling

- API keys and media-server access tokens are sent in request headers, never URL query parameters.
- Setup values stored in SQLite are encrypted with `ENCRYPTION_KEY`; secret fields are never prefilled in the web UI.
- Service URLs containing credentials, query parameters, or fragments are rejected.
- Structured logging applies a final redaction layer for configured secrets, credential-bearing URLs, sensitive attribute names, and credential-related errors.
- Audit and integration diagnostics are sanitized again before rendering, including older database records.

Treat log redaction as defense in depth. Do not add secrets to log messages or audit metadata, and do not log complete request headers, cookies, or forms.
