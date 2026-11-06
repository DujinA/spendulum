# Sealed Secrets with automatic key renewal turned off and one off-server key backup

Secrets are committed to the repo as Sealed Secrets. The controller runs with `keyrenewperiod: 0`, so there is a single sealing key, backed up once to the owner's password manager, and restored before the controller starts when the cluster is rebuilt. The default 30-day renewal would add a new key each month that also needs an off-server backup; that chore is easy to forget, and losing an unbacked key makes every secret sealed with it unreadable. Renewal also never re-encrypts existing secrets, so for one user it adds little.

## Consequences

- If a key leak is suspected, rotate by hand: create a new key, re-seal all secrets, back the new key up.
