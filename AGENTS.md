# go-kit — Forge shared Go platform library

The canonical Go library every forge service consumes via
`github.com/fromforgesoftware/go-kit/...`. Split from the forge monorepo;
versioned + published independently. This is the BASE module — no forge deps.

## Commands
- Build: `go build ./...`
- Test: `go test ./...` (race: `go test -race ./...`)
- Lint: `golangci-lint run`
- Regenerate platform protobuf: `buf generate` (sources in `proto/`)

## Stack
Go 1.25 · gRPC + JSON:API · GORM/Postgres + Redis · OpenTelemetry · buf.

## Capabilities (packages)
transport (rest/grpc/ws/amqp/nats) · persistence (postgres/gormdb/redisdb) ·
auth (jwt/oidc/password) · jsonapi · resource · search · audit (Event + Sink port) ·
outbox · monitoring (logger/tracer) · app (fx bootstrap) · errors · validation ·
config · ratelimit · idempotency · proto (generated under `proto/`).

## Conventions
- Commits: `<type>(<scope>): <subject>` — one line, ≤72 chars, lowercase, no
  body/footer, no Co-Authored-By trailer.
- Keep domain-agnostic: no service/app-specific code. Game/sim packages were
  evicted to a separate game-kit and must not return here.

## Boundaries
- NEVER hand-edit generated `proto/**/*.pb.go` — edit `.proto` + `buf generate`.
  (A blind module-path sed corrupts proto `rawDesc` descriptors → runtime panics.)
- NEVER commit secrets. Don't add dependabot.
- Breaking changes ripple to every consumer — bump versions deliberately.
