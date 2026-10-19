# API and integration

All paths below are relative to the app origin/subpath. GET /healthz is public health only. GET /api/bootstrap returns non-secret configuration. GET /api/me returns the signed-in account and session CSRF token. Writes require an exact Origin header plus X-CSRF-Token from that session; use same-origin credentials and never log tokens. Owner-only POST /api/backups streams the encrypted archive. Login/OIDC/session endpoints are provided by the public version-pinned One Workspace apphost package.

Owner only: GET/POST /api/checks; PUT/DELETE /api/checks/{id}; POST /api/checks/{id}/run; GET /api/samples/{id}; GET /api/incidents; PUT /api/incidents/{id}/ack; GET /api/overview; PUT /api/settings. GET /status is 404 until publication is enabled and then exposes only opted-in names/states.

User resources are authorised on every request. Missing/inaccessible user records do not expose private titles. Limits and filters use bounded input; unsupported operations fail explicitly. No API grants direct database access to another app. Managed MyAccounts credentials are server-side only. Search and Drive integrations verify that the source's current immutable subject matches the requesting app, and recheck identity after search. Source permissions apply again when opening a link.
