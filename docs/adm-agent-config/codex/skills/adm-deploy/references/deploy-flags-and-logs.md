> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das
# Deploy Flags and Logs

## Project isolation

- App scripts must default to the `<build-project>`.
- AI scripts must default to the `<ai-project>`.
- Every `gcloud` resource command must include `--project=`.

## Schema change sequence

1. Finalize intended code changes.
2. Build: `<build script> --env staging`
3. Deploy with migrate: `<deploy script> --env staging --migrate`

## Common deploy flags

- `--env <target>`
- `--migrate`
- `--seed-reset`
- `--dual-tenant`
- `--multi-tenant`
- `--alpha-ea-tenants`
- `--two-tenant-empty`
- `--data-migrate`
- `--cleanup`
- `--force`

## Logs

- Service logs: `gcloud run services logs read <service> --region <region> --project <build-project> --limit 200`
- Job executions: `gcloud run jobs executions list --job <job> --region <region> --project <build-project>`
- Execution details: `gcloud run jobs executions describe <execution> --region <region> --project <build-project>`
