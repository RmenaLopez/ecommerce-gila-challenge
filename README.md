# E-Commerce Challenge

> Draft README — started as raw notes from working sessions with Claude Code. Everything below is accurate but written in a fairly literal, unpolished way; it's meant to be rewritten in my own words before submission, not shipped as-is.

An "enterprise-grade" e-commerce app built as a technical take-home: product CRUD, CSV import, product search, a purchase flow, a local database, a UI, and Docker support. Backend is Go, frontend is ClojureScript.

## Status

Feature-complete on the backend — product CRUD (including lookup by SKU), search, CSV import, and the purchase flow (including looking up a past order) are all implemented, tested, and verified end-to-end against the real running server. `POST /orders` handles multi-item purchases transactionally: locking and checking stock per item, rejecting the whole purchase with every problem listed if anything can't be fulfilled, and correctly preventing overselling under concurrent purchases (verified with an actual concurrent-goroutines test, not just reasoned about). Full product field validation is in place on both `Create` and `Update`.

The frontend now covers the full loop, not just a product-list skeleton: browsing products, creating and editing them, building a cart, and checking out — all wired to the real backend via re-frame, with client-side routing (`/products`, `/products/new`, `/products/:id/edit`, `/cart`) instead of everything living on one screen. Every piece has been verified live against the real running backend, including both the success and rejection paths of checkout (a real over-stock purchase correctly rejected with the backend's actual per-item reason). Not yet built: view/rendering tests (deliberately, see the testing philosophy decision below) and a persisted, user-owned cart (the current cart is frontend-only and ephemeral — see "Known gaps").

A Dockerized test pipeline (`docker compose --profile test run --rm test`) covers the backend's domain, service, repository, and HTTP layers; the frontend has its own `cljs.test` suite for event-handler logic (`npx shadow-cljs compile test`). See `progress-log.md` at the repo root for the running dev log.

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

Starts Postgres, applies migrations automatically (`db-migrate`, a one-shot service the `backend` container waits on), then starts the backend. The frontend is not included by default (see below).

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

SQL migrations live in `backend/migrations/`, using the `golang-migrate` naming convention (`NNNNNN_name.up.sql` / `.down.sql`). Both `docker compose up` and `docker compose --profile full up` apply them automatically via the `db-migrate` service before `backend` starts — see "Automatic migrations for the dev database" below. They only need to be run by hand when running the backend outside Docker (the "Backend only, without Docker" setup above) or against some other database:

```
migrate -path backend/migrations -database "$DATABASE_URL" up
```

### Running the backend test suite

```
docker compose --profile test run --rm test
```

Runs `go test ./...` inside a Linux container against a dedicated, disposable `test-db` Postgres container (migrated automatically first). Tests don't run natively on macOS here — see the "Tests run in Docker" decision below for why. `docker compose --profile test down -v` tears the test containers and volumes down entirely, if a truly clean slate is ever needed (e.g. after a migration file changes shape).

### Running the frontend test suite

```
cd frontend
npx shadow-cljs compile test
```

Runs `events.cljs`'s handler-logic tests (`cljs.test`, via a `:node-test` shadow-cljs build) under Node — no browser needed. Covers pure event-handler logic; view/rendering isn't tested yet, deliberately (see the testing philosophy decision below).

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

**Follow-up, once search finally had a frontend to actually type into**: real usage immediately surfaced that a short prefix ("Camp") found nothing, while the full word ("Camping") worked — this had gone unnoticed because "verified live" earlier only ever meant hitting the endpoint directly with full/exact-ish query strings, never trying it interactively the way an actual person types into a search box. Root-caused precisely against real Postgres (not assumed): `pg_trgm`'s `%`/`similarity()` — what the query originally used — compares two *whole strings*, so a short query scores too low against a much longer name (`similarity('Camping Chair', 'Camp')` = 0.2857, just under the 0.3 default threshold) purely because so much of the longer string goes uncovered. `word_similarity()`/`<%` (and its commutator `%>`) exist specifically for "does the query match some substring of a longer string" — the same pair scores 0.8 that way. One genuine subtlety caught while fixing this: `word_similarity`'s two arguments aren't interchangeable the way `<%`/`%>` are (`word_similarity('Camp', 'Camping Chair')` = 0.8 vs. the reversed `word_similarity('Camping Chair', 'Camp')` = 0.3077) — confirmed both orders directly against the database before writing any Go code, not just trusted the documentation's description. Fixed by switching to `name %> $1` / `word_similarity($1, name) DESC`. Also added `TestProductRepository_SearchByName_PrefixMatch` — this SQL had zero repository-level test coverage before this, despite being exactly the kind of real, non-trivial logic this project's testing philosophy says should have it.

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

Tests aren't a listed requirement of this challenge, and aren't treated as the top priority relative to finishing the required features — but they're written for the pieces of logic actually worth protecting with a regression test, as those pieces get built, rather than deferred to a separate "add tests" pass at the end: service-layer business rules (`SearchByName`'s pagination clamping, `Create`'s SKU-required validation, `ImportCSV`'s row validation and duplicate-SKU deduplication) against a fake in-memory `ProductRepository`, no database needed; repository-layer behavior that's genuinely easy to silently break (`BulkUpsert`'s insert-vs-update branching, SKU immutability on `Update`, `GetBySKU`) as integration tests against real Postgres; and, for `GET /products`, an HTTP-layer integration test (`internal/api/product_handler_test.go`) that routes a real request through a real `Server` backed by real Postgres — chosen because the actually-new, riskiest logic in that handler is the query-param-to-`ProductFilter` mapping, which a service- or repository-level test alone wouldn't exercise. `ImportService`'s unit test uses the exact edge cases found in the real challenge sample CSV (a `$`-prefixed price, a duplicate SKU) rather than invented ones. `OrderRepository.CreatePurchase` gets the most thorough coverage of anything in the project — a successful multi-item purchase (correct total, correct stock decrements), the whole-purchase-rejected-with-every-problem-listed case, and, notably, an actual concurrency test that fires two real goroutines at a single-unit-of-stock product and asserts exactly one succeeds — proportional to it being the riskiest, most consequential logic built so far, not an arbitrary choice. Trivial passthrough methods don't get tests just to hit a coverage number.

