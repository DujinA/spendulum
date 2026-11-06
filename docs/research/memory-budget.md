# Memory budget on an 8 GB node and monitoring choice

Research for issue #7. Researched on 2026-11-06.

## Question

Will the planned stack fit on one Hetzner Cloud x86 server (4 vCPU, 8 GB RAM) that runs single-node k3s and also hosts another modest non-root workload? Which monitoring setup should v1 use:

- **(a)** self-hosted kube-prometheus-stack, Loki in single-binary mode, and Grafana Alloy, with 14-day retention, or
- **(b)** Grafana Alloy shipping to the Grafana Cloud free tier?

What `--system-reserved` and `--kube-reserved` values should the node use?

## Answer

**Use option (b), Alloy shipping to the Grafana Cloud free tier.**

| | Steady-state budget | Headroom on ~7.7 GiB usable |
|---|---|---|
| Platform + app + other workload (shared by both options) | ~4.6 GiB | n/a |
| **Option (a)** total, self-hosted monitoring (+~2.1 GiB) | **~6.8 GiB** | ~0.9 GiB, tight |
| **Option (b)** total, Grafana Cloud free tier (+~0.4 GiB) | **~5.1 GiB** | ~2.6 GiB, comfortable |

Option (a) does fit on paper. It leaves under 1 GiB for page cache, Prometheus compaction and query spikes, Argo CD repo-server render spikes, and growth in the other workload. Postgres and the Prometheus TSDB both lean on the page cache, so a node running this close to full would behave badly long before the OOM killer acts. Option (b) costs about 2 GiB less. Its limits (10k active series, 50 GB logs per month, 14-day retention) are far above what one user with tiny traffic produces, as long as metrics are scraped every 60 s and filtered to an allowlist.

