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

Released preview3. Built in a clean detached worktree, preserving the pre-existing
untracked internal/app/linux.go in the source checkout. No UI changes. Actions
remain disabled. Task artifacts I:/Projects/tooling/workspace-ports-20261021;
synthetic NTFS data C:/Projects/.cache/workspace-ports-20261021. Never owner fixtures.
Native installation and deployment acceptance completed; see verified delivery.

Use per-process GIT_CONFIG_GLOBAL=NUL/GIT_CONFIG_NOSYSTEM=1 with this exact safe
directory and explicit author identity to avoid stalled unavailable-drive entries.

## Verified delivery

Monitor3 (2d233c6) and Workspace58 (6245915) are public; all uploaded asset sizes
and GitHub SHA256 digests match local packages. Native synthetic occupied-port
fresh setup selected36032 instead of36000, preserved the occupied listener,
created matching local origins, completed OIDC and showed MyAccounts UP with a
real HTTP200 sample. Browser errors were empty; test services stopped. The
numeric MyAccounts Host remains rejected as intended; its host protection was
not weakened. Screenshot is in the task artifacts.

Normal backed-up local57-to58 update completed, changing only the manager and
Monitor. All25 apps retained, all24 services healthy, Chat9 and other24 app
contracts/packages, identity/storage paths, selections and ports preserved.
Monitor settings and previous samples retained; the earlier false down state
now records UP/HTTP200. Newest rollback manifest verified. Privacy-filtered
deployment/native/publication receipts are in docs/evidence/ports-health-*.json.
Prior failed samples remain historical evidence; no fabricated uptime reset.

No Actions, clean-host/reboot or all-feature acceptance claim. Release tags/assets
are immutable. Source checkout's pre-existing Linux file remains untouched.
Task cleanup and installer copy are recorded separately in the task artifacts.
