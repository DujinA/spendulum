# Off-server backup storage for the database

Research for issue #9. Researched on 2026-11-06. Prices are as published on that date.

## Question

Where should the database's off-server backups go? The database is one small PostgreSQL 18 instance (well under 1 GB), managed by CloudNativePG with the barman-cloud plugin. It uses continuous WAL archiving plus nightly base backups, runs on a Hetzner Cloud server in Nuremberg, and gets a monthly restore drill.

Candidates: Hetzner Object Storage, Backblaze B2, Cloudflare R2.

## Recommendation

**Use Backblaze B2 in the EU Central region (Amsterdam).** Create a bucket with Object Lock enabled, set a default retention, and add a lifecycle rule for hidden versions.

- At our size it costs nothing. The first 10 GB of storage is free, API classes A to C are free, and a restore drill fits inside the free egress allowance of 3x the stored data.
- It is a different company, account and country from the database server. A billing problem, account lockout or termination at Hetzner cannot take the backups with it. Hetzner deletes all data on contract termination, even locked data.
- It supports real S3 Object Lock (governance and compliance modes) and keeps old versions by default. That protects backups from deletion if the cluster's credentials are compromised.
- New B2 buckets encrypt at rest by default (SSE-B2, AES-256) at no extra charge. This works without any barman-cloud feature that is still unreleased.

The runner-up is Hetzner Object Storage in a *different* location (Falkenstein or Helsinki, not Nuremberg). Choose it only if keeping a single vendor matters more than about €78 a year. Wait for a plugin release that includes SSE-C support before relying on it for encryption.

Cloudflare R2 is not recommended. It has no S3 versioning or S3 Object Lock, and an open plugin issue reports failed restores against R2.

## Comparison

| | Hetzner Object Storage | Backblaze B2 | Cloudflare R2 |
|---|---|---|---|
| Monthly cost at under 1 GB | €6.49 base fee (excl. VAT), always charged while any bucket exists | $0 (first 10 GB free) | $0 (10 GB-month free on Standard) |
| Price beyond free or included | Base fee covers 1 TB storage and 1 TB egress; then €0.0087 per TB-hour storage | $6.95 per TB per month | $0.015 per GB-month |
| Minimums | 64 KB minimum billable object size; base fee charged even for an empty bucket | No minimum file size and no minimum storage duration | None on Standard (Infrequent Access has a 30-day minimum) |
| API request cost | Free | Classes A, B, C free; class D $0.004 per 10,000 after 2,500 free per day | Class A $4.50 per million, class B $0.36 per million, after 1M / 10M free per month |
| Egress for a restore drill | Included (1 TB per month) | Free up to 3x average stored data, then $0.01 per GB | Free |
| EU location | Falkenstein, Nuremberg, Helsinki (one data center per bucket) | EU Central, Amsterdam (chosen per account, at sign-up) | `eu` jurisdiction (guaranteed); location hints are best effort only |
| Encryption at rest | Not by default. SSE-C only | On by default for new buckets (SSE-B2, AES-256), free. SSE-C also available | Always on (AES-256, Cloudflare-managed keys). S3 `x-amz-server-side-encryption` header not implemented |
| Object Lock | Yes (governance and compliance); must be enabled at bucket creation | Yes (governance and compliance), 1 to 3,000 days; no extra cost | No S3 Object Lock. Cloudflare-specific "bucket locks" instead |
| Versioning | Yes (enabled automatically with Object Lock) | Yes; all versions kept by default | No (`PutBucketVersioning` not implemented) |
| Independent of Hetzner account | No | Yes | Yes |
| barman-cloud plugin fit | Works as S3-compatible. Encryption needs SSE-C, which is merged but not yet in a release (latest is v0.15.1) | Works as a standard S3-compatible endpoint | Open issue reporting restore failures |

### Cost notes

- WAL volume, not database size, drives storage. CloudNativePG sets `archive_timeout = '5min'`, so even a quiet database ships a 16 MB WAL segment up to every five minutes when there is any activity. How much is kept depends on the retention policy. Even at 30 GB stored, B2 would cost about $0.14 a month above the free 10 GB, and R2 about $0.30. Hetzner stays at the flat €6.49.
- A monthly restore drill downloads one base backup plus the WAL since then, about the size of the stored set. That is inside the free egress of all three.

## Protection setup for B2 (for the implementation ticket)

barman-cloud never modifies uploaded files. The plugin docs recommend versioning, object locking and lifecycle rules to protect them. On B2:

1. Create the bucket in an EU Central account with Object Lock enabled, and set a default retention. Governance mode allows recovery from mistakes with a special key. Compliance mode cannot be shortened by anyone. Pick a period a little shorter than the barman retention policy, for example 14 days of lock against a 30-day recovery window.
2. When barman-cloud's retention deletes old backups, B2 hides them instead of removing them. Add a lifecycle rule that deletes hidden versions after a few days. Locked versions stay until their lock expires, and lifecycle cannot delete them before that.
3. Give the cluster an application key limited to that one bucket, without the capability to bypass governance retention.
4. Leave `encryption` unset in the `ObjectStore`, because SSE-B2 is the bucket default. The endpoint follows the pattern `https://s3.<region>.backblazeb2.com`; the console shows the exact region on the bucket page.
5. If uploads fail with an `x-amz-content-sha256` error, set `AWS_REQUEST_CHECKSUM_CALCULATION` and `AWS_RESPONSE_CHECKSUM_VALIDATION` to `when_required` in `instanceSidecarConfiguration.env`, as the plugin docs describe.

