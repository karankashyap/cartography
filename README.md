# Cartograph
> Local-first AI e-commerce analytics. Your data never leaves your machine.

## Quick start (2 minutes)

```bash
git clone https://github.com/karankashyap/cartograph && cd cartograph
docker compose -f deploy/docker-compose.yml up
# open http://localhost:3000
# drag sample-data/shopify_orders.csv onto the import panel
```

## Demo

https://github.com/user-attachments/assets/4fd258ff-eaad-4c38-aa8a-e19e7d75da54

## Features

- **Analytics dashboard** — revenue, AOV, cohort retention, dead stock, inventory velocity
- **AI narrative** — grounded insights, zero hallucinated numbers (V3: only pre-computed metrics passed to LLM)
- **Text-to-SQL chat** — multi-turn conversation history, safe read-only queries, 6-layer guardrail suite
- **AI provider switching** — Ollama (fully local) or LM Studio (Gemma 4 / any OpenAI-compatible server)
- **Content studio** — product descriptions, SEO copy, email campaigns
- **Semantic product search** — pgvector + HNSW index via `nomic-embed-text` embeddings
- **Expo mobile dashboard** — React Native metrics + charts
- **Multi-platform import** — Shopify, Amazon, WooCommerce CSV

## Architecture

```mermaid
graph TD
  Browser["Browser :3000"] -->|GraphQL / WS| API["Go API :8080\ngqlgen + pgx"]
  Mobile["Expo Mobile"] -->|GraphQL| API
  API -->|SQL| DB["Postgres 16\n+ pgvector"]
  API -->|HTTP| Ollama["Ollama :11434\nllama3.2 (default)"]
  API -->|HTTP| LMStudio["LM Studio :1234\nGemma 4 / any model"]
  Worker["Go Worker"] -->|embed jobs| DB
  Worker -->|embed model| Ollama
  CSV["CSV Upload\nShopify / Amazon / Woo"] -->|parse + upsert| Worker
```

## Stack

| Layer | Tech |
|---|---|
| API | Go 1.22 · gqlgen · pgx/v5 |
| LLM | Ollama (llama3.2) · LM Studio (Gemma 4) — fully local, OpenAI-compatible |
| Database | Postgres 16 + pgvector + pg_trgm |
| Web | Next.js 14 (App Router) · Tailwind · urql |
| Mobile | Expo SDK 56 · victory-native |
| Monorepo | Turborepo + pnpm workspaces |
| Infra | Docker Compose — single `up` to run everything |

## AI providers

Cartograph supports two local LLM providers, switchable per-request from the UI dropdown.

| Provider | Default model | Use case |
|---|---|---|
| Ollama | `llama3.2` | Fast, lightweight, runs anywhere |
| LM Studio | `gemma-4-27b` | Higher accuracy, requires GPU |

Override via environment variables in `deploy/.env`:

```env
OLLAMA_URL=http://host.docker.internal:11434
OLLAMA_MODEL=llama3.2

LM_STUDIO_URL=http://host.docker.internal:1234
LM_STUDIO_MODEL=gemma-4-27b
```

> LM Studio must be set to listen on `0.0.0.0` (not `127.0.0.1`) for Docker to reach it via `host.docker.internal`.

## Security: Text-to-SQL

LLM proposes, deterministic Go disposes. Store isolation is enforced server-side — the LLM never sees or sets `store_id`.

```
Layer 1: keyword blocklist        — rejects INSERT/UPDATE/DELETE/DROP/…
Layer 2: statement type check     — rejects anything that is not SELECT or WITH
Layer 3: read-only Postgres role  — cartograph_chat user: SELECT only, no writes physically possible
Layer 4: store filter injection   — server rewrites FROM/JOIN to filtered subquery before execution
Layer 5: LIMIT injection          — caps result set at 100 rows
Layer 6: repair loop              — on exec error, sends failed SQL + Postgres error back to LLM for one retry
```

Full test suite: 20+ malicious inputs, all blocked. See [`services/api/internal/ai/sql_test.go`](services/api/internal/ai/sql_test.go).

## Invariants

```
V1: ∀ SQL from LLM → guardrail.Validate() before execution
V2: text-to-SQL role = cartograph_chat (read-only, no writes physically possible)
V3: narrative → metrics JSON only passed, no raw rows
V4: email → SHA-256 hashed at parse time, plain text never stored
V5: import idempotent → upsert by external_id, no double-count on re-upload
V6: LLM unavailable → dashboard still renders (AI features degrade gracefully)
V7: ∀ metric computation → SQL/Go, never LLM
```

## Running tests

```bash
# Go — ingest parsers + analytics + AI guardrails
cd packages/ingest-core && go test ./... -v -race
cd ../../services/api   && go test ./... -v -race

# Web typecheck
pnpm turbo run typecheck

# E2E (requires running stack)
cd apps/web && pnpm exec playwright test
```

## Project structure

```
cartography_code/
├── apps/
│   ├── web/          # Next.js 14 dashboard + chat + content studio
│   └── mobile/       # Expo SDK 56 mobile app
├── services/
│   ├── api/          # Go GraphQL API (gqlgen)
│   └── worker/       # Go ingestion + embedding jobs
├── packages/
│   └── ingest-core/  # Shopify / Amazon / WooCommerce CSV parsers
├── deploy/           # Docker Compose + Dockerfiles + Postgres init
└── sample-data/      # Shopify CSV exports for local testing
```
