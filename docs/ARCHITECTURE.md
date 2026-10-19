# ADR003: My Monitor

Decision: original Apache-2.0 app at oneworkspacestudio/my-monitor. One supervisor
entry per host. A new documented loopback-only, separately scoped registry API in
One Workspace returns discovered app health targets, maintenance/update and backup
metadata. Its per-app capability is server-only, never an owner password. No hardcoded
port inventory, network scan or DB reads. SQLite stores check samples, incidents,
maintenance windows and notification delivery state. Windows APIs provide local
CPU/RAM/disk measurements. Only the configured Workspace owner sees infrastructure;
public aggregate status is disabled by default and never shows URLs, paths or logs.
Registered local targets are exact allowlisted checks. Optional custom HTTP/TCP
checks require owner action; resolve and pin every IP, reject metadata/link-local
and private networks outside explicit policy, disable redirects and proxy env.
Webhooks are explicit opt-in, SSRF checked and protected at rest. No email route
is advertised without a supported user-mail notification API. Domain-expiry checks
are omitted until a verified RDAP connector is provided.

Shared decision: native Go server, embedded AUI-generated React/TypeScript UI,
SQLite (modernc, BSD-3-Clause), protected per-app NTFS data folders and one EXE.
Reuse the existing One Workspace Apache-2.0 platform, OIDC, branding and supervisor
conventions through a public pkg/apphost package in the actual suite repository.
Canonicalise the suite Go module path to its existing organization repository;
there are no supported external consumers of its internal packages. Each original
app has a dedicated organization repository, rather than a misleading fork.
OIDC uses code+PKCE, state cookie, nonce, issuer/audience verification and immutable
issuer/subject keys. Session/CSRF secrets stay on the server. Local bootstrap is
loopback-only and explicitly disabled in managed mode. No GitHub Actions: user
instruction overrides the brief's Actions-template suggestion; run gates locally.
