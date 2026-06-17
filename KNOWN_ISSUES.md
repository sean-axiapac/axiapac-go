# Known Issues

A log of **temporary workarounds and known limitations** that the team must
stay aware of. Each entry should state what it is, why it exists, the
trade-off, and the exact condition + steps to remove it. Delete the entry in
the same commit that removes the workaround.

---

## TEMP: Expired kiosk JWTs are accepted (auth)

- **Status:** 🟠 Temporary — active
- **Introduced:** 2026-06-18 — commit `929569f`
- **Location:** `web/middlewares/authentication.go`

**What:** `parseJwt()` is called with `jwt.WithoutClaimsValidation()`, and the
`exp` check in `Authentication()` is commented out. Any token with a valid
**signature** is accepted even if expired; `nbf`/`iat` are skipped too.
Signature verification itself is unchanged.

**Why:** Older kiosk builds shipped a hardcoded, now-expired JWT. Enforcing
`exp` would lock those devices out of the timesheet API until they are
re-provisioned.

**Security trade-off:** An expired or leaked token can be replayed
indefinitely. Combined with the **shared signing secret** (one
`AXIAPAC_SIGNING_SECRET` across tenants) and the tenant-less kiosk token
(`nameid: 1`), this widens the kiosk auth surface — a single captured token
stays valid forever. See also the host-based tenant resolution note below.

**Remove when:** the entire kiosk fleet is confirmed on **v1.1.4+** (the
Knox-provisioned token carries a real `exp`).

**How to remove:**
1. Delete `jwt.WithoutClaimsValidation()` from `parseJwt()` (restores default
   `exp`/`nbf`/`iat` validation).
2. Uncomment the `exp` check block in `Authentication()`.
3. Remove this entry.

> Clean revert: this was committed in isolation, so `git revert 929569f`
> undoes the code change by itself.

---

## Host-header tenant resolution is client-trusted (auth/multi-tenancy)

- **Status:** 🟡 Known limitation — by design, hardening pending
- **Location:** `oktedi/web/common/handler.go` (`GetHostname`), all kiosk
  handlers via `DatabaseManager.GetDB`

**What:** The tenant schema is derived from the request `Host` header. There is
no allowlist of provisioned tenants and no binding between the authenticated
token and the tenant, so a caller with any valid kiosk token could target
another tenant's schema by changing `Host`.

**Why it stands:** This is the platform-wide tenant model (the `timesheet`
handlers and the .NET/Go backends all resolve tenant from host). Changing it is
a platform decision, not a per-handler fix.

**Hardening options (not yet done):**
1. Validate the derived schema against `DatabaseManager.GetAllDatabases()` and
   reject unknown hosts (`400/404`) before any `USE`. Small, blocks unprovisioned
   /system schemas; does **not** stop cross-tenant access with a valid token.
2. Tenant-scoped auth: per-tenant signing keys (or a validated tenant claim),
   so a token only authorizes its own tenant. Closes this **and** the expired-JWT
   item above. Platform-level change spanning token minting + Knox provisioning.
