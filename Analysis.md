# new-api — Codebase Analysis

## What it is

`new-api` (org: QuantumNous) is a self-hosted **AI API gateway**: it sits in front of 35+ upstream model
providers and exposes a single, consistent surface (OpenAI Chat/Responses, Anthropic Messages, Gemini,
realtime/WebSocket, images, audio, embeddings, rerank, and pluggable async task APIs) plus a web console for
routing, auth, quotas, billing, and observability.

- **Backend**: Go 1.25.1, Gin, GORM v2. ~240k LOC across `.go` files.
- **Frontend**: `web/` — React 19 + TypeScript, Rsbuild, TanStack Router/Query/Table, Zustand, Base UI,
  Tailwind CSS 4, Bun as package manager. ~276k LOC across `.ts`/`.tsx`.
- **`relaykit/`**: an independently-buildable Go module (own `go.mod`) that isolates protocol DTOs and
  request/response conversion between the four text protocols (OpenAI Chat, Responses, Anthropic Messages,
  Gemini) from transport/auth/DB/billing concerns, which stay in the host module.
- **Databases**: SQLite, MySQL (>=5.7.8), PostgreSQL (>=9.6) must all be supported for the primary DB; a
  separately configured log database additionally supports ClickHouse.
- **Cache**: Redis (go-redis) plus an in-memory layer.
- **Auth**: sessions, API/personal access tokens, JWT, WebAuthn/Passkeys, TOTP, OAuth/OIDC; Casbin-based
  authorization lives in `service/authz/`.
- **Extensibility**: JavaScript task plugins (image/video/etc. async jobs) executed via the Sobek JS engine,
  under `plugins/tasks/` + `pkg/jsplugin/`; there's also an Electron desktop wrapper (`electron/`).

## High-level architecture

```
main.go → InitResources() (DB/Redis/i18n/authz/pricing/OAuth/cache warmup)
        → router.SetRouter()
              ├─ api-router.go        management/console REST API
              ├─ relay-router.go      upstream-facing relay endpoints (/v1/*)
              ├─ channel-router.go    channel test/admin endpoints
              ├─ task-router.go / task-plugin-protocol-router.go   async task + plugin routes
              ├─ video-router.go
              ├─ authz-router.go
              └─ web-router.go        serves the embedded React build (web/dist, go:embed)
```

Request flow for a relay call is roughly:
`router/` → `middleware/` (auth, rate limit, i18n, request id, logging) → `controller/` → `relay/` (per-provider
adaptor in `relay/channel/<provider>/`, request/response conversion via `relaykit/`) → `service/` (quota,
billing, pricing, task settlement) → `model/` (GORM models/queries) → primary DB (+ Redis cache) and a
separate log DB (optionally ClickHouse).

Background jobs started from `main.go`: channel cache sync, option hot-reload, authz policy sync, quota-data
dashboard aggregation, Codex credential auto-refresh, subscription quota resets, a system-instance reporter for
multi-node deployments, and a DB-lease-based scheduled task runner for channel tests / model list updates /
async task polling (Midjourney, Suno, video, etc.).

### Directory map (Go side)

| Dir | Files | Role |
| --- | --- | --- |
| `controller/` | 127 | HTTP handlers for the management API |
| `relay/` | 244 | Provider adaptors (`relay/channel/*`), relay helpers, task-plugin protocol handling |
| `relaykit/` | 130 | Standalone protocol-conversion module (no host imports) |
| `model/` | 111 | GORM models: users, channels, tokens, logs, pricing, subscriptions, etc. |
| `service/` | 112 | Business logic: billing/quota, task settlement, authz (`service/authz`), passkeys (`service/passkey`) |
| `setting/` | 66 | Typed settings modules (billing, ratios, model, console, performance, reasoning, system, perf metrics) |
| `common/` | 65 | Cross-cutting utilities (JSON wrapper, env/config, crypto, caching helpers) |
| `middleware/` | 38 | Gin middleware (auth, i18n, request id, rate limiting, trusted proxies) |
| `pkg/` | 44 | Standalone packages: `jsplugin` (Sobek JS runtime), `billingexpr`, `cachex`, `wsmanager`, `perf_metrics`, `ionet` |
| `router/` | 18 | Route registration, split by concern, each with its own `_test.go` |
| `constant/` / `dto/` / `types/` | 15 / 8 / 4 | Shared enums/constants, DTOs, typed value objects |
| `oauth/` | 10 | OAuth/OIDC + custom provider loading |
| `plugins/tasks/` | 15 | JS task plugin sources |

