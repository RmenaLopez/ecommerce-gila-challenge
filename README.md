# E-Commerce Challenge

> Draft README — started as raw notes from working sessions with Claude Code. Everything below is accurate but written in a fairly literal, unpolished way; it's meant to be rewritten in my own words before submission, not shipped as-is.

An "enterprise-grade" e-commerce app built as a technical take-home: product CRUD, CSV import, product search, a purchase flow, a local database, a UI, and Docker support. Backend is Go, frontend is ClojureScript.

## Status

In progress — `ProductRepository`'s full CRUD (`Create`, `GetByID`, `GetBySKU`, `Update`, `Delete`), `SearchByName`, and `BulkUpsert` are implemented and verified against real Postgres. `GET`, `POST`, `PUT`, and `DELETE` on `/products[/{id}]` are all wired up and verified end-to-end against the real running server (`docker compose up db backend`) — search, CSV import, the purchase flow, and the whole frontend UI are still stubbed or not started. A Dockerized test pipeline (`docker compose --profile test run --rm test`) has a handful of tests in it so far — see "Tests are written when logic is genuinely risky" below. See `progress-log.md` at the repo root for the running dev log.

## Prerequisites

- Go 1.22+
- Node.js 18+ and npm
- Java 11+ (required by the ClojureScript/shadow-cljs toolchain)
- Docker and Docker Compose

## Project layout

- `backend/` — Go REST API, layered as `domain` → `repository` (interfaces) → `repository/postgres` (implementations) → `service` → `api` (HTTP handlers)
- `frontend/` — ClojureScript SPA (shadow-cljs + Reagent)

## Running locally

### Database + API via Docker

```
docker compose up --build
```

Starts Postgres and the backend. The frontend is not included by default (see below).

### Frontend (dev, hot reload)

```
cd frontend
npm install
npm run watch
```

### Full stack, including a production frontend build

```
docker compose --profile full up --build
```

### Backend only, without Docker

```
cd backend
export DATABASE_URL=postgres://ecommerce:ecommerce@localhost:5434/ecommerce?sslmode=disable
go run ./cmd/api
```

### Database migrations

SQL migrations live in `backend/migrations/`, using the `golang-migrate` naming convention (`NNNNNN_name.up.sql` / `.down.sql`). Run them with the `migrate` CLI, or via its Docker image, e.g.:

```
migrate -path backend/migrations -database "$DATABASE_URL" up
```

### Running the test suite

```
docker compose --profile test run --rm test
```

Runs `go test ./...` inside a Linux container against a dedicated, disposable `test-db` Postgres container (migrated automatically first). Tests don't run natively on macOS here — see the "Tests run in Docker" decision below for why. `docker compose --profile test down -v` tears the test containers and volumes down entirely, if a truly clean slate is ever needed (e.g. after a migration file changes shape).

## Architecture, decisions & alternatives considered

### Overall backend shape: layered architecture

