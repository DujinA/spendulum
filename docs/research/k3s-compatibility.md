# k3s single-node compatibility: Traefik Gateway API, cert-manager, CloudNativePG

Research for issue #6. Researched on 2026-11-06. Versions are the latest stable releases on that date.

## Question

Do the planned cluster components work together on a current single-node k3s, and what configuration do they need?

The setup: one Hetzner Cloud x86 server (4 vCPU, 8 GB) running single-node k3s. The same server also runs another workload, outside Kubernetes, under a separate unprivileged user. GitOps uses Argo CD, Kustomize and Sealed Secrets. PostgreSQL runs on CloudNativePG with the barman-cloud plugin for off-server backups. Goose migrations run as an Argo CD PreSync Job. Database data sits on a Hetzner Cloud Volume used by k3s's local-path provisioner, and must not be readable by the other host user.

## Short answer

Yes, all the components work together, with these conditions:

1. **Pin k3s to the v1.36 channel, not `stable`.** The k3s `stable` channel is already on v1.37. CloudNativePG 1.30 and cert-manager 1.21 both support Kubernetes up to 1.36 only.
2. **Traefik's Gateway API provider is off by default.** Turn it on with a k3s `HelmChartConfig`. k3s already installs the standard-channel Gateway API CRDs (v1.6.3), so you do not install them yourself.
3. **cert-manager needs `config.gatewayAPI.enabled: true`.** Put the Gateway in the same namespace as its TLS Secret, and give each HTTPS listener a `hostname`.
4. **The barman-cloud plugin needs cert-manager.** It must be installed in the CloudNativePG operator's namespace. CloudNativePG 1.32 (about March 2027) removes the in-tree Barman support, so start on the plugin.
5. **A single-instance cluster blocks node drains** unless `enablePDB: false` is set.
6. **The default local-path StorageClass deletes data when a PVC is deleted.** Use a separate StorageClass with `reclaimPolicy: Retain` for the database.
7. **k3s already makes the storage root `0700` root-owned.** This keeps volume contents away from other host users, as long as the storage path is not set somewhere that undoes it. See the permissions section.
8. **A PreSync hook runs before anything else in its Application.** On the very first sync the database does not exist yet. Put the database in an earlier Application, or run the migration as a `Sync` hook in a later wave.
9. **Back up the Sealed Secrets keys off-server.** Do it again after every key renewal (every 30 days by default).

## Versions on 2026-11-06

