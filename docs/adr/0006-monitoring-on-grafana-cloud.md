# Metrics and logs go to the Grafana Cloud free tier, not a self-hosted stack

Grafana Alloy in the cluster ships metrics and logs to the Grafana Cloud free tier instead of running kube-prometheus-stack and Loki on the node. On the 8 GB server the self-hosted stack would leave under 1 GiB of headroom (and the Loki chart's defaults would not schedule at all), while Alloy alone adds about 0.4 GiB. The free tier (10k active series, 50 GB logs per month, 14-day retention) is enough if metrics are scraped every 60s through an allowlist.

The external uptime check is Grafana Cloud Synthetic Monitoring probing `https://spendulum.adujin.com/healthz`, so all alerts live in one place. Instrumentation is OpenTelemetry, so bringing storage home later changes only Alloy's destination.
