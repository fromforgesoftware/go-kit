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
application (repository/usecase) · resource · errors · filter · search ·
transport (rest/grpc/websocket/tcp/udp/amqp/nats) · jsonapi · openapi ·
persistence (postgres/gormdb/sqldb/redisdb) · migrator ·
auth (jwt/oidc/password/provider) · firebase · monitoring (logger/tracer) ·
decimal · retry · saga · instance · ptr · slicesx · golden · fixtures · sops ·
proto (generated under `proto/`) · pb. See README.md for one line per package.

Only add a package when a consumer needs it now — no speculative packages.

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