The same philosophy applies on the frontend: `events.cljs`'s handlers (`fetch-products`, `products-loaded`, `products-load-failed`, `initialize-db`) are pure functions — `db`/event in, new `db` (or effects map) out — and are tested with plain `cljs.test`, run via a `:node-test` shadow-cljs build (`npx shadow-cljs compile test`, no browser needed). View rendering isn't tested yet, deliberately — it's a heavier thing to test well, and less valuable while the views themselves are still actively being built. Getting there required naming the event handlers as top-level functions rather than passing anonymous functions straight to `reg-event-db`/`reg-event-fx` — a small, standard re-frame idiom, since an anonymous function registered directly in re-frame's registry isn't otherwise callable on its own for a unit test.

### Testing ClojureScript under Node surfaces a real gap: browser-only globals

Running the new test build the first time failed with `Cannot find module 'xmlhttprequest'` — `events.cljs` requires `ajax.core` (for `day8.re-frame/http-fx`'s HTTP effect), and `ajax.core` expects a browser's `XMLHttpRequest` global, which plain Node doesn't have. Fixed with the standard, well-known solution for exactly this combination: installing the `xmlhttprequest` npm package as a dev dependency, which polyfills the global under Node. Verified: `npx shadow-cljs compile test` now runs all 4 tests / 11 assertions cleanly.

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

### Full product field validation — and why `Update` needs its own version, not the same one `Create` uses

SKU-only validation was an intentionally incomplete first pass — revisited once the project's overall bar shifted from "good enough for a coding exercise" to "would a real team ship this" (see `progress-log.md`/memory notes around 2026-09-08). `domain.Product` now has two validation methods rather than one:

- **`Validate()`** — everything a new product needs, called from `Create`: non-empty `sku`, non-empty `name`, and non-negative `price_cents`/`stock`/`weight_kg` (matching the schema's own `CHECK` constraints for the first two, and the same rule CSV import already applies for the third, which has no `CHECK` constraint but is just as physically nonsensical negative). `category`/`description` stay optional, consistent with the CSV import decision that an uncategorized or description-less product is a legitimate state.
- **`ValidateForUpdate()`** — everything above *except* `sku`. This isn't a separate decision so much as a direct consequence of `sku` being immutable via `Update` (see above): a `PUT` request body might not include `sku` at all, and since `Update` ignores whatever's there regardless, requiring it would wrongly reject a perfectly valid update that simply omitted an unused field.

Both live as methods on `domain.Product` itself rather than in the service layer — validating "is this data acceptable" is a domain-level concern, not tied to HTTP or persistence, and keeping it on the type makes it independently unit-testable (`internal/domain/product_test.go`) without a fake repository or any service at all. Error messages are specific (`"price_cents must be non-negative"`, not a generic "invalid input"), which `writeServiceError` already surfaces to the client via `err.Error()` — a real, visible improvement to API usability, not just an internal check.

CSV import's own row-level validation was deliberately left as-is, not refactored to call these — it already has equivalent checks with more useful, row-specific error messages (`"row 7 (YM-015): invalid price \"free\""`), and forcing it through a generic `Validate()` call would have made those messages worse for no real benefit. Not every duplication is worth eliminating immediately; this one was judged not worth the trade-off.

### SKU is immutable via `PUT /products/{id}`

Decided explicitly rather than leaving it as an accident of `Update`'s original `SET` clause (which never included `sku`): a product's SKU cannot be changed through `Update`. Reasoning: a SKU is a stable identifier other things may come to depend on (past order line items, external references) — real inventory systems generally treat re-assigning one as an unusual, deliberate operation, not a routine field edit. The alternative (allow it, and add the same duplicate-SKU `409` handling `Create` has) was considered and rejected for this project — nothing currently needs it, and the `Create` version already demonstrates that pattern if it's ever needed here too.

`Update`'s `RETURNING` clause was extended to also pull back `sku` and `created_at` (previously just `updated_at`), so the handler returns a complete, accurate resource in one database round trip regardless of what the request body contained — a request body's `sku` field is simply overwritten by the real, unchanged value read back from the row, rather than silently trusted or requiring a second `GetByID` call to fetch the rest of the resource. The `id` in the URL is likewise authoritative over the request body's `id`, matching the standard REST convention that a resource's identity comes from where it lives, not from its representation.

### `GET /products` query parameters: `q`, `category`, `limit`, `offset`

`q` was picked over the more literal `name` for the search-text parameter — it's the near-universal convention for a free-text search box (GitHub, Google, etc.), immediately recognizable without reading docs, even though `SearchByName` only ever matches against the `name` column internally.

Malformed `limit`/`offset` values (`?limit=abc`) return `400`, not a silent fallback to the default — consistent with how a malformed product `id` is handled (see "Malformed IDs are rejected before touching the database" above): a client sending garbage should get a clear error, not a guess at what it meant. An *absent* or empty value is different from a malformed one, though — `?limit=` or omitting `limit` entirely falls back to `ProductService.SearchByName`'s existing default (20)/max (100) clamping, since there's nothing actually wrong with not specifying a value. `internal/api/params.go`'s `parseIntParam` implements this distinction and is shared by both `limit` and `offset`.

### CSV import: upload mechanism, validation, and the duplicate-SKU-within-one-file problem

`POST /products/import` takes a `multipart/form-data` file upload (`file` field) rather than a raw request body — the standard way a real file-upload feature works, and what a browser `<input type="file">` posts by default, which the eventual frontend import UI will need anyway.

Before implementing, the actual challenge sample CSV (`Code Challenge E-Commerce.csv`) was analyzed row-by-row, since it turned out to contain deliberately planted edge cases rather than uniformly clean data: a `$`-prefixed price, a literal non-numeric price (`"free"`), negative stock, an empty name, a whitespace-only name, two fully blank rows, an XSS payload as a product name, a SQL-injection payload as a SKU, and — most consequentially — the same SKU repeated 2–3 times within the file itself. Validation was designed against these specific cases, not hypothetical ones:

- `sku` and `name` are both required (trimmed, non-empty) — same rule `Create` already applies to `sku`, extended to `name` since a nameless product doesn't make sense.
- `price` strips an optional leading `$`, then must parse as a non-negative number — this is exactly why `"free"` needs rejecting rather than crashing the batch.
- `stock` must parse as a non-negative integer.
- `weight_kg` defaults to `0` when blank (matching the schema default); a present-but-unparseable value still gets rejected, for consistency with the other numeric fields.
- `category` is not required — an uncategorized product is a legitimate state.
- The XSS and SQL-injection payloads are accepted and stored as plain text, no special filtering — verified live: the SQL-injection SKU is stored as inert text with the `products` table completely unaffected (parameterized queries make the content irrelevant), and the XSS name is stored as inert text (the actual defense against that is output-encoding at render time in the frontend, not input filtering, which is a well-known weak defense anyway).

A single row failing any of these doesn't fail the import — it's counted in `ImportResult.Skipped` with a specific reason in `ImportResult.Errors` (e.g. `"row 7 (YM-015): invalid price \"free\""`), and the rest of the file still imports. This is why validation has to happen row-by-row in `ImportService` *before* the single `BulkUpsert` call — a bad value reaching the database would fail the whole atomic batch instead of just that row.

**The duplicate-SKU case surfaced a real bug risk, confirmed against actual Postgres, not just reasoned about:** `BulkUpsert` sends its whole batch as one multi-row `INSERT ... ON CONFLICT` statement, and Postgres flat-out rejects a single statement that targets the same conflict key twice (`ERROR: ON CONFLICT DO UPDATE command cannot affect row a second time`) — confirmed by reproducing it directly before writing any handling for it. Since the sample file contains exactly this (SKU `BS-021` appears 3 times, `RS-001` twice), naively passing every parsed row straight to `BulkUpsert` would have crashed the entire import on this exact file. Fixed in `ImportService` by deduplicating by SKU before building the batch, keeping the *last* occurrence in file order and recording earlier ones in `ImportResult.Errors` as `"superseded by a later row with the same sku"`. "Last wins" was chosen deliberately, not just as the simpler option: the sample data's own descriptions confirm it — one duplicate's description literally says "Updated lightweight shoes," another literally says "Same SKU different description and price" — the file was clearly constructed to test exactly this, and same-SKU-means-same-product-being-updated is consistent with how this whole project has treated SKU as the product's business identity from the start.

Verified end-to-end against the real sample file: `88` rows imported, `9` skipped, matching a hand-derived prediction of exactly which 9 rows and why, made *before* running the import (6 for failing validation, 3 for being superseded duplicates).

### The purchase flow: never trust client-supplied prices, and the first multi-statement transaction in this codebase

`POST /orders` takes `{"items": [{"product_id": "...", "quantity": N}, ...]}` — no price in the request. Price is always looked up fresh from the product and snapshotted into `order_items.unit_price_cents` at purchase time, never accepted from the client; otherwise a client could just declare its own price. This is also why `order_items.unit_price_cents` exists as its own stored column rather than being computed on the fly from a join to `products` — a later catalog price change must not retroactively alter what a historical order shows it was actually charged.

This is the first feature in the codebase needing a transaction that spans multiple statements across multiple tables — everything before this was a single SQL statement per repository call. `OrderRepository.CreatePurchase` owns the whole thing: for every item, it locks and checks the product row, and only if every item can be fulfilled does it decrement stock and create the order and its line items, all inside one `pgx.Tx`. This is also why `CreatePurchase` reaches directly into the `products` table via raw SQL rather than composing through `ProductRepository`'s own methods — a cross-table, transactional operation like this doesn't decompose cleanly into calls against two separate single-table repositories without either a bigger unit-of-work abstraction spanning both, or breaking the transaction boundary. For a purchase-sized operation, owning the SQL directly in one method was the simpler choice; a `Transactor`/unit-of-work pattern would be the thing to introduce if more operations like this show up.

**No partial fulfillment — the whole purchase is rejected if any item can't be fulfilled, but every problem is reported at once, not just the first one.** The original instinct was either "stop at the first bad item" or "silently fulfill what you can, drop the rest" (the way a real storefront like Amazon quietly removes an out-of-stock item from your cart) — but this project has no cart, and no user/session system at all to say whose cart it even is, so a partial purchase would just create an order missing items the customer had no chance to confirm losing. Rejecting the whole request and reporting every problem in one response lets whoever's retrying (today: a direct API caller; eventually: a frontend) fix everything in a single pass instead of a frustrating one-error-at-a-time retry loop.

**Concurrent purchases of the same product can't oversell stock — verified with an actual concurrency test, not just reasoned about.** Each item's stock check-and-decrement happens via `SELECT price_cents, stock FROM products WHERE id = $1 FOR UPDATE` followed by an application-level check, inside the transaction. The row lock taken by `FOR UPDATE` is held until the transaction commits or rolls back, so no concurrent purchase can read stale stock in between the read and the write — this is exactly as safe as a single atomic statement would be, just structured as two statements instead of one. It was chosen over a single conditional `UPDATE ... WHERE stock >= quantity` specifically because the "collect every problem" requirement above needs to distinguish *why* an item failed (`"product not found"` vs. `"insufficient stock: requested N, available M"`) for every item, and the conditional-`UPDATE` approach can't tell those apart without a follow-up query anyway. `TestOrderRepository_CreatePurchase_ConcurrentPurchasesDoNotOversell` fires two real concurrent goroutines at a product with exactly one unit of stock and asserts exactly one succeeds — this is the one test in the project that specifically exercises concurrency rather than just sequential correctness.

**A persisted, user-owned cart is the ideal next step, not a shortcut skipped for time.** The frontend does now have a cart (see "The cart: frontend-only, in app-db" below) — but it's purely ephemeral client state, gone on page refresh, with no backend resource behind it at all. A *persisted* cart needs its own resource, its own storage, and critically a notion of *whose* cart it is, which means this project would need a user/session/auth system it doesn't have at all today. Building persisted cart storage without solving "who does this belong to" first would just be state with no real owner. Worth naming explicitly as the honest next architectural step, not glossed over.

### Closing the "implemented but unreachable" gap: `GetBySKU` and order lookup by ID

`ProductRepository.GetBySKU` and `OrderRepository.GetByID` were both built and tested weeks-in-code-time before this pass, but neither had an HTTP route calling them — a fully-working capability nobody could actually use, which doesn't hold up once the bar shifted to "would a real team ship this." `OrderRepository.GetByID` was also still a bare stub at the repository layer despite the interface already declaring it — actually implementing it required two queries (the order row, then its `order_items`, since an order's line items live in a separate table with a foreign key back to it) rather than one.

`GET /products/sku/{sku}` was chosen over reusing `/products/{id}` with a smarter handler that guesses whether the path segment is a UUID or a SKU — same reasoning as the earlier decision against auto-detecting search intent: don't make the backend infer what the caller means from a string's shape when the URL structure can just say it unambiguously. Confirmed live that this doesn't collide with the existing `/products/{id}` route — Go's `net/http` router matches on path *shape* (segment count), and `/products/sku/{sku}` has one more segment than `/products/{id}`, so a request can only ever match one of them. `GET /orders/{id}` reuses the exact same `parseIDParam` malformed-ID handling already used everywhere else, rather than inventing a new pattern for it.

### Verifying `docker compose --profile full`, and adding automatic migrations for the dev database

Running the full containerized stack end-to-end for the first time surfaced a real gap: the `frontend` Dockerfile's `node:20-alpine` build stage has no JDK, but `shadow-cljs` (the ClojureScript build tool, run via `npx shadow-cljs release app`) is a JVM tool — the build failed with `Executable 'java' not found on system path`. Fixed by adding `RUN apk add --no-cache openjdk11-jre-headless` to the build stage, same as this project's own machine needing Java 11 to run `shadow-cljs` locally. Logged in `BUGS.md`.

Fixing that revealed a second, more structural gap: unlike the `test` profile (which has `test-db-migrate` applying schema before tests run), nothing migrated the dev `db` automatically. It only "worked" because `db`'s Docker volume already had schema applied from earlier manual `migrate` runs during development — a fresh clone with an empty volume would start `db` successfully but with zero tables, and `backend`'s first query would fail. Verified this precisely: ran the stack under a separate Compose project name (`-p ecommerce-migrate-verify`) so it got its own brand-new volume rather than touching the real one, confirmed `backend` returning `200 []` immediately, and confirmed both migrations (`create_products_table`, `create_orders_tables`) applied automatically in the logs.

Fixed by adding a `db-migrate` service — the same `migrate/migrate` one-shot pattern `test-db-migrate` already used — that `backend` now depends on via `condition: service_completed_successfully`. Considered instead using Postgres's built-in `docker-entrypoint-initdb.d` mechanism (SQL files it runs automatically, but only on a container's very first, truly-empty-volume boot): rejected because it silently stops covering schema changes the moment a migration file is added after that first boot, whereas `golang-migrate` tracks applied migrations in a `schema_migrations` table and stays correct indefinitely — every `docker compose up` now applies whatever's pending and no-ops (`no change`) otherwise, verified both ways.

