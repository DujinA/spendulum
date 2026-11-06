# Host Caddy is the public edge; k3s Traefik and servicelb are disabled

The server already runs Caddy on ports 80 and 443, with CrowdSec reading its logs and banning IPs through an iptables bouncer. Instead of letting k3s's bundled Traefik take those ports, we keep Caddy as the only public entry point: it terminates TLS with its own automatic Let's Encrypt certificate for `spendulum.adujin.com` and proxies to the app's `NodePort` on `127.0.0.1`. k3s starts with `disable: [traefik, servicelb]`. This keeps CrowdSec seeing every real client IP with no extra wiring, and avoids a second proxy for an app with one route.

## Considered Options

- **Traefik as the edge with Gateway API and cert-manager** (the original plan, and what the k3s compatibility research configured): requires removing Caddy and blinds CrowdSec unless it is wired into Traefik.
- **Caddy in front of Traefik on a NodePort**: keeps Gateway API routing in Git but adds a second proxy. This remains the upgrade path if Gateway API practice is wanted later; nothing public changes.

## Consequences

- Public routing and TLS live in `deploy/host/caddy/spendulum.caddy`, imported by the host's Caddyfile and applied by the host script, not by Argo CD.
- cert-manager is still installed, because the CloudNativePG barman-cloud plugin needs it, but it issues no public certificate.
- kube-proxy runs with `nodeport-addresses=127.0.0.0/8`; the Hetzner Cloud Firewall also blocks the NodePort range.
- The API trusts `X-Forwarded-For` only from Caddy.
- CrowdSec must allowlist the pod range (10.42.0.0/16) and service range (10.43.0.0/16).
- The DNS record is DNS-only on Cloudflare (not proxied), so Caddy can complete HTTP-01 and sees real client IPs.
