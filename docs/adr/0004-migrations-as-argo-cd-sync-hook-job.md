# Migrations run as an Argo CD Sync-hook Job, not at API startup

Goose migrations run as a Kubernetes Job (`spendulum migrate`, the same image as the API) declared as an Argo CD Sync hook, in a sync wave after the CloudNativePG Cluster and before the API Deployment, with hook-delete-policy `BeforeHookCreation`. The API never migrates itself: if it finds pending migrations at startup it exits non-zero. This follows the fail-fast rule (a bad migration stops the sync visibly instead of crash-looping the API) and works for any number of replicas.

## Considered Options

- **PreSync hook**: rejected because on the first sync it runs before the Cluster exists and fails.
- **Migrate at API startup under an advisory lock**: simplest, but mixes schema changes with serving and hides migration failures as pod restarts.

## Consequences

- Local development on compose runs the same `migrate` subcommand before starting the API.
