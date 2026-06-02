# `go/kit/i18n`

[![CI](https://github.com/fromforgesoftware/forge/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/fromforgesoftware/forge/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/fromforgesoftware/go-kit/i18n.svg)](https://pkg.go.dev/github.com/fromforgesoftware/go-kit/i18n)
[![Module version](https://img.shields.io/github/v/tag/fromforgesoftware/forge?filter=go/kit/v*&label=go/kit&sort=semver&color=blue)](https://github.com/fromforgesoftware/forge/tags?q=go%2Fkit%2F)
[![License: MIT](https://img.shields.io/github/license/fromforgesoftware/forge)](../../LICENSE)

Framework-agnostic Go translation engine. The TypeScript twin lives at
[`libs/ts-i18n`](../../libs/ts-i18n) with an identical API surface so
the same key constants, fallback semantics, and `{{name}}`
interpolation travel round-trip between services and frontends.

## Install

```sh
go get github.com/fromforgesoftware/go-kit/i18n
```

## Usage

```go
import (
    "embed"

    "github.com/fromforgesoftware/go-kit/i18n"
)

//go:embed locales/*.json
var localesFS embed.FS

func main() {
    i18nInstance, err := i18n.LoadFromFS(localesFS, "locales", "en", "en")
    if err != nil {
        panic(err)
    }

    i18nInstance.Translate("es_MX", "common::mobile")              // "Celular"
    i18nInstance.Translate("es_MX", "common::save")                // "Guardar" (base es)
    i18nInstance.Translate(
        "en",
        "welcome::message",
        map[string]string{"name": "Ada"},
    )                                                              // "Welcome, Ada!"

    // Use generated constants once you've run `cmd/i18n-gen` (see below):
    i18nInstance.Translate("en", i18n.SharedEnumRoleOrgadmin)      // "Customer Administrator"

    // Enum-shaped UI helper:
    roles := i18nInstance.GetKeyValues("en", "shared::enum::role")
    // map[string]string{"org.admin":"Customer Administrator", ...}
}
```

Use `LoadFromDir` instead of `LoadFromFS` for filesystem reads during
local development.

### Fallback chain

`exact locale -> base language (en from en_US) -> fallback language ->
the key itself`.

### Key shape

`::` separates namespaces so dots / underscores / hyphens are free to
live inside key names (`shared::enum::role::org.admin`,
`planner::roster-section-heading`). The generator strips in-key
punctuation only from the constant identifier — the original key value
is preserved.

## Constants generator

The [`forge` CLI](../../cmd/forge/README.md) generates compile-time-safe
key constants:

```sh
go install github.com/fromforgesoftware/forge/go/cmd/forge@latest

# In your service root (the directory with locales/):
forge i18n generate-constants --lang go
```

Writes `i18n_constants.go` (pkg `i18n`) from `locales/en.json`. Override
with `--out`, `--pkg`, `--primary` if you need different paths.

The output gives you leaf-key constants, parent-key constants, and
`AllTranslationKeys` / `AllTranslationParentKeys` slices. Wire it into
your service's `go generate` so the constants stay in sync with the
canonical locale file:

```go
//go:generate forge i18n generate-constants --lang go
```

`forge i18n validate` will also lint the locales directory for typos,
shape mismatches, and (with `--strict`) missing translations.

## API

| Method                                       | Returns             | Notes                                                  |
| -------------------------------------------- | ------------------- | ------------------------------------------------------ |
| `Translate(locale, key, params...)`          | `string`            | exact -> base -> fallback -> key                       |
| `HasTranslation(locale, key)`                | `bool`              | walks the same fallback chain                          |
| `GetAvailableLanguages()`                    | `[]string`          | sorted list of loaded locale codes                     |
| `GetKeys(locale, parentKey)`                 | `[]string`          | leaf keys with full namespace, merged across fallbacks |
| `GetKeyValues(locale, parentKey, params...)` | `map[string]string` | direct string children only — enum-shaped UI helper    |
| `GetValues(locale, parentKey, params...)`    | `[]KeyValue`        | sorted-by-id slice variant of `GetKeyValues`           |

## Test + bench

```sh
cd go/kit && go test ./i18n/...                          # tests
cd go/kit && go test -race ./i18n/...                    # race-clean
cd go/kit && go test -bench=. -benchmem -run=^$ ./i18n/  # benchmarks
```
