# AgenticDevelopmentModel Environment Configurations

## Staging vs Production

| Parameter | Staging | Production |
|-----------|---------|------------|
| Service name | `<staging-service>` | `<prod-service>` |
| Managed Postgres instance | `<staging-db-instance>` | `<prod-db-instance>` |
| Managed Postgres tier | `<managed Postgres tier>` (~400 max connections, managed connection pooling) | `<managed Postgres tier>` (~400 max connections, managed connection pooling) |
| CPU | 2 vCPU | 2 vCPU |
| Memory | 2Gi | 2Gi |
| Min instances | 0 | 1 |
| Max instances | 3 | 3 |
| Concurrency | 80 | 80 |
| PG_MAX_CONNECTIONS | 100 | 100 |
| CLUSTER_WORKERS | 1 | 1 |
| PLATFORM_DOMAIN | `<staging-domain>` | `<prod-domain>` |

## PG_MAX_CONNECTIONS Formula

```
PG_MAX_CONNECTIONS = floor(managed_Postgres_max_connections / MAX_INSTANCES / CLUSTER_WORKERS)
```

With the managed Postgres tier above (~400 max connections), 3 max instances, 1 worker:
- Each instance gets: requestMax ~60, sessionMax 6, workerMax ~18
- 3 instances x 90 connections = 270 < 400 (safe headroom for superuser/migration/tooling)
- **Managed connection pooling enabled** — the managed Postgres service handles connection lifecycle, idle timeout, and recycling via a built-in pooler. The app connects to the pooler port via the managed auth proxy.

## Secrets Per Environment

**Production:** `<db-url secret>`, `<session secret>`, `<model-api-key secret>`, `<smtp-user secret>`, `<smtp-pass secret>`, `<vault-key secret>`

**Staging:** `<staging db-url secret>`, `<staging session secret>`, `<model-api-key secret>` (shared), `<smtp-user secret>` (shared), `<smtp-pass secret>` (shared), `<vault-key secret>` (shared)
