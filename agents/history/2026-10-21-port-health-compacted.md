# Managed MyAccounts health repair

User requested correction of One Workspace port-conflict installation failures
and the false MyAccounts unhealthy state. Direct diagnosis showed numeric
127.0.0.1 Host returns403, while localhost health returns200 from the same healthy
service. The supervisor already probes localhost; Monitor used the numeric Host.

Managed probes now use localhost:<port> as HTTP Host only for exact URLs trusted
by the loopback registry. DNS resolution, connection pinning, arbitrary user
checks, no-redirect and SSRF restrictions remain. Real HTTP status still determines
health. Regressions cover a host-validating server, user checks, unregistered local
targets and existing SSRF/readiness/incident behavior. Go tests and vet passed.

Preparing preview3. Build in a clean detached worktree, preserving the pre-existing
untracked internal/app/linux.go in the source checkout. No UI changes. Actions
remain disabled. Task artifacts I:/Projects/tooling/workspace-ports-20261021;
synthetic NTFS data C:/Projects/.cache/workspace-ports-20261021. Never owner fixtures.
Native install and deployment acceptance will be recorded after verification.

Use per-process GIT_CONFIG_GLOBAL=NUL/GIT_CONFIG_NOSYSTEM=1 with this exact safe
directory and explicit author identity to avoid stalled unavailable-drive entries.