### Frontend create/edit product form: local component state, not app-db; one component for both modes

Following the same "local Reagent state for anything nothing else needs to react to" principle used for the product list, the create/edit form's field values and open/closed/editing-target state live in plain Reagent atoms (`fields` inside `product-form`, `form-target` at the `views.cljs` namespace level), not `app-db` — only the request's `:submitting?`/`:error` status lives in `app-db`, since that's what the async re-frame event handlers need to write into. `product-form` is a Reagent "form-2" component (an outer function that runs once per mount, returning an inner render function) specifically so that closing and reopening it — or switching between different products — gives it fresh field values for free, with no manual reset code: `product-manager` forces a remount whenever `form-target` changes to a different product (or to `:new`) by attaching a changing `:key` to the mounted `[product-form ...]` element, since Reagent would otherwise reuse the existing component instance and show stale data from whichever product was being edited before.

One component handles both create and edit (an optional `editing-product` argument, `nil` for create) rather than two separate components — the two modes differ only in: whether fields start pre-filled (via `product->fields`, the inverse of `build-product-payload`), whether the SKU input is disabled (SKU is immutable via `PUT`, per the existing backend decision), which event gets dispatched on submit (`:create-product` vs `:update-product`), and the submit button's label. Considered a separate `product-edit-form` component instead: rejected as needless duplication of the same seven-field layout for a difference that's really just a handful of conditionals.

