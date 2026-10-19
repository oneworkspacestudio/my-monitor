# My Monitor: operation and recovery

## Storage and launch

Windows 10/11 x64, NTFS private folder, writable dedicated program folder. One main `MyMonitor.exe` embeds the API and compiled web UI. No separate Node/Python/database server is needed. Defaults: loopback listen only; dedicated data folder beside the executable if no --data was supplied. Prefer an explicit separate data folder. Do not put private data in a publicly served web folder or a synchronisation folder. Reparse points and drive roots are rejected. One instance locks its data folder. Stop before moving it.

Data includes app.json schema marker, app.sqlite with WAL, audit/application logs, and immutable objects where applicable. Monitor additionally has an encrypted-secret key. Back up the whole app, not SQLite alone. Managed state is under Workspace Private/monitor; apps and ordinary Drive files remain at the selected separate roots.

## Managed identity and proxy

One Workspace configures issuer, client, owner, loopback listener, public origin with /monitor subpath and callback. Authorization Code + PKCE, state and nonce are verified; issuer/subject, not email, identify a person. Managed mode disables standalone passwords. Existing Workspace sign-in is reused. My Vault secrets never participate in this app. Configure a separate OIDC client and exact callback for standalone managed deployments. WORKSPACE_OIDC_ISSUER, WORKSPACE_OIDC_CLIENT_ID, WORKSPACE_OIDC_CLIENT_SECRET, WORKSPACE_OWNER_SUBJECT and WORKSPACE_HOME_URL are the supported environment contract (see apphost FromEnv for exact current mapping). Never expose client secrets in browser code. Set --origin to the public HTTPS URL including subpath; the upstream proxy must preserve the approved Host and route. Arbitrary forwarded headers are not trusted. Local development permits HTTP loopback.

## Backup

Owner Settings > encrypted backup uses a consistent SQLite snapshot plus objects and required app keys, protected by Argon2id and authenticated chunked AES-256-GCM. Keep the backup password separately; no backdoor can recover it. Workspace scheduled/transactional backup also covers the app. It is off by default until configured by the owner.

For CLI backup, stop the standalone app first, set WORKSPACE_BACKUP_PASSWORD in this process and run:

```powershell
.\MyMonitor.exe --data C:\WorkspaceData\my-monitor --backup D:\Backups\my-monitor.owb
```

Choose a new output file. Restore into a **new, absent NTFS folder**, preserving the current folder for rollback:

```powershell
.\MyMonitor.exe --data C:\WorkspaceData\my-monitor-restored --restore D:\Backups\my-monitor.owb
```

Restored sessions are removed, objects and database integrity are checked, and ACL protection is reapplied. Restart with the original identity configuration. Managed issuer/subject must remain the same; changing issuer creates another identity. Test restoration before relying on a backup. Managed suite recovery must use the suite recovery workflow to keep MyAccounts and all app state consistent.

## Update / removal / errors

Use Workspace Updates for managed installs. It verifies organization release hashes, backs up, stops children, replaces packages, restarts and checks health. Explicit opt-outs are retained. After stopping Workspace, `Workspace.exe --data <Private> remove-app monitor` removes its catalog/executable only; data, clients and backups remain. Re-enable with `install-apps --bundle <verified suite ZIP> --apps monitor`. Never delete the private folder as part of an update.

401: reopen from Workspace to reconnect. 403: verify account/role and exact origin; do not disable checks. NTFS error: choose a protected NTFS folder before starting. Port occupied: select a free loopback port; managed install allocates offsets. Corrupt/malformed PDF jobs preserve originals and explain resource/password limits. Inspect app.log for structured server diagnostics; do not post private databases, tokens or backups in issues. Logs/audit records currently need owner retention management.

## Current product limits

Infrastructure APIs are server-owner only. Uptime is the percentage of successful samples over 24 hours, not time-weighted SLA availability. Domain registration expiry and email alerts are not implemented. No remote agent or private-network scanning is enabled. Custom endpoints must resolve only to public addresses; exact registry endpoints are trusted separately. Status publishing is off by default.
