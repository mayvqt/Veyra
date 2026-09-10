# Documentation conventions

Write for one reader and one job. Keep first-run steps in [Setup](../setup.md),
settings in [Configuration](../configuration.md), service behavior in
[Integrations](../integrations.md), deployment basics in [Deployment](../deployment.md),
and contributor detail here. Describe shipped features in
[Capabilities](../capabilities.md), and link to a procedure instead of copying it.
Only add a roadmap or known-issues page when there is accepted future work or a
reproduced public issue to put in it.

For risky integration work, keep the agreed design stable while it is being
implemented and record execution evidence separately. Inventory upstream calls,
verify contracts against official documentation, test deterministic fixtures,
and make real-service checks clearly opt-in.

## Documentation impact map

| If you change... | Also update... |
| --- | --- |
| User-facing behavior | Nearest user page, [Capabilities](../capabilities.md) when scope changes, and the README if navigation changes |
| Setup or configuration | [Setup](../setup.md), [Configuration](../configuration.md), relevant deployment/troubleshooting pages, and nearest index |
| Integration behavior | [Integrations](../integrations.md), [Architecture](architecture.md), relevant user page, and nearest index |
| Package ownership or request flow | [Codebase map](codebase-map.md), [Architecture](architecture.md), and this index |
| Schema, encryption, retention, or cache | [Data](data.md), [Operations](operations.md) when upgrades change, and this index |
| Tests, tools, UI states, or CI | [Validation](validation.md) and this index |
| Security or privacy | [Security](../security.md), relevant architecture/data/operations page, and nearest index |
| Deployment, backup, health, or rollback | [Operations](operations.md), [Deployment](../deployment.md), and the README when navigation changes |

Add new documentation domains here and to the nearest index.