Validation and post-submit behavior mirror the create flow's earlier decisions exactly: no client-side rule duplication (the backend's real validation message is read via `get-in` out of the failure response and displayed as-is), and a successful edit triggers `:fetch-products` to refresh the list rather than patching the edited product into `app-db` directly.

Verified live end-to-end: a successful create appears in the list with correct price formatting; editing pre-fills real values (including a zero, `weight_kg: 0`, to make sure "falsy but valid" values render correctly); clicking "Edit" on a different product while the form is already open for another one correctly resets every field (proving the `:key` remount actually works, not just "closed then reopened"); a successful edit updates the list; and submitting an edit with a blank name surfaces the backend's actual `"invalid input: name is required"` message rather than a generic one.

### The cart: frontend-only, in `app-db` (not local, unlike the form)

Unlike the create/edit form's local-atom state, the cart (`:cart` in `app-db`, a map keyed by product id: `{"<id>" {:product {...} :quantity N}}`) lives in shared state — the deciding factor from the earlier local-vs-app-db criterion: it genuinely needs to be read and written from multiple, unrelated places (every product row's "Add to cart" button, plus the separate cart summary view), which is exactly the case the criterion says belongs in `app-db`. There's still no backend cart at all (see "Known gaps" — this was true before and remains true); the cart is purely a frontend accumulator, flattened into one `{:items [...]}` payload and sent as a single `POST /orders` request at checkout, then discarded.