| Component | Version to use | Notes |
|---|---|---|
| k3s | v1.36.6+k3s1 (channel `v1.36`) | `stable` and `latest` both point to v1.37.2+k3s1 ([channel server](https://update.k3s.io/v1-release/channels)) |
| Traefik (bundled) | v3.7.15, chart 40.1.0 on 1.36; chart 41.6.1 on 1.37 | [k3s release-1.36 traefik.yaml](https://github.com/k3s-io/k3s/blob/release-1.36/manifests/traefik.yaml), [master traefik.yaml](https://github.com/k3s-io/k3s/blob/master/manifests/traefik.yaml) |
| Gateway API CRDs (bundled) | v1.6.3, standard channel | See Traefik section |
| local-path-provisioner (bundled) | v0.0.37 | [v1.37 release notes](https://docs.k3s.io/release-notes/v1.37.X) |
| cert-manager | v1.21.3 | Supports Kubernetes 1.33 to 1.36 ([supported releases](https://cert-manager.io/docs/releases/)) |
| CloudNativePG | v1.30.1 | Supports Kubernetes 1.34 to 1.36; 1.37 is "tested, but not supported" ([supported releases](https://github.com/cloudnative-pg/cloudnative-pg/blob/main/docs/src/supported_releases.md)) |
| barman-cloud plugin | v0.15.1 | Needs CloudNativePG 1.26 or later ([installation](https://cloudnative-pg.io/plugin-barman-cloud/docs/installation/)) |
| Argo CD | v3.6.1 | Tested on Kubernetes 1.34 to 1.37 ([tested versions](https://github.com/argoproj/argo-cd/blob/v3.6.1/docs/operator-manual/tested-kubernetes-versions.md)) |
| Sealed Secrets | v0.41.2 (Helm chart 2.22.0) | [releases](https://github.com/bitnami-labs/sealed-secrets/releases) |
| Goose | v3.29.0 | [releases](https://github.com/pressly/goose/releases) |

Kubernetes 1.36 is the only minor version that every component officially supports. Install with `INSTALL_K3S_CHANNEL=v1.36`, or pin `INSTALL_K3S_VERSION`. Move to 1.37 only after cert-manager and CloudNativePG list it as supported. Both are expected to do so in their next minor releases (CloudNativePG 1.31 is planned for about December 2026).

## Traefik and Gateway API on k3s

**Provider off by default.** k3s ships Traefik v3 with Gateway API support turned off. The k3s docs say to turn it on with a `HelmChartConfig` ([k3s networking services](https://docs.k3s.io/networking/networking-services)). The Traefik chart default is `providers.kubernetesGateway.enabled: false` ([chart values v41.6.1](https://github.com/traefik/traefik-helm-chart/blob/v41.6.1/traefik/values.yaml)).

**CRDs already installed.** k3s installs the standard-channel Gateway API CRDs itself:
- On the 1.36 line they are bundled in the k3s `traefik-crd` chart, at bundle version v1.6.3, with `helm.sh/resource-policy: keep` ([k3s-charts patch](https://github.com/k3s-io/k3s-charts/blob/main/packages/traefik-1.34-1.36/generated-changes/patch/crds/gateway-standard-install.yaml.patch)).
- On 1.37 and later they are a separate packaged component called `gateway-api-crd` (chart 1.6.301). It can be turned off with `--disable=gateway-api-crd` ([manifest](https://github.com/k3s-io/k3s/blob/master/manifests/gateway-api-crd.yaml)).
- The k3s docs page still says "Gateway API v1.4". The shipped manifests are newer, and Traefik v3.7 supports Gateway API standard v1.6.3 ([Traefik provider docs](https://doc.traefik.io/traefik/reference/install-configuration/providers/kubernetes/kubernetes-gateway/)).
- Do not install another copy of the CRDs from Argo CD. Only install the experimental channel if you need TCPRoute, which this app does not.

**Default Gateway.** When the provider is on, the Traefik chart creates a GatewayClass `traefik` (controller `traefik.io/gateway-controller`). It also creates a default Gateway `traefik-gateway` in Traefik's namespace (`kube-system` on k3s), with only an HTTP listener on port 8000. The `websecure` listener is left out by default because it needs `certificateRefs` ([gateway template](https://github.com/traefik/traefik-helm-chart/blob/v41.6.1/traefik/templates/gateway.yaml), [gatewayclass template](https://github.com/traefik/traefik-helm-chart/blob/v41.6.1/traefik/templates/gatewayclass.yaml)). The GatewayClass is created whenever the provider is on, even when `gateway.enabled` is false.

**Listener ports.** Gateway listener ports are Traefik entrypoint (container) ports: 8000 for `web` and 8443 for `websecure`. They are not the Service ports 80 and 443. ServiceLB exposes ports 80 and 443 on the host.

**Configuration.** Put this in `/var/lib/rancher/k3s/server/manifests/traefik-config.yaml` on the host. It is cluster bootstrap, so it should exist before Argo CD runs:

```yaml
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: traefik
  namespace: kube-system
spec:
  valuesContent: |-
    providers:
      kubernetesGateway:
        enabled: true
    gateway:
      enabled: false   # we define our own Gateway in Git, next to the TLS Secret
```

Notes:
- Do not edit the k3s-managed `traefik.yaml`, because k3s overwrites it at startup. If a `HelmChartConfig` already exists, add values to it rather than creating a second one ([k3s networking services](https://docs.k3s.io/networking/networking-services)).
- Since chart v40 (k3s June 2026 releases), the Service type key is `service.spec.type`. The old `service.type` key is silently ignored.

Then define our own Gateway in Git. It lives in the app namespace so cert-manager can manage the Secret there:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: web
  namespace: spendulum
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  gatewayClassName: traefik
  listeners:
    - name: http
      port: 8000
      protocol: HTTP
      hostname: budget.example.org
    - name: https
      port: 8443
      protocol: HTTPS
      hostname: budget.example.org
      tls:
        mode: Terminate
        certificateRefs:
          - name: budget-tls
```

The app's HTTPRoute attaches to `sectionName: https`. A second HTTPRoute on `sectionName: http`, with a `RequestRedirect` filter (`scheme: https`, `statusCode: 301`), handles the redirect to HTTPS. The ACME challenge route that cert-manager creates uses an `Exact` path match. Gateway API gives Exact matches priority over a `PathPrefix: /` redirect, so the challenge still reaches the solver. Check this once after the first certificate is issued.

## cert-manager with Let's Encrypt via Gateway API

Source: [cert-manager Gateway docs](https://cert-manager.io/docs/usage/gateway/) and [HTTP-01 docs](https://cert-manager.io/docs/configuration/acme/http01/).

- **Turn it on.** Gateway API support has not been behind a feature gate since 1.15, but it is off by default. Set Helm value `config.gatewayAPI.enabled: true`.
- **Start order.** cert-manager checks for Gateway API CRDs only at startup. On k3s the CRDs exist from first boot, so this is fine. If they are ever reinstalled, restart cert-manager.
- **Listener rules for the annotation shim.** A listener is used only if it has a non-empty `hostname`, `tls.mode: Terminate`, and a `certificateRefs` entry naming a Secret in the Gateway's own namespace. Cross-namespace Secrets are not supported. This is why the Gateway above lives in the app namespace and not in `kube-system`.
- **HTTP-01 solver.** Use a `gatewayHTTPRoute` solver whose `parentRefs` point at our Gateway:

  ```yaml
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: letsencrypt-prod
  spec:
    acme:
      server: https://acme-v02.api.letsencrypt.org/directory
      privateKeySecretRef:
        name: letsencrypt-prod-account
      solvers:
        - http01:
            gatewayHTTPRoute:
              parentRefs:
                - name: web
                  namespace: spendulum
                  kind: Gateway
  ```

  cert-manager creates a temporary HTTPRoute that sends `/.well-known/acme-challenge/<token>` to a solver Service, then deletes it. The Gateway needs a listener that is reachable on public port 80. Here that is listener port 8000, exposed as port 80 by ServiceLB. Open port 80 in the Hetzner firewall.
- **Optional.** ListenerSet support (alpha in 1.20) is not needed for one app.
- **Argo CD health.** Argo CD has built-in health checks for `cert-manager.io` and `acme.cert-manager.io` resources, and for Gateway, HTTPRoute and GatewayClass ([resource_customizations](https://github.com/argoproj/argo-cd/tree/master/resource_customizations)).

## CloudNativePG with the barman-cloud plugin on one node

**Version support.** CloudNativePG 1.30.x supports Kubernetes 1.34, 1.35 and 1.36, and PostgreSQL 14 to 18. 1.29 reached end of life on 29 September 2026 ([supported releases](https://github.com/cloudnative-pg/cloudnative-pg/blob/main/docs/src/supported_releases.md)).

**In-tree Barman is going away.** The docs say: "CloudNativePG 1.31 is the last minor release that ships with native Barman Cloud support. It is removed starting from 1.32.0." Use the plugin from day one so there is nothing to migrate.

**Plugin install requirements** ([installation](https://cloudnative-pg.io/plugin-barman-cloud/docs/installation/)):
- CloudNativePG 1.26 or later.
- cert-manager must be installed first. The plugin uses it for its TLS certificates.
- The plugin must be in the same namespace as the operator, usually `cnpg-system`. Install with the `cnpg/plugin-barman-cloud` Helm chart or the release `manifest.yaml`.

**Configuration** ([usage](https://cloudnative-pg.io/plugin-barman-cloud/docs/usage/), [retention](https://cloudnative-pg.io/plugin-barman-cloud/docs/retention/), [CNPG backup docs](https://github.com/cloudnative-pg/cloudnative-pg/blob/v1.30.1/docs/src/backup.md)):
- An `ObjectStore` (`barmancloud.cnpg.io/v1`) holds `destinationPath`, `endpointURL`, the S3 credentials (from a Secret, sealed in Git), `wal.compression`, and `retentionPolicy: "30d"`. Leave `serverName` empty in the ObjectStore.
- The `Cluster` turns on archiving through `spec.plugins: [{name: barman-cloud.cloudnative-pg.io, isWALArchiver: true, parameters: {barmanObjectName: <store>}}]`.
- `ScheduledBackup` must set `method: plugin` and `pluginConfiguration.name: barman-cloud.cloudnative-pg.io`. The default method is still `barmanObjectStore`. Its schedule is a six-field cron that includes seconds, for example `"0 0 3 * * *"`.
- To restore, use `externalClusters[].plugin` with `bootstrap.recovery.source`. A restored cluster does not archive WAL until you add a `plugins` entry, and that should point at a new store path.
- Which object store to use is covered by issue #9 (`docs/research/backup-storage.md` on its branch).

**Single-instance caveats** ([Kubernetes upgrade docs](https://github.com/cloudnative-pg/cloudnative-pg/blob/v1.30.1/docs/src/kubernetes_upgrade.md)):
- With `instances: 1`, the operator creates PodDisruptionBudgets that stop the node from being drained. This blocks `kubectl drain` and any automated k3s upgrade that drains, such as system-upgrade-controller. Set `enablePDB: false`, and expect a short database outage during node maintenance. For a single-user app this is fine.
- The data lives only on this node and this volume. Off-server WAL archiving plus base backups is the real protection. Test restores regularly.
- The local-path provisioner does not enforce PVC size ([README](https://github.com/rancher/local-path-provisioner/blob/v0.0.37/README.md)). The database can fill the whole Hetzner Volume. Watch disk usage.

**Prevent accidental data deletion.**
- The k3s `local-path` StorageClass uses `reclaimPolicy: Delete`. The CloudNativePG Cluster owns its PVCs, so if Argo CD prunes the `Cluster` (for example after a bad Kustomize change), the PVC, the PV and the directory on disk are all deleted.
- Mitigations:
  - Add a second StorageClass (provisioner `rancher.io/local-path`, `reclaimPolicy: Retain`, `volumeBindingMode: WaitForFirstConsumer`) and use it in `spec.storage.storageClass`.
  - Annotate the `Cluster` with `argocd.argoproj.io/sync-options: Prune=false,Delete=false`.

**Argo CD health.** Argo CD has a built-in health check for `postgresql.cnpg.io/Cluster` ("Cluster in healthy state" means Healthy). Sync waves can therefore wait for the database to be ready ([health.lua](https://github.com/argoproj/argo-cd/blob/master/resource_customizations/postgresql.cnpg.io/Cluster/health.lua)).

## local-path storage on a Hetzner Volume, and keeping database files private

**What k3s does.** k3s runs its own `local-path-config` ConfigMap ([manifest](https://github.com/k3s-io/k3s/blob/master/manifests/local-storage.yaml)). Its setup script is:

```sh
mkdir -m 0777 -p "${VOL_DIR}"
chmod 700 "${VOL_DIR}/.."
```

- Each volume directory is world-writable (`0777`) so any pod UID can write to it.
- The parent directory, which is the storage root, is set to `0700` and owned by root.
- This was a deliberate fix: "/var/lib/rancher/k3s/storage/ should be 700, /var/lib/rancher/k3s/storage/* should be 777" ([k3s commit a736b4b](https://github.com/k3s-io/k3s/commit/a736b4b1), fixing k3s issue #2348).
- A non-root host user cannot enter the storage root, so it cannot read or list any volume, whatever the modes inside it.

**Inside the volume.** CloudNativePG runs PostgreSQL as UID/GID 26 by default ([security docs](https://github.com/cloudnative-pg/cloudnative-pg/blob/v1.30.1/docs/src/security.md)). `initdb` creates `PGDATA` as `0700`, owned by UID 26. That is a second layer of protection: only root and UID 26 can read the data files. Make sure the other workload's user is not UID 26. Some distributions give UID 26 to a `postgres` system account.

**Pointing storage at the Hetzner Volume.**
- Set `default-local-storage-path` in `/etc/rancher/k3s/config.yaml` to a **subdirectory** of the Volume mount, for example `/mnt/data/k3s-storage`. Do not use the mount point itself. k3s then chmods `k3s-storage` to `0700` on each provisioning.
- Do the same once by hand at setup time (`install -d -m 0700 -o root -g root /mnt/data/k3s-storage`), so the directory is never open in between.
- Mount the Volume with `nodev,nosuid`, and make the mount point root-owned and not writable by others.

**Things to avoid:**
- A custom StorageClass `pathPattern` such as `{{ .PVC.Namespace }}/{{ .PVC.Name }}/`. The k3s script then chmods only the namespace directory, not the storage root. Keep the default pattern, or check modes yourself.
- Replacing the k3s `local-path-config` without keeping the `chmod 700` line. The upstream local-path-provisioner example only does `mkdir -m 0777`.
- k3s starting before the Volume is mounted. It would create the storage directory on the root disk underneath the mount point. Add a systemd drop-in for `k3s.service` with `RequiresMountsFor=/mnt/data` (or use `x-systemd.required-by=k3s.service` in fstab).

**Other host paths that hold secrets or data:**
- Keep `write-kubeconfig-mode` at its default `0600`. The k3s docs warn that `644` lets other unprivileged users read the admin kubeconfig ([k3s server CLI](https://docs.k3s.io/cli/server)). Admin access to the cluster gives access to everything.
- Turn on `secrets-encryption: true`, so Secrets (database passwords, S3 keys) are encrypted in the k3s datastore under `/var/lib/rancher/k3s/server`.
- Do not add the other user to any group that owns k3s or containerd sockets.
- Check on the real host that `/var/log/pods` log files are not world-readable. PostgreSQL logs go to stdout there.

**Encryption at rest.** None of this protects the disk image from Hetzner or from someone who has the Volume. If that matters, use LUKS on the Volume. It is out of scope for this ticket.

**Quick check after setup:**

```sh
sudo -u <otheruser> ls /mnt/data/k3s-storage    # expect: Permission denied
stat -c '%a %U' /mnt/data/k3s-storage           # expect: 700 root
```

## Resource reservations (`--system-reserved`, `--kube-reserved`)

Source: [Kubernetes: reserve compute resources](https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/), [node-pressure eviction](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/), [k3s configuration: kubelet configuration files](https://github.com/k3s-io/docs/blob/main/docs/installation/configuration.md).

**What the settings do.**
- `kubeReserved` is for Kubernetes daemons. On k3s that is the single `k3s` process: API server, controllers, kubelet, containerd and the SQLite datastore.
- `systemReserved` is for the OS, the kernel and anything else outside Kubernetes. **On this server that includes the other workload.** Size it to that workload's peak memory, plus about 0.5 to 1 GB for the OS.
- The scheduler never places pods beyond Allocatable, which is Capacity minus kubeReserved, minus systemReserved, minus the hard eviction threshold.
- Reserving without enforcement (`enforceNodeAllocatable` left at `["pods"]`) only lowers Allocatable. That is what we want.
- Enforcing `kube-reserved` or `system-reserved` needs cgroups that already exist. The kubelet does not create them. This adds risk on k3s for little gain, so do not enforce them.

**Eviction defaults trap.** If you set any `evictionHard` value, the other defaults become zero unless `mergeDefaultEvictionSettings: true` is set. List them all.

**How to set them.** k3s 1.32 and later recommends a kubelet drop-in file in `/var/lib/rancher/k3s/agent/etc/kubelet.conf.d/`. `kubelet-arg` entries in `config.yaml` also work. A starting point for 8 GB, assuming the other workload needs about 1 GB at peak:

```yaml
# /var/lib/rancher/k3s/agent/etc/kubelet.conf.d/10-reserved.conf
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
systemReserved:
  cpu: 500m
  memory: 2Gi      # OS + the other workload; adjust to its measured peak
kubeReserved:
  cpu: 500m
  memory: 1Gi      # k3s server process (apiserver, kubelet, containerd, kine)
evictionHard:
  memory.available: "200Mi"
  nodefs.available: "10%"
  nodefs.inodesFree: "5%"
  imagefs.available: "15%"
  imagefs.inodesFree: "5%"
```

This leaves about 4.8 GiB allocatable for pods. Check with `kubectl describe node` after a restart.

**Also:**
- Set memory requests equal to memory limits on the PostgreSQL `Cluster` and the app.
- Consider a systemd `MemoryMax=` on the other workload's unit, so neither side can push the other into the OOM killer.
- Measure the real headroom in the memory budget ticket.

## Argo CD PreSync hook for Goose migrations

Source: [Argo CD sync phases, waves and hooks](https://argo-cd.readthedocs.io/en/stable/user-guide/sync-waves/).

**How hooks behave:**
- `argocd.argoproj.io/hook: PreSync` runs the Job before the Sync phase. If it fails, "the whole sync process will stop and will be marked as failed". That is the behavior we want for migrations.
- Hooks run on **every** sync, not only when migrations change. Goose's `up` is a no-op when the database is current, so this is safe.
- The default delete policy is `BeforeHookCreation`, which is meant to be used with a fixed `metadata.name`. Use `argocd.argoproj.io/hook-delete-policy: BeforeHookCreation,HookSucceeded`. A failed Job then stays around for its logs and is replaced on the next sync.
- The docs warn that a PreSync hook runs **before** the Application's own Secrets, ConfigMaps and ServiceAccounts are applied.

**The first-sync problem.** On the very first sync of a single Application that contains both the CloudNativePG `Cluster` and the migration Job, the PreSync Job runs before the Cluster exists. The operator-generated `<cluster>-app` Secret also does not exist yet. The Job fails and the sync stops. Two ways out:

1. **Split Applications (fits "PreSync").** One Application holds the database (`Cluster`, `ObjectStore`, `ScheduledBackup`, sealed S3 credentials). A second holds the app (migration PreSync Job, Deployment, HTTPRoute). In an app-of-apps, give the database Application an earlier sync wave. Argo CD only waits on child Application health if the `argoproj.io/Application` health check is added back in `argocd-cm`, because it was removed from defaults in v1.8. The migration Job reads the DSN from the `<cluster>-app` Secret, which the operator creates, so the "Secrets not yet applied" warning does not apply.
2. **One Application, Sync hook plus waves.** Keep everything in one Application:
   - `Cluster` at wave 0, so Argo waits for its built-in health check.
   - The Goose Job as `hook: Sync` at wave 1.
   - The Deployment at wave 2.

   The migration still runs before the new app version rolls out, and it fails the sync on error.

Both work. Option 1 also keeps app changes from ever touching the database Application, which pairs well with `Prune=false` on the `Cluster`.

**Job settings:**
- `backoffLimit: 0` or 1, and an `activeDeadlineSeconds`.
- `restartPolicy: Never`.
- The same image tag as the app, so the embedded migrations match the code.
- Run as non-root.

## Sealed Secrets key backup

Source: [Sealed Secrets README](https://github.com/bitnami-labs/sealed-secrets/blob/main/README.md).

**Backup:**

```sh
kubectl get secret -n kube-system -l sealedsecrets.bitnami.com/sealed-secrets-key -o yaml > main.key
```

- The file contains private keys. Store it off-server and encrypted, for example in a password manager. Never put it in Git.
- The controller adds a new sealing key every 30 days by default. Old keys are kept, not rotated out. A backup taken before a renewal cannot decrypt secrets sealed after it, so repeat the backup after every renewal. Either automate it, or set a known `--key-renew-period` and back up on that schedule. `0` turns renewal off, but the README warns against doing that casually.

**Restore** (rebuilding the server):
- Apply `main.key` **before** the controller starts, so before Argo CD syncs Sealed Secrets.
- If the controller is already running, apply the file and delete the controller pod so it reloads.

**Offline sealing.** Run `kubeseal --fetch-cert > cert.pem` once, then use `kubeseal --cert cert.pem`. Refresh the cert after renewals.

**Name mismatch.** The Helm chart's default controller name is `sealed-secrets`, but `kubeseal` expects `sealed-secrets-controller` by default. Either set `fullnameOverride: sealed-secrets-controller`, or pass `--controller-name` to `kubeseal`. Argo CD has a built-in health check for `SealedSecret`.

## Install order

1. Hetzner Volume mounted at `/mnt/data`. Create `/mnt/data/k3s-storage` with mode `0700`. Add the systemd mount dependency.
2. k3s, channel `v1.36`. Its `config.yaml` sets `default-local-storage-path`, `secrets-encryption: true` and `write-kubeconfig-mode` left at `0600`. Place the kubelet reservation drop-in and the Traefik `HelmChartConfig` before the first start.
3. Sealed Secrets controller. On a rebuild, restore the key first.
4. Argo CD, then everything else from Git:
   - cert-manager, with `config.gatewayAPI.enabled: true`.
   - CloudNativePG operator.
   - barman-cloud plugin, which needs cert-manager.
   - ClusterIssuer.
   - Database Application.
   - App Application: Gateway, HTTPRoutes, migration hook, Deployment.

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| k3s auto-upgrade or `stable` channel moves to 1.37 before cert-manager and CloudNativePG support it | Running unsupported versions | Pin channel `v1.36`. Upgrade k3s only after checking both support matrices |
| Argo CD prune deletes the `Cluster` | PVC, PV and on-disk data deleted (`reclaimPolicy: Delete`) | Retain StorageClass. `Prune=false,Delete=false` on the Cluster. Off-server backups |
| Single-instance PDB blocks drain | k3s upgrades hang | `enablePDB: false` |
| Volume not mounted when k3s starts | Data written to the root disk, then hidden under the mount | `RequiresMountsFor=` drop-in |
| Custom `pathPattern` or replaced `local-path-config` | Storage root left `0755`, so volume names are visible and protection depends on inner modes | Keep the k3s defaults; check with `stat` |
| PreSync hook on first sync | First deploy fails | Split Applications, or a Sync hook with waves |
| Sealed Secrets key lost or stale | Every SealedSecret in Git becomes undecryptable after a rebuild | Off-server key backup after each renewal |
| CloudNativePG 1.32 drops in-tree Barman | Forced migration later | Use the plugin from the start |
| local-path ignores PVC size | Database fills the Volume | Disk-usage alerting. Size the Volume with headroom |
| Doc drift: k3s docs say Gateway API v1.4 while v1.6.3 ships | Confusion when following docs | Trust the manifests; check `kubectl get crd gateways.gateway.networking.k8s.io -o yaml` for `bundle-version` |

## Recommendations

- Use k3s **v1.36** (pinned channel), with bundled Traefik, Gateway API turned on through `HelmChartConfig`, and the chart's default Gateway turned off.
- Define our own Gateway in the app namespace, with an HTTP listener (8000) and an HTTPS listener (8443) and the cert-manager `cluster-issuer` annotation. Use an HTTP-01 `gatewayHTTPRoute` solver and an HTTPRoute that redirects HTTP to HTTPS.
- Use cert-manager 1.21 with `config.gatewayAPI.enabled: true`.
- Use CloudNativePG 1.30 with `instances: 1`, `enablePDB: false`, a Retain StorageClass and `Prune=false`. Use barman-cloud plugin 0.15 for WAL and base backups, with `method: plugin` on ScheduledBackup.
- Keep the k3s local-path defaults, point the storage path at a `0700` subdirectory of the Hetzner Volume, keep the kubeconfig at `0600`, and turn on secrets encryption.
- Reserve memory for the other workload in `systemReserved`. Do not enforce the reserved cgroups.
- Run migrations from a separate app Application with a PreSync hook, after a database Application. Otherwise use a Sync hook at a wave after the Cluster.
- Script the Sealed Secrets key backup and run it after every renewal.