## Why not the others

**Hetzner Object Storage.** It is technically capable: Object Lock, versioning and an EU location close to the server. But it has three drawbacks for this use:

- It is the same provider and account as the database server. Hetzner states that on contract termination it permanently deletes all stored data, "even if you had activated Object Locking or Retention." Off-server is not the same as off-provider.
- There is no default encryption at rest, and Hetzner lists data-at-rest encryption as the client's responsibility. The only server-side option is SSE-C. The barman-cloud plugin added SSE-C (`sseCustomerKey`) on its main branch in October 2026, but the latest release (v0.15.1, 2026-09-30) does not include it. Once released, the key must also be stored safely outside the bucket, or the backups are unrecoverable.
- It has a fixed €6.49 per month base fee. That is reasonable for a 1 TB allowance, but we use a tiny fraction of it.

If it is used anyway, put the bucket in Falkenstein or Helsinki rather than Nuremberg, so a single-site incident does not hit both copies.

**Cloudflare R2.** It is free at this size, has a guaranteed EU jurisdiction and always encrypts at rest. However:

- The S3 API does not implement versioning or Object Lock. R2 has its own "bucket lock" rules, but they are a Cloudflare-specific mechanism outside the S3 semantics barman-cloud and the plugin docs assume.
- Plugin issue #411, "plugin-barman-cloud does not work with r2 S3 compatible storage", is still open. It reports that backups are written but restore fails. For a setup whose point is a monthly restore drill, that is disqualifying until resolved.

## Sources

Hetzner Object Storage
- Pricing and features: https://www.hetzner.com/storage/object-storage/
- Overview, endpoints, limits: https://docs.hetzner.com/storage/object-storage/overview/
- General FAQ (no default at-rest encryption, SSE-C, single data center, no cross-location replication): https://docs.hetzner.com/storage/object-storage/faq/general/
- Buckets and objects FAQ (Object Lock at creation; deletion on contract termination even with Object Lock): https://docs.hetzner.com/storage/object-storage/faq/buckets-objects/
- Supported S3 actions (encryption: SSE-C only): https://docs.hetzner.com/storage/object-storage/supported-actions/
- Object Lock retention guide: https://docs.hetzner.com/storage/object-storage/howto-protect-objects/protect-object-lock-retention/
- Technical and organizational measures (at-rest encryption is the client's responsibility): https://docs.hetzner.com/general/others/technical-and-organizational-measures

Backblaze B2
- Pricing (storage, 10 GB free, egress 3x free, transaction classes, no minimums): https://www.backblaze.com/cloud-storage/pricing
- Data regions (EU Central is Amsterdam): https://www.backblaze.com/docs/cloud-storage-data-regions
- S3-compatible API introduction (endpoint format): https://www.backblaze.com/apidocs/introduction-to-the-s3-compatible-api
- Object Lock: https://www.backblaze.com/docs/cloud-storage-object-lock
- Default encryption for new uploads from 2026-09-14 (SSE-B2, no charge): https://www.backblaze.com/blog/backblaze-b2-to-encrypt-new-uploads-by-default/
- Server-side encryption: https://www.backblaze.com/docs/cloud-storage-server-side-encryption
- Lifecycle rules (all versions kept by default): https://www.backblaze.com/docs/cloud-storage-lifecycle-rules

Cloudflare R2
- Pricing: https://developers.cloudflare.com/r2/pricing/
- S3 API compatibility (no versioning, no Object Lock, no `x-amz-server-side-encryption`, SSE-C supported): https://developers.cloudflare.com/r2/api/s3/api/
- Data location (`eu` jurisdiction guaranteed, hints best effort): https://developers.cloudflare.com/r2/reference/data-location/
- Data security (AES-256 at rest): https://developers.cloudflare.com/r2/reference/data-security/
- Bucket locks: https://developers.cloudflare.com/r2/buckets/bucket-locks/

CloudNativePG and barman-cloud plugin
- Object store docs (S3-compatible endpoints, checksum workaround, versioning and locking advice): https://cloudnative-pg.io/plugin-barman-cloud/docs/object_stores/
- SSE-C docs on the main branch (not yet released): https://github.com/cloudnative-pg/plugin-barman-cloud/blob/main/web/docs/object_stores.md
- SSE-C feature request, closed as completed 2026-10-27: https://github.com/cloudnative-pg/plugin-barman-cloud/issues/646
- Releases (latest v0.15.1, 2026-09-30): https://github.com/cloudnative-pg/plugin-barman-cloud/releases
- R2 restore issue (open): https://github.com/cloudnative-pg/plugin-barman-cloud/issues/411
- Checksum error issue: https://github.com/cloudnative-pg/plugin-barman-cloud/issues/393
- CloudNativePG default `archive_timeout = '5min'`: https://github.com/cloudnative-pg/cloudnative-pg/blob/main/docs/src/postgresql_conf.md

## Not verified

- Whether B2 accepts the newer boto3 default checksum headers without the `when_required` workaround. Try it during setup.
- The exact B2 behavior when barman-cloud's `DeleteObjects` hits a locked object version. Check it during the first retention run after setup.