Built and verified in two chunks. Chunk 1 (state only — add/remove/adjust quantity, no submission): three pure `reg-event-db` handlers (`add-to-cart`, `remove-from-cart`, `set-cart-quantity`) and a `cart-view` component with per-line `+`/`-`/`Remove` controls and a running total. Verified live: adding via the button and via re-adding the same product both correctly increment the existing line (not create a duplicate one), decrementing to zero removes the line entirely, and the running total's math is correct.

Chunk 2 (checkout): `checkout`/`order-placed`/`checkout-failed` mirror the create/update-product event pattern, but `checkout-failed` handles a genuinely different response shape — a rejected purchase (`409`) carries a `problems` array (`{:product_id ... :quantity ... :reason ...}` per failed item, see `order_handler.go`), richer than the generic `{"error": "..."}` used everywhere else in this app. Chose to display that real per-item detail (via `get-in error [:response :problems] []`) rather than just the generic message, since the backend already computes exactly which items failed and why. On success, the cart is cleared and `:fetch-products` refetches the list — deliberately, since a successful purchase decrements real stock on the backend, so the UI needs to reflect that, same reasoning as `product-created`/`product-updated` already refetching after their own writes.

Verified live both ways against the real backend: a valid purchase (2× a $24.99 item) correctly decremented that product's stock (30 → 28) and displayed a real order ID and total; an over-stock purchase (999 requested against 8 available) was rejected with the exact backend reason (`"insufficient stock: requested 999, available 8"`) surfaced in the UI, left stock completely unchanged (confirming the all-or-nothing transaction held), and kept the cart intact so the quantity could be corrected and retried.

