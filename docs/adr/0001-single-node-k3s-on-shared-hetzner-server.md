# Single-node k3s on the shared Hetzner server, with Postgres data on a Cloud Volume

Spendulum runs on single-node k3s (pinned to the v1.36 channel) on the existing Hetzner Cloud server in Nuremberg, which also runs another non-root workload (`hermes`), Caddy and CrowdSec. We chose this over serverless or a dedicated server because running a real cluster is a goal of the project and the server already exists at a flat price. Postgres data lives on a 20 GB Hetzner Cloud Volume mounted at `/mnt/data`, with k3s's `default-local-storage-path` pointing into it, rather than on the faster local disk, so a server rebuild or resize means reattaching the Volume instead of restoring from backup.

## Consequences

- The host is shared, so `system-reserved` covers the other workload as measured (CrowdSec, hermes, Caddy, the OS): `system-reserved=memory=1792Mi`, `kube-reserved=memory=1Gi`. Re-measure if the other workload grows.
- The Volume is in the same location as the server; it protects against a rebuild, not a site loss. Off-site protection comes from backups (see ADR-0003).
- k3s must wait for the mount (`RequiresMountsFor=/mnt/data`), or local-path would write to the root disk.
- Host setup is an idempotent script plus config files in `deploy/host/`, run by hand. No Ansible for one node.
