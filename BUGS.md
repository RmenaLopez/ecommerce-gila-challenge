# Bugs & Failures Log

> Draft — raw notes, meant to be reworded before submission, same as `README.md`. Kept separate from the README on purpose, per a tip from a previous candidate who took this challenge.

A running log of bugs and failures found during development — both fixed and, while they lasted, not yet fixed — rather than folding all of it into the README's decisions section.

## Fixed

### Frontend used React 17's render API under React 18

Live browser verification of the first frontend chunk (re-frame + a product list view) surfaced a console warning: `ReactDOM.render is no longer supported in React 18. Use createRoot instead.` — `core.cljs` was using `reagent.dom/render`, Reagent's older API, while shadow-cljs had pulled in React 18. Not a functional bug (React just ran in a backward-compatibility mode), but worth fixing properly rather than ignoring. Checked Reagent 1.2.0's own jar contents rather than assume — confirmed it ships `reagent.dom.client` with `create-root`/`render` wrapping React 18's real `createRoot` API. Switched to it, with the root created via `defonce` (a React root must only be created once per DOM element; recreating it on every hot-reload would itself warn). Reverified live: the same page load/render/fetch behavior, zero warnings in the console afterward.

### `ProductRepository.Create` called itself (infinite recursion)

An early draft of `Create` read:

```go
func (r *ProductRepository) Create(ctx context.Context, p *domain.Product) error {
	err := r.Create(ctx, p)
	return err
}
```

`Create` calling itself with no base case — every call would recurse until the stack overflowed at runtime (not a compile error). Found during a code review pass before the method was actually finished; rewritten to perform the real `INSERT ... RETURNING` before this ever ran against a live database.

### `GET /products/{id}` returned `500` for a malformed ID instead of `400`

A non-UUID `id` (e.g. `/products/not-a-uuid`) reached Postgres, which rejected it as an invalid UUID — that raw error fell through to a generic `500`, rather than the `400` a client error should get. Fixed by validating the ID's shape (`github.com/google/uuid`) in the handler before any service/repository call, rather than relying on the database's own rejection. Verified live: 200 / 404 / 400 all correct for a real ID, a well-formed-but-missing ID, and a malformed ID respectively.

### `POST /products` returned `500` for a duplicate SKU instead of `409`

Creating a product with a SKU that already existed hit Postgres's unique-constraint violation, which also fell through to a generic `500`. Fixed by detecting the specific Postgres error code (`23505`, unique_violation) in `ProductRepository.Create` and translating it to a new `domain.ErrConflict`, mapped to `409 Conflict`. Verified live.

### Tests crashed natively on macOS (`dyld: missing LC_UUID load command`)

Not an application bug, but a real development-environment failure worth recording: any Go binary linking `pgx` (via `go test`, `go run`, or a built binary) crashed at execution with a dyld error specific to this machine's Go/macOS-linker combination — a dependency-free package's tests ran fine, only `pgx`-linked ones crashed. Rather than chase what looked like a possibly machine-specific toolchain issue, tests now run inside a Linux container (`docker compose --profile test run --rm test`), which sidesteps it entirely — confirmed empirically that a real `pgx` connection + query passes cleanly there. See `knowledge/docker-test-pipeline.md` for the full investigation.

### `ProductRepository.GetBySKU` — invalid SQL + wrong error message

```go
const query = "SELECT id, sku, name, description, category, price_cents, stock, weight_kg, created_at, updated_at" +
    "FROM products " +
    "WHERE sku = $1"
```

Two issues:
1. Missing space between `"...updated_at"` and `"FROM products "` — the concatenated string becomes `...updated_atFROM products...`, which fails at runtime with a Postgres syntax error (two adjacent word-characters glue into one invalid token — unlike punctuation, which doesn't need a surrounding space).
2. The error-wrapping message said `"querying product by id"`, copy-pasted from `GetByID`, instead of "by sku".

Never exercised by any HTTP route (nothing currently calls `GetBySKU` at all), which is exactly why it went unnoticed until deliberately caught during a design-session review rather than a live failure — there was no automated test for this method either, which is exactly the gap that let it go unnoticed for as long as it did. Fixed: added the missing space, corrected the error message, and added the `id::text` cast `GetByID` already used for consistency. Two new tests (`TestProductRepository_GetBySKU`, `TestProductRepository_GetBySKU_NotFound`) cover it now, both passing against real Postgres.

### `BulkUpsert` would have crashed on the actual challenge sample CSV (duplicate SKUs within one file)

Caught during design, before it ever ran against real data — but confirmed as a genuine bug against real Postgres, not just reasoned about. `BulkUpsert` sends its whole batch as a single multi-row `INSERT ... ON CONFLICT (sku) DO UPDATE` statement. Reproduced directly:

```sql
INSERT INTO products (sku, name, price_cents) VALUES
  ('BS-021', 'Bluetooth Speaker', 5999),
  ('BS-021', 'Bluetooth Speaker Updated', 4999)
ON CONFLICT (sku) DO UPDATE SET name = EXCLUDED.name, price_cents = EXCLUDED.price_cents;

ERROR:  ON CONFLICT DO UPDATE command cannot affect row a second time
HINT:  Ensure that no rows proposed for insertion within the same command have duplicate constrained values.
```

The actual challenge sample CSV (`Code Challenge E-Commerce.csv`) contains exactly this: SKU `BS-021` appears 3 times, `RS-001` twice. Naively parsing every row and handing them all to one `BulkUpsert` call would have crashed the entire import — not just skipped the duplicates, failed on all 97 rows — the first time this feature ran against the file it was built for. Fixed in `ImportService.ImportCSV` by deduplicating by SKU before building the batch (keeping the last occurrence in file order, recording earlier ones as skipped/superseded). Verified against the real sample file: `88` imported, `9` skipped, matching a prediction made before running the import.

### `frontend` Docker build failed — `shadow-cljs` needs a JVM, `node:20-alpine` doesn't have one

`docker compose --profile full` had never actually been run end-to-end before. Predicted the failure before running (the same tool needs Java 11 locally too), then confirmed it exactly: `RUN npx shadow-cljs release app` failed with `Executable 'java' not found on system path`. Fixed by adding `RUN apk add --no-cache openjdk11-jre-headless` to the `frontend/Dockerfile` build stage. Verified live afterward: the full stack (`db` + `backend` + `frontend`) builds and runs, the nginx-served frontend on `:8081` fetches from the backend and renders, confirmed in a real browser with a clean console.

### Dev database had no automatic migrations — would fail on a fresh clone

Fixing the above surfaced a second, more serious gap: `docker-compose.yml` had a one-shot `test-db-migrate` service applying schema for the `test` profile, but nothing equivalent for the dev `db`. It only "worked" during manual testing because `db`'s Docker volume already had schema applied from earlier development sessions. A genuinely fresh clone would start `db` successfully but empty, and `backend`'s first query would fail. Verified precisely by running the stack under an isolated Compose project name (its own fresh volume, no risk to the real one) and confirming `backend` returned `200 []` immediately with both migrations applied in the logs. Fixed by adding a `db-migrate` one-shot service (mirroring `test-db-migrate`) that `backend` now waits on via `condition: service_completed_successfully`.

## Open

None currently — every bug/failure found so far has been fixed or designed around. This section stays here for whatever turns up next.