### Client-side routing: `reitit-frontend`, hash-based URLs, and replacing the atom-based form toggle

Added `metosin/reitit-frontend` — the first new ClojureScript dependency since the initial `reagent`/`re-frame`/`http-fx` set — for real URLs (`/products`, `/products/new`, `/products/:id/edit`, `/cart`) instead of everything living on one screen behind plain Reagent atom toggles.

**Hash-based routing** (`/#/products/new`, via `{:use-fragment true}`) was chosen over real paths (`/products/new`) specifically to avoid a server-config dependency: the browser never sends anything after `#` to the server, so `index.html` loads identically regardless of which route a user lands on directly — a refresh, a bookmark, a shared link. Real paths would need nginx's config updated with an SPA fallback (`try_files ... /index.html`) for the production Docker build, since without it a fresh load of a deep URL like `/products/abc-123/edit` would 404 at the server before any ClojureScript even runs. Hash routing sidesteps that entirely, at the cost of a slightly less clean-looking URL — a reasonable trade for this project's scope.

**Moving create/edit onto their own routes replaced the old `form-target` atom/toggle-button mechanism entirely** (not layered on top of it) — `product-manager` no longer exists. What used to be "click a button, toggle some local state, conditionally render the form inline" is now real navigation: `product-list`'s "Edit" link points at `/products/:id/edit`, "Add product" points at `/products/new`, and a `case` in `current-page` (matching the current route's name against each option, same idea as `cond` but comparing one value against a fixed set of possibilities) decides which top-level page component to render. The old `:key`-based remount trick (forcing Reagent to discard stale form state when jumping between editing two different products) is still needed and still present — just now keyed off the route's `:id` path param instead of the old atom's value, since Reagent still won't automatically remount a component just because its props changed while it stays mounted at the same tree position.

**A custom `reg-fx` (`:navigate!`) was introduced** so a successful create/update can redirect back to `/products` without breaking the "handlers describe effects as data, re-frame performs them" model established from the very first `:http-xhrio`/`:dispatch` usage — `product-created`/`product-updated` now return `:navigate! :products` alongside their existing `:db`/`:dispatch` keys, and the one-line `reg-fx` handler (in `events.cljs`) is the only place that actually calls reitit's `push-state` function.

Verified live: the bare root URL correctly lands on the product list rather than reitit's default "no route matched" state (a real gap caught during testing — nothing matches an empty hash fragment, so `nil` route names are now treated the same as `:products`); nav links and the browser's back/forward buttons both work correctly; a successful create or edit redirects back to `/products`; and — the case the whole `:key` mechanism exists for — navigating directly between two different products' edit URLs (via the address bar, no intermediate route) correctly resets every field rather than showing stale data from whichever product was being edited before. Also re-verified the production Docker build (`docker compose --profile full build frontend`) still succeeds cleanly with the new dependency added.

### CSV import UI: real multipart file upload, not JSON

Every other frontend request in this app sends JSON (`:params` + `ajax/json-request-format`), but `POST /products/import` expects a real multipart file upload (the backend handler reads `r.FormFile("file")`), so it needed a genuinely different `:http-xhrio` shape: a `js/FormData` object (the browser API for building a multipart body) passed directly as `:body`, with no `:format` at all — the browser sets the correct `multipart/form-data` `Content-Type` (including its required boundary) on its own once it sees a `FormData` body, exactly as it would for a plain HTML `<form>` file upload. Reading the picked file off the `<input type="file">` element needed `aget` (indexing into the browser's native `FileList`, which isn't a real Clojure collection) rather than any of the map/vector functions used everywhere else in this codebase.

Verified live through the actual browser UI (not curl, unlike the original backend-only verification) against the real challenge sample CSV: `Imported: 88, Skipped: 9`, exactly matching the original prediction, with the real per-row skip reasons (invalid price, negative stock, empty name, duplicate SKU superseded) displayed to the user rather than just logged. Also incidentally confirmed a real security property while doing this: the sample CSV contains a deliberate stored-XSS payload as a product name (`<script>alert('xss')</script>`) — it rendered as inert plain text in the product table, not executed, which is Reagent/React's automatic text-escaping working as expected rather than something this project did deliberately to defend against it.

