# oktedi-api

Go backend (Gin) for the OKTEDI timesheet kiosk and related tooling. Module
`axiapac.com/axiapac`, Go 1.25.1. The HTTP server lives in `oktedi/web` and
listens on `0.0.0.0:8090`.

## Repository layout

| Path | What it is |
|---|---|
| `oktedi/web` | The kiosk HTTP API server (`make oktedi …`). |
| `oktedi/web/handlers`, `oktedi/web/handlers/timesheet` | Endpoint handlers. |
| `oktedi/web/common` | `GetHostname` + the per-request `GetDB` tenant helper. |
| `core` | `DatabaseManager` (connection pool + `USE <schema>` switching). |
| `lambdas/axiapac-reply-email-handler`, `lambdas/sync-calendar` | AWS Lambdas. |

## Deploy

All builds go through the **root `makefile`**, which is a dispatcher:

```
make <module> <task...>
```

The first word is the module, the rest are tasks forwarded to that module's
`makefile.mk`. Running `make` with no arguments prints a usage hint.

### Deploy the kiosk API server (`oktedi`)

```bash
# Full deploy: clean → build server + client → upload to S3
make oktedi deploy
```

`make oktedi deploy` runs `clean build upload`, where:

| Task | Command | Effect |
|---|---|---|
| `make oktedi clean` | — | `rm -rf ./oktedi/dist` |
| `make oktedi build-server` | `GOOS=linux GOARCH=amd64 go build` | Linux/amd64 binary → `oktedi/dist/server` |
| `make oktedi build-client` | `cd ../oktedi-web && pnpm run build` | Builds the web frontend from the **sibling `oktedi-web` repo** |
| `make oktedi build` | — | `clean` + `build-server` + `build-client` |
| `make oktedi upload` | `aws s3 sync ./oktedi/dist/ s3://axiapac-development/oktedi/ --delete` | Publishes artifacts to S3 (`--delete` mirrors, removing stale objects) |
| `make oktedi deploy` | — | `clean` + `build` + `upload` |

Prerequisites:

- Go 1.25.1 toolchain.
- `pnpm` and the `oktedi-web` repo checked out **as a sibling directory**
  (`../oktedi-web`) — required by `build-client`. To build/upload only the
  server, run the server tasks explicitly: `make oktedi clean build-server upload`.
- AWS credentials with write access to `s3://axiapac-development/oktedi/`
  (`aws sts get-caller-identity` to confirm you're on the right account).

> The makefile's deploy boundary is the S3 upload. How the published binary is
> rolled out onto the running host is handled outside this repo — confirm the
> current rollout step before relying on `upload` alone to take effect.

### Deploy a Lambda

Both lambdas build an arm64 `bootstrap`, zip it, and push it with
`aws lambda update-function-code`:

```bash
make axiapac-reply-email-handler deploy
make sync-calendar deploy
```

Per-lambda tasks: `build`, `zip`, `upload`, `deploy` (= `clean build zip upload`).
`sync-calendar` also has `make sync-calendar run` for a local invoke.

## Run locally

```bash
DSN='user:pass@tcp(host:3306)/' \
AWS_REGION=ap-southeast-2 \
AXIAPAC_SIGNING_SECRET='<base64-encoded HMAC secret>' \
go run ./oktedi/web
```

Required environment variables (read in `oktedi/web/main.go`):

| Var | Purpose |
|---|---|
| `DSN` | MySQL DSN **without** a schema/db name — the schema is selected per request (see below). |
| `AWS_REGION` | AWS region for SDK calls. |
| `AXIAPAC_SIGNING_SECRET` | Base64-encoded HMAC secret used to validate the kiosk JWT. |

## Multi-tenancy (how the schema is chosen)

Each request's tenant **schema is derived from the `Host` header**:
`GetHostname(c.Request.Host)` strips the port, and `DatabaseManager.GetDB`
takes the first DNS label and runs `USE <schema>` on a dedicated connection —
e.g. `oktedi.axiapac.net.au` → schema `oktedi`. A `Host` of `localhost`
(local dev) falls back to the schema embedded in `DSN`.

All kiosk handlers resolve the tenant this way (consistent with the
`timesheet` handlers); none hardcode a schema.