Supported relay channels (`relay/channel/*`, 35 provider dirs): OpenAI, Anthropic Claude, Google Gemini/PaLM,
Vertex AI, AWS Bedrock, Azure (via `openai`/`advancedcustom`), Cloudflare, Codex, Cohere, Coze, DeepSeek,
Dify, Jimeng, Jina, LingyiWanwu, Minimax, Mistral, MokaAI, Moonshot, new-api-to-new-api, Ollama, OpenRouter,
Perplexity, Replicate, SiliconFlow, sub2api, Submodel, Tencent, Volcengine, xAI, Xinference, Xunfei (iFlytek),
Zhipu/Zhipu-4V, Baidu/Baidu v2, Ali (Qwen), AI360 — plus a generic `advancedcustom` adaptor and a `task`
adaptor abstraction for async job platforms.

### Frontend (`web/src/`)

Feature-folder structure under `web/src/features/`: `auth`, `channels`, `chat`, `dashboard`, `keys`,
`model-pricing`, `models`, `performance-metrics`, `playground`, `pricing`, `profile`, `rankings`,
`redemption-codes`, `security`, `setup`, `subscriptions`, `system-info`, `system-settings`, `system-update`,
`task-plugins`, `usage-logs`, `users`, `wallet`, plus shared `components/`, `stores/` (Zustand), `hooks/`,
`lib/`, `context/`, `routes/` (TanStack Router), and `i18n/` (7 languages: en, zh-CN, zh-TW, fr, ru, ja, vi).

## Notable conventions enforced by the repo (from `AGENTS.md`)

These are load-bearing for any future change and worth calling out up front:

1. **JSON**: all marshal/unmarshal in the root module must go through `common.Marshal`/`common.Unmarshal`/etc.
   (`common/json.go`); `relaykit/` uses its own `kitutil` wrapper instead. Raw `encoding/json` calls are
   disallowed in business code.
2. **Three-database compatibility**: every DB-touching change must be verified against real SQLite, MySQL, and
   PostgreSQL instances (not just mocks/unit tests). Row locking must go through `lockForUpdate(tx)`, not the
   legacy GORM v1 `Set("gorm:query_option", ...)` pattern (silently a no-op in GORM v2).
3. **relaykit independence**: must remain buildable standalone (`cd relaykit && GOWORK=off go build ./...`),
   with zero imports from the root module.
4. **Billing is a gated subsystem**: a specific list of files/paths (pricing, quota, settlement, task billing,
   `relay/helper/price.go`, etc.) requires reading `.agents/rules/billing.md` in full before any change.
5. **Relay DTOs**: optional scalar fields destined for upstream JSON must be pointers with `omitempty` so
   explicit zero values survive round-tripping (client `0`/`false` must not collapse into "absent").
6. **JS task plugins**: contract is documented in `docs/plugin-api/v1.md` (+ schema/`.d.ts`); pricing-field
   descriptions in plugin schemas have specific wording rules (must name billing subject + unit price).
7. **Auth flows** must follow OWASP ASVS / Cheat Sheet Series guidance explicitly, with server-side enforcement
   and sanitized audit logging.
8. Strict **test hygiene** rules (no scattered per-layer test files for one small change, no coverage-only
   tests, testify `require`/`assert` for new/rewritten tests) and **frontend UI reuse** rules (check
   `web/AGENTS.md` + existing shared components before adding new UI).
9. **Protected identifiers**: "new-api" and "QuantumNous" branding/attribution must never be removed or altered.
10. Docs/plugin directories: no new files under `docs/` or plugin dirs unless explicitly requested by the user.

## Repo state observed

- Current branch: `claude/analysis-documentation-6eqjx3`, working tree clean at time of writing.
- Recent history is active maintenance: stream-option/channel fixes, pricing-UI diffing, plugin-in-use dialog
  UX, playground/log-table styling — consistent with a mature, actively-developed product rather than a
  greenfield project.

## Where to go next

Some natural follow-ups, depending on what you actually want to do:

- **Billing/pricing deep dive** — read `.agents/rules/billing.md` plus `pkg/billingexpr/`,
  `setting/billing_setting/`, `model/pricing*.go` if the next task touches pricing/quota logic.
- **Add or modify a provider channel** — look at an existing `relay/channel/<provider>/` as a template plus
  `relaykit/` conversion code.
- **Frontend feature work** — read `web/AGENTS.md` and the relevant `web/src/features/<name>/` directory.
- **Task-plugin work** — read `docs/plugin-api/v1.md` before touching `pkg/jsplugin/` or `plugins/tasks/`.
- Tell me which of these (or something else entirely — a bug, a feature request, a specific file) you want to
  dig into, and I'll go deeper there.
