# Agent guidance — cw-alert-router

A small Go AWS Lambda that routes CloudWatch alarm state changes
(EventBridge → SQS) to Slack and PagerDuty, with a CloudWatch-rendered metric
graph. See [README.md](README.md) for the user-facing docs; this file covers
what you need to work on the code.

## Build and test

```sh
go test ./...        # full suite, no AWS credentials or network needed
go vet ./...
gofmt -l .           # CI fails on unformatted files
task build           # docker image + function.zip (arm64, provided.al2023)
task build-local     # cross-compile without docker
```

CI (GitHub Actions) runs vet, the gofmt check, tests and the build on every
push — keep all four green.

## Layout

| Package | Role |
|:--|:--|
| `cw/` | Plain structs mirroring the raw EventBridge alarm event + CloudWatch client (tags, `GetMetricWidgetImage`) |
| `lambda/` | Handler wiring, config from env, SQS batch processing |
| `pagerduty/`, `slack/`, `parameterstore/`, `s3/` | One thin client each |
| `test/` | Shared mocks and fixtures (`MockCWAPI`, `MockSSMClient`, `SlackServer`, `GenTestSQSEvent`, ...) |
| `integration_test/` | End-to-end handler run against all mocks via the env-var config path |
| `examples/terraform/` | Complete reference deployment |

## Conventions

- **Module path is `github.com/tidal-music/cw-alert-router/v2`** — releases
  are `v2.x.y` git tags; a major bump requires changing the module path
  suffix everywhere.
- **AWS SDK for Go v2 only.** Each package defines the small interface it
  consumes (e.g. `cw.API`, `s3.API`) and the mocks in `test/` implement them.
  Don't introduce SDK v1 or generated mock frameworks.
- **Every `.go` file carries the Apache 2.0 license header** (copy it from
  any existing file).
- **Dependencies stay light** — this compiles to a static Lambda `bootstrap`
  binary (`CGO_ENABLED=0`, `-tags lambda.norpc`). Question any new module.
- Config comes from environment variables declared in `lambda/config.go`;
  if you add or change one, update the tables in README.md in the same
  change. Library-only knobs (no env var) live on `lambda.Config`.
- Structured logging via `log/slog` (JSON handler) — no other log libraries.

## Behaviour that looks odd but is deliberate

- `alerts:suppress_pagerduty` suppresses **only PagerDuty**; Slack always
  gets the message.
- SQS batch processing **stops at the first failure** and reports the failed
  record plus every remaining record as batch item failures. This preserves
  FIFO queue ordering on redelivery — don't "optimise" it back to
  per-record independent processing.
- The PagerDuty dedup key is the **alarm ARN**. Changing it breaks
  auto-resolution of incidents that were triggered before a deploy.
- Slack graph upload retries on `invalid_blocks`/`file_not_found` (file
  propagation race) and then falls back to posting without the graph — an
  alert must never be lost because a graph failed.
- Only `OK -> ALARM` and `ALARM -> OK` transitions are routed; everything
  else (e.g. from/to `INSUFFICIENT_DATA`) is intentionally ignored.

## Testing patterns

Tests inject mocks through the `lambda.With*` options — see
`lambda/handler_test.go` for the fixture pattern and
`integration_test/main_test.go` for the full env-var startup path. The fake
Slack server (`test.SlackServer`) implements `chat.postMessage` and the
`files.uploadV2` flow; extend it rather than stubbing the Slack client if
you touch message sending.