### nginx cache headers: preventing a stale frontend build after a rebuild

Discovered a real bug while checking a "the UI isn't showing my change" report: `shadow-cljs release` outputs a fixed, unhashed filename (`js/main.js`) on every build, and stock `nginx:1.27-alpine` serves static files with no `Cache-Control` header at all. Without one, browsers apply their own heuristic caching and can keep serving an old cached `main.js` indefinitely after a rebuild — with no filename change to ever signal that anything's different, unlike a typical production frontend build pipeline that content-hashes filenames specifically to make this a non-issue. Confirmed precisely (see `BUGS.md`): the built container's on-disk JS already had the new code, but a normal reload — even in a brand-new browser tab — still showed the old version; only a genuine hard reload (bypassing the disk cache) showed the current one.

Fixed with `frontend/nginx.conf` (a custom `default.conf`, copied into the image in `Dockerfile`), setting `Cache-Control: no-cache` — this forces a cheap revalidation request (via the file's `ETag`) on every load instead of trusting a possibly-stale local copy, so a rebuild is always reflected on the very next refresh. Content-hashed filenames (the more typical production answer to this) were considered but not worth the added build complexity at this project's scale; `no-cache` gets the same practical outcome (never serve stale) at the cost of one small revalidation request per load, which is negligible for an app this size.

### Delete button: an inline confirm instead of `js/confirm`, and a real cljs-ajax gotcha

The delete button uses an inline, in-component confirmation ("Delete this product? Yes/No" replacing the button in place) rather than the browser's native `js/confirm` dialog — a small, deliberate choice: native confirm dialogs are blocking and generally considered dated UX in modern web apps, and an inline confirmation is just as safe against a stray click while staying consistent with everything else being a plain re-frame-driven UI element. `confirming?` is local component state (an `r/atom`, form-2 pattern), same reasoning as the cart quantity input — nothing outside this one row needs to know it's mid-confirmation.

Building this surfaced a genuine cljs-ajax gotcha, not a mistake specific to this feature: a `DELETE` request with nothing to send (no `:params`, no `:body`) still needs an explicit `:format` key, or cljs-ajax throws `"unrecognized request format: nil"` — traced into its source, `ApplyRequestFormat` unconditionally tries to resolve a write function from `:format` for any request that doesn't already have a `:body` (a `:body` — like the CSV import's `FormData` — short-circuits this check entirely via a separate interceptor). `GET` requests are apparently exempt in practice, which is why `fetch-products` never needed this. Fixed by setting `:format (ajax/text-request-format)` even though nothing is actually written — see `BUGS.md` for the full trace.

### Query injection vs. cross-site scripting: verifying the CSV import's `<script>` payload is neither