`domain` (plain structs, no framework/DB knowledge) → `repository` (interfaces) → `repository/postgres` (pgx-backed implementations) → `service` (business logic) → `api` (HTTP handlers, using Go 1.22's built-in method+path routing on `net/http.ServeMux`, e.g. `"GET /products/{id}"`).

Why: keeps business logic independent of both the HTTP layer and the DB driver, and makes repositories swappable/mockable for testing the service layer without a real database. Considered `chi`/`gorilla/mux` for routing instead of stdlib — went with stdlib since Go 1.22's pattern matching is enough for method+path REST routes and it avoids an extra dependency; `chi` would be the alternative if a richer middleware ecosystem is needed later.

### Money as integer cents, not float

`Product.PriceCents` is an `int64`, not a `float64`. Avoids floating-point rounding drift in price/total arithmetic — a classic source of subtle bugs in commerce systems. The sample CSV has dollar amounts (some prefixed with `$`), so the CSV importer will need to strip the symbol and convert to cents on the way in.

### Postgres over SQLite for the "local database"

The challenge asks for a local database plus Docker support. Went with Postgres run via `docker compose` — closer to how a real service would be deployed, while still trivial to run locally, and demonstrates real migrations/indexing. SQLite was the main alternative considered: zero extra services, simpler for a pure local exercise, but a weaker demonstration of production DB practice (migrations, indexing, extensions) for something meant to look "enterprise-grade."

### `pg_trgm` trigram index for search

Product search is a required feature. Added a GIN trigram index (`pg_trgm` extension) on `products.name` to get reasonable fuzzy/substring search directly from Postgres, instead of standing up a separate search engine (Elasticsearch, Meilisearch, etc.). Considered a dedicated search engine as the "more enterprise" alternative, but judged it disproportionate to this project's scope — the trigram index is a deliberately lighter-weight choice that still demonstrates search isn't just `LIKE '%...%'`.

### `golang-migrate` for schema migrations

Plain versioned `.up.sql`/`.down.sql` files, no ORM-driven migration DSL. Easy to read/review, and doesn't tie schema evolution to any particular Go library.

### Raw SQL via `pgx`, not an ORM

Went with hand-written SQL over `database/sql`/`pgx` rather than an ORM (GORM, Ent). Reasoning:

- The repository interface (`ProductRepository`, etc.) already gives the service layer the abstraction an ORM would provide — an ORM would just be another layer *inside* the repository implementation, not a replacement for the seam that matters.
- Some required features — trigram search, a bulk upsert for CSV import, a transactional stock-decrement during purchase — are exactly the kind of query ORMs tend to make awkward or generate inefficiently. Raw SQL means knowing precisely what runs against the database.
- Idiomatic Go leans away from heavy ORMs more than, say, Rails/Java shops do; `pgx` + hand-written SQL is a common, well-regarded production pattern, not a shortcut.
- Fewer moving parts to explain if asked "what does this actually run against the DB?" — no generated/lazy-loaded queries to reason about.

Trade-off: more boilerplate (manual `Scan` calls per query), no compile-time query validation, and column names/parameter order have to be kept correct by hand. **`sqlc`** (generates type-safe Go structs and query functions from SQL, checked at build time) was the strongest middle-ground alternative considered — it removes the "struct vs. schema can silently drift" risk below, at the cost of adding a codegen step. Decided plain `pgx` was simpler to set up and explain for this project's size.

### No schema-to-struct auto-mapping (`domain.Product` as the "table struct")

Because there's no ORM, there's also no automatic link between the Go struct and the database schema. `internal/domain/product.go` is deliberately kept as a 1:1 mirror of `migrations/000001_create_products_table.up.sql` — it's the place in the code that says "a product has these fields," and the file to check instead of opening the SQL migration. Nothing enforces that mirroring, though: if a migration adds/renames a column, `domain.Product` (and any hand-written column lists in the repository) have to be updated by hand, and drift wouldn't be caught until runtime. This is the manual-sync cost that tools like `sqlc` or an ORM's struct tags would remove automatically; accepted as a reasonable trade-off at this project's scale.

### Repository error translation

Repository methods translate driver-specific errors into domain errors at the boundary — e.g. `pgx.ErrNoRows` becomes `domain.ErrNotFound` — so the service and handler layers never need to know pgx exists. Other errors are wrapped with `fmt.Errorf("...: %w", err)` to preserve context while staying inspectable via `errors.Is`/`errors.As`. Not yet handled: translating Postgres unique-violation errors (e.g. duplicate SKU, Postgres error code `23505`) into a domain-level conflict error instead of letting the raw driver error surface.

### `Create` uses `RETURNING`, not a plain `INSERT`

`ProductRepository.Create` omits `id`, `created_at`, and `updated_at` from the `INSERT` column list (all three have DB defaults — `gen_random_uuid()` / `now()`), then reads them back via `RETURNING id::text, created_at, updated_at` into the same struct passed in. Reasoning: if those columns were listed with caller-supplied values instead, the DB defaults would be silently bypassed (e.g. a caller that forgets to set `CreatedAt` would insert the Go zero-value timestamp instead of "now"); and without `RETURNING`, the caller would have no way to learn the DB-generated ID for the resource it just created (needed e.g. to return `201 Created` with the new product's ID).

### ClojureScript: shadow-cljs + Reagent, re-frame deferred

Chose `shadow-cljs` over the older `lein-figwheel` toolchain for first-class npm interop and more active maintenance/documentation — relevant since Clojure/ClojureScript is new to me. Reagent is the thin, near-universal React wrapper for ClojureScript. Deliberately stopped the scaffold at a single "hello world" component and did not add `re-frame`'s `app-db`/`events`/`subs`/`views` structure yet — that's real state-management architecture that should be designed once the actual screens (product list, search, cart) and their state needs are known, not scaffolded blindly.

### Docker Compose profiles

Default `docker compose up` starts only `db` + `backend`, since frontend development uses `shadow-cljs watch` for hot reload rather than a container. `docker compose --profile full up` additionally builds and serves a production frontend bundle via nginx, so the fully containerized stack is still demonstrable for submission.

### pgx version pinned below latest

Pinned `github.com/jackc/pgx/v5` to `v5.6.0` instead of latest (`v5.10.0`). The newest version requires Go ≥1.25, which would silently bump this module's `go` directive and force an automatic toolchain download on every build. `v5.6.0`'s own transitive dependencies stay compatible with the installed Go 1.22.4 toolchain.

### `SearchByName`, not a generic `Search` — and no SKU-vs-name auto-detection

The repository method is `SearchByName`, not `Search`: it only ever fuzzy-matches against `products.name` (via `pg_trgm`'s `%` operator) plus an optional exact `category` filter — it was originally named `Search`, which overpromised "searches everything." A precise-lookup-by-SKU path already exists separately as `GetBySKU`.

Considered making one search entry point smart enough to detect whether the input looks like a SKU or a name and pick the right query automatically. Rejected: that just moves the ambiguity from "one honestly-scoped SQL condition" into a fragile string-shape heuristic (breaks silently if the SKU format ever changes) — and both operations already exist as separate, clearly-named methods. The right place for "which one did the user mean" to be decided is the caller (e.g. a UI with an explicit "Search by: Name | SKU" control that calls the matching method) — the backend shouldn't guess intent from a string's shape when the caller can simply state it.

### Pagination defaults/limits enforced in the service layer, not the repository

`ProductService.SearchByName` clamps `filter.Limit` to a default of 20 (if ≤ 0) and a max of 100 (if higher), and floors `filter.Offset` at 0, before calling the repository. `ProductRepository.SearchByName` trusts its input is already sane. Reasoning: "what's an acceptable page size" is a business rule, not a persistence concern — keeping it in the service layer means the repository stays a dumb, honest pass-through, and there's exactly one place that owns the pagination policy.

### `BulkUpsert`: SKU as the conflict key, one multi-row statement, atomic

`sku` is the upsert key (`ON CONFLICT (sku) DO UPDATE ...`) — it's the only column with a `UNIQUE` constraint, and the natural "is this the same product as before" identity when re-importing a CSV (the DB-generated `id` can't serve that role, since a fresh import wouldn't know it).

Considered three ways to send the batch to Postgres: a loop of one upsert per row (simplest, but N network round trips); a single multi-row `INSERT ... VALUES (...),(...),(...) ON CONFLICT (sku) DO UPDATE ...` (one round trip, moderate code, built the same dynamic-placeholder way `SearchByName` builds its `WHERE` clause); or `COPY` into a staging table followed by a merge (the genuinely fastest option for huge imports, but `COPY` doesn't support `ON CONFLICT` directly, so it needs a temp table and a second statement). Went with the single multi-row statement — real efficiency gain without staging-table machinery this project's CSV size doesn't justify.

`BulkUpsert` is intentionally all-or-nothing: a single SQL statement either fully succeeds or fully fails, so it assumes it's being handed already-valid `domain.Product` values. Row-level validation (a malformed price, a negative stock value in one CSV row) is `ImportService`'s job, not `BulkUpsert`'s — the service layer is expected to validate each parsed row first, build a batch of only the valid ones, and separately track which rows were skipped and why (matching the `ImportResult{Imported, Skipped, Errors}` shape already sketched in `import_service.go`). Keeps the repository a dumb, honest primitive and the "what counts as a valid product" business rule in exactly one place.

### Tests are written when logic is genuinely risky, not for coverage's sake

Tests aren't a listed requirement of this challenge, and aren't treated as the top priority relative to finishing the required features — but they're written for the pieces of logic actually worth protecting with a regression test, as those pieces get built, rather than deferred to a separate "add tests" pass at the end: service-layer business rules (`SearchByName`'s pagination clamping, `Create`'s SKU-required validation) against a fake in-memory `ProductRepository`, no database needed; and repository-layer behavior that's genuinely easy to silently break (`BulkUpsert`'s insert-vs-update branching, SKU immutability on `Update`, `GetBySKU`) as integration tests against real Postgres, run via the Docker test pipeline above. Trivial passthrough methods don't get tests just to hit a coverage number.

Earlier in development, `ProductRepository`'s tests deliberately avoided calling `GetBySKU` to verify other methods' results, even though it would've read naturally — `GetBySKU` had a bug at the time (see `BUGS.md`) and using it would have made an unrelated test fail for the wrong reason. Now fixed and has its own dedicated tests.

### Tests run in Docker, not natively on macOS

Native `go test`/`go run` on this development machine crashes with `dyld: missing LC_UUID load command` for anything importing `pgx` — traced to Go's internal linker on this specific macOS/Xcode combination, not a bug in this project's code (a dependency-free package's tests ran fine natively; only pgx-linked binaries crashed). Rather than fix a possibly machine-specific toolchain issue, tests run inside a Linux container instead — `backend/Dockerfile` already builds and runs this binary as Linux, so a macOS-linker-specific bug simply doesn't apply there. Confirmed empirically: a test opening a real `pgx` connection passed cleanly inside a container.

`docker-compose.yml` gained three services, gated behind `profiles: ["test"]` (same pattern as the existing `frontend` service's `"full"` profile) so none of them start on a plain `docker compose up`: `test-db` (Postgres), `test-db-migrate` (a one-shot `migrate/migrate` container applying the schema), and `test` (`go test ./...` against `backend/` bind-mounted as a volume, so edits are picked up without an image rebuild).

`test-db` is a fully separate Postgres container from the dev `db` service — not just a second database name on the same server. A shared-database approach was considered and deliberately rejected in favor of the more production-realistic separation, independent of whether this project's current size strictly required it — see the fuller reasoning in `knowledge/docker-test-pipeline.md`.

### Product JSON responses expose `price_cents` directly — no separate DTO layer

HTTP handlers serialize `domain.Product` straight to JSON; there's no separate request/response type translating cents to a decimal `price` field. Considered adding one (a `ProductResponse` type presenting `"price": 89.99`), which is the more conventional-looking REST shape, but decided against it for this project's size — it only adds mapping code with no functional benefit yet, since nothing currently needs the domain model and wire format to diverge. Precedent for exposing amounts in the smallest currency unit directly: Stripe's API does the same (amounts in cents), specifically to avoid float/decimal ambiguity — this isn't just a shortcut, it's a legitimate real-world convention. If a reason to diverge the wire format from the domain model shows up later, this is the seam where a DTO layer would get introduced.

### Shared `writeServiceError` helper for HTTP error mapping

Every handler funnels errors from the service layer through one function (`internal/api/response.go`) rather than hand-rolling its own status-code logic: `domain.ErrNotFound` → `404`, `domain.ErrInvalidInput` → `400`, anything else → `500` (logged server-side via `slog`, but not leaked to the client — an unexpected error's message could include internal details like SQL text). Built this alongside the first real handler (`GET /products/{id}`) specifically because every subsequent handler reuses it; verified end-to-end against the real running server, including the 200 and 404 paths.

### Malformed IDs are rejected before touching the database

Verifying `GET /products/{id}` live surfaced a real gap: a malformed (non-UUID) `id` returned `500`, not `400` — Postgres was rejecting the invalid UUID before the query could ever reach "not found," and that raw error fell through `writeServiceError`'s default case.

Considered two fixes: (a) translate Postgres's own `invalid_text_representation` error code into `domain.ErrInvalidInput` in the repository — reusing the exact pattern already used for `pgx.ErrNoRows` → `domain.ErrNotFound`, no new dependency; or (b) validate the ID's shape in the handler, before any service/repository call, using the small dependency-free `github.com/google/uuid` library. Went with (b): unlike "does this row exist" (which only the database can answer), "is this string a syntactically valid UUID" is knowable with zero ambiguity without touching the database at all — using a wasted DB round trip to validate pure string format doesn't hold up once framed that way, and avoiding unnecessary database calls matters for a system that's meant to look production-grade. `internal/api/params.go`'s `parseIDParam` is the shared helper every ID-based handler will use.

### SKU is mandatory; duplicate SKU on create is `409 Conflict`, not `500`

Wiring up `POST /products` (`handleCreateProduct`) surfaced the same class of gap as the malformed-ID case, but for a different reason. Two related decisions:

**SKU is required.** `ProductService.Create` trims `p.SKU` and rejects an empty result with `domain.ErrInvalidInput` (`400`), before ever calling the repository. The schema's `sku TEXT NOT NULL UNIQUE` already blocks an actual SQL `NULL`, but not an empty string — without this check, omitting `sku` from a request body would silently insert `sku = ""`. SKU is treated as mandatory because it's the meaningful business identifier for a product (matching how the sample CSV and real inventory systems use it), distinct from `id` (the auto-generated, opaque surrogate key) — a sellable product without a SKU isn't really a coherent state for this system. SKUs are never auto-generated anywhere in this codebase; they're always supplied by whoever creates the product (a human via the API, or a row in the imported CSV) — which is exactly why duplicates are a realistic scenario in the first place: data-entry mistakes, double-submitted requests, or races between two concurrent creates using the same SKU.

**A duplicate SKU on `Create` returns `409`, not `500`.** Unlike the malformed-ID case, there's no upfront-validation alternative here that actually solves the problem — even a pre-check `SELECT` before inserting can't prevent two concurrent `Create` requests from both passing the check and then racing into the same unique-constraint violation, so the database's constraint has to be the real arbiter regardless. The fix instead is the same pattern already used for `pgx.ErrNoRows`: `ProductRepository.Create` detects Postgres's unique-violation code (`23505`, via `pgconn.PgError`) and translates it to a new `domain.ErrConflict`, mapped to `409` in `writeServiceError`. `409` rather than reusing `400`/`ErrInvalidInput`, because the two are semantically different for a client to react to: `400` means "the request itself is malformed," `409` means "the request is fine, but conflicts with existing state" (the standard REST convention for uniqueness conflicts, per RFC 7231) — worth keeping distinct since `BulkUpsert` deliberately treats the same situation (an existing SKU) as the normal, expected case, while `Create` treats it as an error; the status code is part of how a caller tells those two operations' contracts apart.

### SKU is immutable via `PUT /products/{id}`

Decided explicitly rather than leaving it as an accident of `Update`'s original `SET` clause (which never included `sku`): a product's SKU cannot be changed through `Update`. Reasoning: a SKU is a stable identifier other things may come to depend on (past order line items, external references) — real inventory systems generally treat re-assigning one as an unusual, deliberate operation, not a routine field edit. The alternative (allow it, and add the same duplicate-SKU `409` handling `Create` has) was considered and rejected for this project — nothing currently needs it, and the `Create` version already demonstrates that pattern if it's ever needed here too.

`Update`'s `RETURNING` clause was extended to also pull back `sku` and `created_at` (previously just `updated_at`), so the handler returns a complete, accurate resource in one database round trip regardless of what the request body contained — a request body's `sku` field is simply overwritten by the real, unchanged value read back from the row, rather than silently trusted or requiring a second `GetByID` call to fetch the rest of the resource. The `id` in the URL is likewise authoritative over the request body's `id`, matching the standard REST convention that a resource's identity comes from where it lives, not from its representation.

## Known gaps

- Request body validation is still minimal — only `sku` is required so far (see "SKU is mandatory" above). Other fields (empty `name`, negative `price_cents`/`stock`, etc.) would currently reach the database uncaught. ID-format validation is handled separately (see "Malformed IDs are rejected before touching the database" above).
- Test coverage is intentionally selective (see "Tests are written when logic is genuinely risky" above) — not every method has a test, and none of the not-yet-implemented features (CSV import, purchase flow, search handler) do either, since they don't exist yet.
- CSV import (`ImportService.ImportCSV`), search (`GET /products`), and the purchase flow are all unimplemented at the HTTP layer.
- Frontend has no routing, data fetching, or state management yet.
- `docker compose --profile full` (the full stack including the frontend via nginx) has been written but never actually run end-to-end — nothing worth serving from the frontend yet.

Bugs found and fixed along the way (rather than just avoided) are logged separately in `BUGS.md`, per a tip from a previous candidate who took this challenge.
