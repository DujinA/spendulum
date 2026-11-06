# CloudNativePG with barman-cloud backups to Backblaze B2, and a restore drill before real data

Postgres runs as a single-instance CloudNativePG Cluster, using the barman-cloud plugin (in-tree Barman is removed in CloudNativePG 1.32), instead of Postgres directly on the host with hand-written backup scripts. WAL is archived continuously (`archive_timeout` 5 min, so at most 5 minutes of entries can be lost) and a base backup runs daily at 03:00 Europe/Belgrade, with a 30-day barman retention policy. Backups go to Backblaze B2 in EU Central, a different provider from Hetzner, in a bucket with Object Lock in governance mode (14-day default retention), a lifecycle rule that removes hidden versions after 3 days, and an application key limited to that bucket that cannot bypass governance.

No real data enters production, including the sheet history upload, until a first restore drill has passed: recover into a scratch namespace with `bootstrap.recovery`, compare row counts per table with production, then delete the scratch namespace. After that the drill is run monthly by hand from a runbook.

## Consequences

- One instance means `enablePDB: false`, or node drains hang.
- The StorageClass uses `reclaimPolicy: Retain` and the Cluster carries `Prune=false`, because local-path deletes data with the PVC by default.
- Governance mode (not compliance) keeps a way out of a mistaken lock.
- Automating the drill as a CronJob waits until after v1.