Reserved resources for `/etc/rancher/k3s/config.yaml` (see the [Reserved resources](#reserved-resources) section for the reasoning):

```yaml
kubelet-arg:
  - "kube-reserved=cpu=500m,memory=1Gi"
  - "system-reserved=cpu=500m,memory=1536Mi"   # OS ~0.5 GiB + the other workload (1 GiB placeholder)
  - "eviction-hard=memory.available<256Mi,nodefs.available<10%,nodefs.inodesFree<5%,imagefs.available<15%,imagefs.inodesFree<5%"
```

Keep the kubelet default `enforce-node-allocatable=pods`. Do not enforce the system or kube reserved cgroups.

## How the numbers were built

Most charts used here ship with `resources: {}`. They set **no default requests or limits**, so chart defaults can't be summed into a budget. The figures below combine:

1. measured numbers from upstream docs where they exist (k3s, Alloy, the Prometheus TSDB),
2. the resource examples that chart authors leave commented out in `values.yaml`, and
3. estimates of steady-state working set for a single-user cluster. These are marked *estimate*. Check them after install with `kubectl top pods -A` and `systemd-cgtop`.

Budgets round upward on purpose. Usable RAM on an "8 GB" VM is a little below 8 GiB after the kernel and firmware reservations. The tables use ~7.7 GiB; check the real value with `free -m` on the server.

## Shared components (both options)

| Component | Budget | Basis |
|---|---|---|
| OS + k3s server (with containerd, Kine/SQLite) + bundled CoreDNS, Traefik, metrics-server, local-path-provisioner | 1.6 GiB | The k3s resource profiling, run on a 4-core/8 GB x86 VM (the same shape as this node), measured **1428 MB** for a server with no workload and **1596 MB** for a server with a workload. Figures came from `kubectl top node`, so they cover the whole node, and they "do not include overhead for system-level management agents or logging". [k3s resource profiling] Bundled pod requests: CoreDNS 70Mi request / 170Mi limit [k3s coredns.yaml], metrics-server 70Mi request [k3s metrics-server], Traefik `resources: {}` [traefik values]. |
| cert-manager (controller, webhook, cainjector) | 150 MiB | *Estimate.* All three default to `resources: {}`. Commented examples are 10m/32Mi each [cert-manager values]. Controller memory grows with the number of cached TLS Secrets [cert-manager scaling], and this cluster has only a few. |
| Argo CD (application-controller, repo-server, server, redis, applicationset; dex and notifications disabled) | 600 MiB | *Estimate.* All components default to `resources: {}`. The chart's commented examples: controller 256Mi request / 512Mi limit, repo-server 64Mi/128Mi, server 64Mi/128Mi, redis 64Mi/128Mi, applicationset 128Mi/128Mi, dex 32Mi/64Mi, notifications 128Mi/128Mi [argo-cd values, chart 10.14.0]. The budget allows for repo-server spikes while it renders Helm or Kustomize. |
| Sealed Secrets controller | 64 MiB | *Estimate.* The chart sets empty `requests`/`limits` [sealed-secrets values]. |
| CloudNativePG operator | 128 MiB | The chart sets `resources: {}`. Its commented example is 100Mi request / 200Mi limit [cloudnative-pg values, chart 0.29.1]. |
| PostgreSQL 18 (one CNPG instance) | 1 GiB | CNPG recommends Guaranteed QoS for Postgres. Its example is a 1024Mi request equal to the limit, with `shared_buffers: 256MB`, and it says "a reasonable starting value for `shared_buffers` is 25% of the memory" [CNPG resource management]. CNPG does not override `shared_buffers` by default [CNPG postgresql_conf]. 512Mi with `shared_buffers: 128MB` would also be enough for tiny traffic. |
| Go API | 128 MiB | *Estimate.* A small Go HTTP service usually runs well under 100 MiB. |
| Other non-root workload (outside Kubernetes) | 1 GiB | Placeholder. Its size is unknown, so measure it and adjust `system-reserved` to match. |
| **Subtotal** | **~4.6 GiB** (4756 MiB) | |

## Option (a): self-hosted kube-prometheus-stack + Loki + Alloy, 14-day retention

| Component | Budget | Basis |
|---|---|---|
| Prometheus | 768 MiB (limit 1 GiB) | Prometheus 2.x needs about 2 GiB per million series for cardinality, plus about 2.5 GiB per million for ingestion at a 15 s scrape, so roughly 4.5 KiB per series. [Robust Perception, a Prometheus maintainer's blog] A default kube-prometheus-stack on one node runs about 50k-80k active series. One third-party single-node test measured 80,163 series at about 430 MB working set [computingforgeeks; secondary, used only as a sanity check]. Retention barely changes RAM because older blocks are memory-mapped, but compaction and 14-day range queries cause spikes. The chart default retention is `10d` [kps values]; the Prometheus default is `15d` [Prometheus storage]. |
| Prometheus operator + config reloader | 100 MiB | *Estimate.* The chart sets `resources: {}`. Commented examples are 100Mi/200Mi (operator) and 50Mi (reloader) [kps values]. |
| kube-state-metrics | 64 MiB | *Estimate.* Small object count. |
| node-exporter | 32 MiB | *Estimate.* |
| Grafana (with dashboard and datasource sidecars) | 384 MiB | Grafana lists a "minimum recommended memory" of 512 MB [Grafana install docs]. A single user with no alert evaluation runs lower. |
| Alertmanager | 64 MiB | *Estimate*, or 0 if disabled. The chart's default retention is `120h` [kps values]. |
| Loki, single binary, filesystem storage, caches disabled | 512 MiB | *Estimate.* Loki only documents sizing for distributed mode; its smallest tier is "less than 3 TB/day" and needs 59 Gi [Loki sizing]. **Chart defaults will not schedule on this node.** The chart defaults to `deploymentMode: SimpleScalable` and enables a memcached chunks cache with `allocatedMemory: 8192`. Memcached pods request and limit `allocatedMemory × 1.2` Mi, so that is about 9.6 GiB for the chunks cache alone, plus about 1.2 GiB for the results cache [loki values + memcached template, chart 7.5.0]. |
| Alloy (DaemonSet, logs to Loki) | 256 MiB | Alloy documents about 120 MiB per 1 MiB/s of logs [Alloy resource estimate]. This node produces far less, so the budget is mostly base runtime. The config-reloader sidecar requests 50Mi [alloy values]. |
| **Monitoring subtotal** | **~2.1 GiB** (2180 MiB) | |
| **Node total** | **~6.8 GiB** | about 0.9 GiB free out of ~7.7 GiB |

Disk is not the constraint here. Prometheus stores 1-2 bytes per sample [Prometheus storage]. At 60k series and a 60 s scrape, 14 days comes to about 1.2-2.4 GB.

## Option (b): Alloy to the Grafana Cloud free tier

| Component | Budget | Basis |
|---|---|---|
| Alloy (one instance: metrics, logs, events) | 320 MiB | Alloy documents about 11 GiB per million active series, or about 11 KiB per series [Alloy resource estimate]. At 10k series that is about 110 MiB, plus logs and base runtime. |
| kube-state-metrics | 64 MiB | As above. |
| node-exporter | 32 MiB | As above. Drop it by using Alloy's built-in `prometheus.exporter.unix`. |
| **Monitoring subtotal** | **~0.4 GiB** (416 MiB) | |
| **Node total** | **~5.1 GiB** | about 2.6 GiB free out of ~7.7 GiB |

Grafana Cloud free tier limits [Grafana pricing]:

- Metrics: "Limited to 10k active series per month", 14 days retention.
- Logs: "Limited to 50 GB ingested per month", 14 day retention. Traces and profiles have the same limits.
- Grafana: "Limited to 3 active users per month".
- Kubernetes Monitoring app: "Limited to 2,232 host hours & 37,944 container hours per month". That is about 3 hosts and 51 containers running all month, which covers this cluster's roughly 25-30 containers.

Billing caveat: usage is `max(active_series, total_dpm / included_dpm)`, measured at the 95th percentile, with 1 DPM per series included [Grafana metrics pricing]. **Scrape every 60 s.** The Prometheus-style 15 s interval quadruples billable series. The pricing page does not document what happens on the free tier when a limit is exceeded, so set up a usage alert.

Trade-offs of (b) to accept:

- Logs and metrics leave the server. Keep secrets and personal data out of logs.
- Monitoring depends on an external service and an internet path. A node outage still shows up, because the data stops arriving, and alerting runs in Grafana Cloud.
- The free tier's limits and terms can change. Option (a) stays available as a fallback, and moving to it means adding a second Alloy output, not a rebuild.

## Reserved resources

The kubelet computes `Allocatable = capacity − kube-reserved − system-reserved − eviction-threshold`. Both reservations are empty by default. The kubelet enforces allocatable on `pods` by default, and the docs warn against enforcing `systemReserved` without exhaustive profiling [k8s reserve compute resources].

- **kube-reserved = 1Gi, 500m CPU.** On k3s the API server, controller manager, scheduler, kubelet, Kine/SQLite and containerd all run inside `k3s.service` ("tracks resource utilization for both K3s and containerd" [k3s resource profiling]). That node-level profile of 1.4-1.6 GB includes the OS and the bundled pods, which leaves roughly 0.8-1.0 GiB for `k3s.service` itself. For comparison, GKE reserves 25% of the first 4 GiB plus 20% of the next 4 GiB (1.8 GiB on an 8 GiB node) even though its control plane runs off the node [GKE node sizes].
- **system-reserved = 1536Mi, 500m CPU.** This covers the OS (sshd, journald, systemd; *estimate* ~0.5 GiB) plus the other non-root workload, because that workload runs outside the kubelet. Change it to 0.5 GiB plus the measured size of that workload.
- **eviction-hard: `memory.available<256Mi`.** The 100Mi default is thin on a node without swap. If you set any hard threshold, the kubelet sets the unset ones to zero instead of keeping their defaults, so list all of them [k8s node-pressure eviction].
- Resulting allocatable is about 7.7 − 1 − 1.5 − 0.25 ≈ **4.9 GiB for pods**. Pod requests come to about 2.3 GiB of shared components (excluding the OS, k3s and the other workload, but including the bundled pods) plus monitoring: about **4.4 GiB with (a)**, which leaves little room, and about **2.7 GiB with (b)**.
- CPU is not a concern. k3s itself used 5-6% of a core in the profile [k3s resource profiling].

## Lean Helm values

### Shared

```yaml
# argo-cd (chart 10.x)
dex: { enabled: false }            # single user, local admin
notifications: { enabled: false }
controller:
  resources: { requests: { memory: 256Mi }, limits: { memory: 512Mi } }
repoServer:
  resources: { requests: { memory: 64Mi }, limits: { memory: 256Mi } }
server:
  resources: { requests: { memory: 64Mi }, limits: { memory: 128Mi } }
redis:
  resources: { requests: { memory: 32Mi }, limits: { memory: 64Mi } }
```

Argo CD Core (`core-install.yaml`) drops the API server, UI auth, RBAC and notifications [Argo CD core]. It saves a little more, at the cost of the full UI.

```yaml
# cert-manager
resources: { requests: { memory: 48Mi }, limits: { memory: 128Mi } }
webhook:  { resources: { requests: { memory: 24Mi }, limits: { memory: 64Mi } } }
cainjector: { resources: { requests: { memory: 32Mi }, limits: { memory: 128Mi } } }

# cloudnative-pg operator
resources: { requests: { memory: 100Mi }, limits: { memory: 200Mi } }
```

```yaml
# CNPG Cluster (Guaranteed QoS: requests == limits for cpu and memory)
spec:
  instances: 1
  postgresql:
    parameters:
      shared_buffers: "256MB"
  resources:
    requests: { cpu: "500m", memory: "1Gi" }
    limits:   { cpu: "500m", memory: "1Gi" }
```

### Option (b), recommended

Use either the Grafana `k8s-monitoring` chart or a plain `alloy` chart with your own config. `k8s-monitoring` deploys the Alloy Operator, Alloy collectors, kube-state-metrics, node-exporter and optional extras (OpenCost, Kepler, Beyla) [k8s-monitoring chart]. Turn off the extras. Keep one metrics collector, one log collector and the events collector. Set `scrape_interval = "60s"` and an allowlist of metrics. Give Alloy roughly 128Mi request and 384Mi limit.

### Option (a), if chosen anyway

```yaml
# kube-prometheus-stack (chart 92.x)
kubeEtcd: { enabled: false }               # k3s embeds these in one process
kubeControllerManager: { enabled: false }
kubeScheduler: { enabled: false }
kubeProxy: { enabled: false }
defaultRules:
  rules: { etcd: false, kubeControllerManager: false, kubeProxy: false,
           kubeSchedulerAlerting: false, kubeSchedulerRecording: false, windows: false }
alertmanager:
  alertmanagerSpec:
    resources: { requests: { memory: 32Mi }, limits: { memory: 64Mi } }
prometheus:
  prometheusSpec:
    scrapeInterval: 60s                      # chart default is 30s
    retention: 14d
    retentionSize: 8GiB
    resources: { requests: { memory: 600Mi }, limits: { memory: 1Gi } }
grafana:
  resources: { requests: { memory: 128Mi }, limits: { memory: 256Mi } }
  sidecar: { resources: { requests: { memory: 48Mi }, limits: { memory: 96Mi } } }
```

```yaml
# loki (chart 7.x): single binary, filesystem, 14-day retention, no caches
deploymentMode: SingleBinary
loki:
  auth_enabled: false
  commonConfig: { replication_factor: 1 }
  storage: { type: filesystem }
  schemaConfig:
    configs:
      - from: "2026-01-01"
        store: tsdb
        object_store: filesystem
        schema: v13
        index: { prefix: index_, period: 24h }   # retention needs a 24h index period
  compactor:
    retention_enabled: true
    delete_request_store: filesystem
  limits_config:
    retention_period: 336h
singleBinary:
  replicas: 1
  resources: { requests: { memory: 256Mi }, limits: { memory: 512Mi } }
read: { replicas: 0 }
write: { replicas: 0 }
backend: { replicas: 0 }
chunksCache: { enabled: false }     # default allocatedMemory 8192 => ~9.6 GiB request
resultsCache: { enabled: false }
gateway: { enabled: false }
lokiCanary: { enabled: false }
test: { enabled: false }
```

The retention settings follow the Loki retention docs [Loki retention]. The Loki install guide notes that a single replica needs `replication_factor: 1` or "requests will fail" [Loki monolithic install].

## Open points

- Measure the other non-root workload and set `system-reserved` from that number.
- After the first install, compare these budgets with `kubectl top pods -A`, `systemd-cgtop`, and `free -m`, then tighten the limits.
- The free tier's behaviour when a limit is exceeded (throttle or drop) is not documented on the pricing pages. Set a usage alert in Grafana Cloud.

## Sources

- [k3s resource profiling]: https://docs.k3s.io/reference/resource-profiling
- k3s requirements: https://docs.k3s.io/installation/requirements
- [k3s coredns.yaml]: https://github.com/k3s-io/k3s/blob/master/manifests/coredns.yaml
- [k3s metrics-server]: https://github.com/k3s-io/k3s/blob/master/manifests/metrics-server/metrics-server-deployment.yaml
- k3s Traefik HelmChart manifest: https://github.com/k3s-io/k3s/blob/master/manifests/traefik.yaml
- [traefik values]: https://github.com/traefik/traefik-helm-chart/blob/master/traefik/values.yaml
- [k8s reserve compute resources]: https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/
- [k8s node-pressure eviction]: https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/
- [GKE node sizes]: https://docs.cloud.google.com/kubernetes-engine/docs/concepts/plan-node-sizes
- [kps values] (kube-prometheus-stack 92.21.2): https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-prometheus-stack/values.yaml
- [Prometheus storage]: https://prometheus.io/docs/prometheus/latest/storage/
- [Robust Perception]: https://www.robustperception.io/how-much-ram-does-prometheus-2-x-need-for-cardinality-and-ingestion/
- [computingforgeeks] (secondary): https://computingforgeeks.com/install-prometheus-grafana-kubernetes/
- [loki values] (chart 7.5.0): https://github.com/grafana/loki/blob/main/production/helm/loki/values.yaml
- memcached template: https://github.com/grafana/loki/blob/main/production/helm/loki/templates/memcached/_memcached-statefulset.tpl
- [Loki sizing]: https://grafana.com/docs/loki/latest/setup/size/
- [Loki retention]: https://grafana.com/docs/loki/latest/operations/storage/retention/
- [Loki monolithic install]: https://grafana.com/docs/loki/latest/setup/install/helm/install-monolithic/
- [alloy values] (chart 1.15.0): https://github.com/grafana/alloy/blob/main/operations/helm/charts/alloy/values.yaml
- [Alloy resource estimate]: https://grafana.com/docs/alloy/latest/introduction/estimate-resource-usage/
- [Grafana install docs]: https://grafana.com/docs/grafana/latest/setup-grafana/installation/
- [Grafana pricing]: https://grafana.com/pricing/
- [Grafana metrics pricing]: https://grafana.com/docs/grafana-cloud/platform/pricing-and-usage/metrics/
- [k8s-monitoring chart]: https://grafana.com/docs/grafana-cloud/monitor-infrastructure/kubernetes-monitoring/configuration/helm-chart-config/helm-chart/
- [argo-cd values] (chart 10.14.0): https://github.com/argoproj/argo-helm/blob/main/charts/argo-cd/values.yaml
- [Argo CD core]: https://argo-cd.readthedocs.io/en/stable/operator-manual/core/
- [cert-manager values]: https://github.com/cert-manager/cert-manager/blob/master/deploy/charts/cert-manager/values.yaml
- [cert-manager scaling]: https://cert-manager.io/docs/devops-tips/scaling-cert-manager/
- [sealed-secrets values]: https://github.com/bitnami-labs/sealed-secrets/blob/main/helm/sealed-secrets/values.yaml
- [cloudnative-pg values] (chart 0.29.1): https://github.com/cloudnative-pg/charts/blob/main/charts/cloudnative-pg/values.yaml
- [CNPG resource management]: https://github.com/cloudnative-pg/cloudnative-pg/blob/main/docs/src/resource_management.md
- [CNPG postgresql_conf]: https://github.com/cloudnative-pg/cloudnative-pg/blob/main/docs/src/postgresql_conf.md