The challenge's sample CSV contains a product name of `<script>alert('xss')</script>` — a classic XSS test string. Worth being precise about what it actually tests, since it's easy to conflate with SQL injection (different vulnerability, different payload shape — SQL injection looks like `' OR '1'='1`, aimed at breaking a query's syntax; XSS looks like this, aimed at getting a browser to execute injected script). Checked both, rather than assuming either was fine:

- **Not SQL injection**: `BulkUpsert`'s query construction uses `fmt.Sprintf` only to build placeholder *positions* (`$1`, `$2`, ...) — actual values, including a product name, are always passed as bind parameters, never concatenated into the SQL text. No value can alter the query's structure regardless of its content.
- **Not XSS**: confirmed there's zero use of `dangerouslySetInnerHTML` anywhere in `frontend/src` (grepped the whole tree). Every value renders through ordinary hiccup text interpolation, which Reagent/React always escapes automatically — verified live, the payload rendered as inert plain text with a clean console, no script execution.

Nothing needed fixing here; both paths already went through the same safe patterns used everywhere else in the app.

### CSV import: add-to-stock vs. overwrite-stock, defaulting to add

Originally, re-importing a CSV always replaced an existing product's stock outright (`stock = EXCLUDED.stock`) — reasonable for every other field (name, price, description all *should* just reflect the latest CSV), but wrong for stock specifically: a restock CSV re-run would silently erase any stock changes that happened in between exports (a sale, a manual adjustment), rather than adding to what's already there. Changed the default to *add* to existing stock on conflict, with an explicit opt-in to the old overwrite behavior — matching the request as stated, and because "add" is the safer default: it can't silently erase stock the way "overwrite" can, while "overwrite" remains available for whenever someone genuinely wants the CSV's numbers to be authoritative (e.g., reconciling after a stock count).

Implemented as a boolean (`addToStock`) threaded through `BulkUpsert` → `ImportService.ImportCSV` → the HTTP handler, which reads it from an optional `mode` multipart form field (`"overwrite"` opts out; anything else, including the field being absent, defaults to add) — kept as a single flag covering the whole import rather than a per-row choice, matching how the feature was asked for. `BulkUpsert`'s `ON CONFLICT` clause switches between `stock = EXCLUDED.stock` and `stock = products.stock + EXCLUDED.stock` based on the flag; every other field's `SET` clause is unchanged in both modes. New products (no existing SKU) are unaffected by the flag either way, since `INSERT`'s values are never touched by the `ON CONFLICT` branch at all.

Verified precisely, not just by reasoning: re-imported the exact same CSV twice in a row through the real browser UI. With "Add" (the new default), every product's stock exactly doubled. With "Overwrite," a third import reverted every value back to the CSV's raw numbers exactly. Also added tests at both layers this change touched: the repository test (real Postgres, both modes, asserting the actual combined/replaced stock value) and the service test (asserting the flag threads through to `BulkUpsert` unchanged).

### Frontend pagination: a real bug where a created product silently never appeared

A user report — "I created a product, it's not showing up" — traced to a genuine gap rather than a create-flow bug: `fetch-products` called `GET /products` with no `limit`/`offset` at all, and the backend silently defaults to `limit=20` with no total count in the response. Once real usage (repeated CSV re-imports during testing) pushed the product count past 20, anything beyond the first page simply vanished from the UI with no indication a next page even existed. Confirmed precisely before fixing anything: `curl`ing the backend directly showed the "missing" product genuinely existed, and resubmitting the exact same create the user had done returned `"sku already exists"` — proof the original attempt had actually succeeded.

Fixed with real pagination rather than just raising the fetch limit (which would only move the same problem to a later, larger product count): `:products-page` (`{:limit :offset}`) lives in `app-db` — shared state, since both the fetch itself and the Previous/Next buttons need it — with `next-page`/`prev-page` events adjusting `:offset` and refetching. Since the backend never reports a total count, "is there a next page" is inferred the standard way without one: a full page (exactly `limit` items returned) means "there might be more," enabling Next; a partial page means this was the last one. Verified live by paging through all 93 real products (5 pages) and finding the "missing" item exactly where expected on the last page, with Next correctly disabled there.

### Frontend search: wiring up what the backend already fully supported, and where it still falls short of "enterprise-grade"

Asked directly whether the pagination work above made this area "enterprise-grade." The honest answer was no, for a bigger reason than pagination itself: `SearchByName`/`GET /products?q=&category=` had been fully built, tested, and verified on the backend since early in this project — and never once wired to any frontend UI. Every product-list view up to this point only ever browsed everything, paginated; there was no way to search from the app at all, despite the backend fully supporting it.

Fixed with `search-form` — two inputs (name, category) plus Search/Clear — dispatching `search-products`/`clear-search`, which set `:products-page`'s `:q`/`:category` and reset `:offset` back to `0` (staying on whatever page you were browsing before would otherwise land you mid-way through a completely different result set) before refetching. `q`/`category` get escaped with `js/encodeURIComponent` before being placed in the URL — unlike `limit`/`offset` (always plain numbers), these are free text, and a category like `"Home & Office"` (real data in the sample CSV) would otherwise corrupt the query string via its own `&`.

**Search is submit-triggered (Enter or the Search button), not live-search-as-you-type.** Recording this honestly rather than dressing it up: this was mostly a time-constraint call, not a strong claim that submit-triggered is the only correct answer. Live search-as-you-type is the more modern, arguably more polished UX, but needs debouncing to be usable (firing a request per keystroke otherwise) and more state to manage well; submit-triggered was the pragmatic choice given the time available for this pass.

Verified live: a name search ("Air Purifier") correctly narrowed to its one match; a category search for `"Home & Office"` correctly returned exactly its 13 members (proving the URL-encoding genuinely matters, not just defensive code); "Clear" correctly restored the full unfiltered, paginated view.

**Where this area still falls short of genuinely enterprise-grade**, named honestly rather than glossed over:
- **A real race condition on rapid pagination clicks.** `products-loaded` unconditionally overwrites `:products` with whatever response arrives, with no way to tell if it's even for the request that's still current — click Next twice quickly and, if responses arrive out of order, the UI can silently end up showing the wrong page with no error. Not fixed in this pass.
- **No total count or page indicator.** The backend API returns a bare array, never a count — so "is there a next page" has to be inferred rather than known, and the UI can never say how many results or pages actually exist.
- **No specific empty-results state for search** — a zero-match search just renders an empty table, same as a slow load, rather than a "no results for X" message.

## Known gaps

- Test coverage is intentionally selective (see "Tests are written when logic is genuinely risky" above) — not every method has a test, by choice.
- The cart is frontend-only and ephemeral (see "The cart: frontend-only, in app-db" above) — no persisted, user-owned cart exists on the backend, since there's no user/session/auth system in this project at all; refreshing the page loses an in-progress cart.
- `product-edit-page` looks up the product being edited from the already-loaded `:products` list rather than fetching it individually — correct in practice (the list is always fetched on startup, before routing can happen), but a genuinely invalid product id in the URL shows "Loading..." forever rather than a real not-found message, since nothing distinguishes "not loaded yet" from "will never exist."

Bugs found and fixed along the way (rather than just avoided) are logged separately in `BUGS.md`, per a tip from a previous candidate who took this challenge.
