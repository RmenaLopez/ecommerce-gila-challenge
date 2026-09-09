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

### The nginx-served frontend could silently serve a stale build after a rebuild

The user reported the CSV import UI was missing after rebuilding/reloading `docker compose --profile full`'s frontend on `:8081`, even right after the feature was added. Verified the built JS bundle on disk in the running container did contain the new code (`grep` for the feature's text inside `/usr/share/nginx/html/js/main.js`), and confirmed live in a completely fresh browser tab that the feature was still missing there too — ruling out "just reload the tab." Root cause: `shadow-cljs release` outputs a fixed, unhashed filename (`js/main.js`) every build, and the stock `nginx:1.27-alpine` image serves static files with no `Cache-Control` header at all — only `Last-Modified`/`ETag`. Without an explicit caching policy, browsers apply their own heuristic caching and can keep serving an old cached copy of `main.js` indefinitely, with no cache-busting filename change to ever signal that anything's different. Confirmed precisely: a normal reload (even in a brand-new tab) still showed the old build; a genuine hard reload (bypassing the disk cache) immediately showed the correct, current one.

Fixed by adding `frontend/nginx.conf` (a custom `default.conf`, copied into the image in the `Dockerfile`) setting `Cache-Control: no-cache` on everything nginx serves — this forces a cheap revalidation request (using the file's `ETag`) on every load rather than trusting a possibly-stale local copy, so a rebuild is always picked up on the very next refresh going forward. This doesn't retroactively fix an *already*-cached copy from before the header existed — that one genuinely needs one hard reload to clear — but prevents the same silent staleness from recurring on any future rebuild. The exact same class of staleness turned up again minutes later on shadow-cljs's own dev server (`:8021`), which apparently also serves `main.js` with no explicit cache header — same symptom (stale code despite a normal page navigation), same fix (a hard reload).

### `delete-product` threw "unrecognized request format: nil" — cljs-ajax requires an explicit request `:format` for any non-GET request with no body

Adding the frontend delete button surfaced a genuine cljs-ajax gotcha: `delete-product`'s `:http-xhrio` map had no `:params`, `:body`, or `:format` — reasonable, since a `DELETE` has nothing to send — but clicking "Delete" threw `Error: ["unrecognized request format: " nil]` every time. Traced into cljs-ajax's actual source (`ajax.interceptors/ApplyRequestFormat`): it unconditionally tries to resolve a `:write` function from `:format` for every request unless a `:body` key is already present (which short-circuits it via a separate `DirectSubmission` interceptor — the reason the CSV import's FormData-as-`:body` request never hit this). With `:format` absent, that resolution fails and it throws, regardless of whether there's actually anything to write. `GET` requests apparently don't need `:format` at all in the same way (`fetch-products` has always worked without it) — the gap is specifically non-GET requests with nothing to send. Fixed by explicitly setting `:format (ajax/text-request-format)` on `delete-product`'s request even though nothing is actually sent. Verified live: delete now completes and refetches the list correctly, with the error gone from the console.

### A newly created product silently didn't appear in the list — `fetch-products` never paginated

Reported by the user: created a product, it never showed up in the list, no errors visible. Confirmed via `curl` that the product genuinely existed in the database — the create had actually succeeded. Root cause: `fetch-products` calls `GET /products` with no `limit`/`offset` at all, and the backend silently defaults to `limit=20` with no total count in the response (`ProductService.SearchByName`). By the time this was hit, CSV re-imports over the course of testing had pushed the real product count to 93 — so anything beyond the first 20 (by whatever order the backend returns them in) simply never appeared, with nothing in the UI to indicate a next page existed at all. Reproduced precisely: resubmitting the exact same create the user did returned `"sku already exists"`, proving the original attempt had succeeded and the list was just incomplete.

Fixed by adding real pagination: `:products-page` (`{:limit :offset}`) in app-db, `next-page`/`prev-page` events that adjust `:offset` and refetch, and Previous/Next buttons in the UI. Since the backend never reports a total count, "is there a next page" is inferred by comparing how many products came back against the page size (a full page means "maybe more," the standard approach without a count). Verified live by paging through all 93 real products across 5 pages and confirming the "missing" item was exactly where expected on page 5, with Next correctly disabled once a page came back partial.

### Fuzzy search failed on short prefixes — `pg_trgm`'s `%` compares whole strings, not substrings

The user noticed searching "Camp" returned nothing while "Camping" correctly found "Camping Chair"/"Camping Tent". Verified precisely against the real database rather than guessing:

```sql
SELECT similarity('Camping Chair', 'Camp');       -- 0.2857 (just under the 0.3 default threshold)
SELECT word_similarity('Camp', 'Camping Chair');  -- 0.8
SELECT 'Camping Chair' % 'Camp';                  -- false
SELECT 'Camp' <% 'Camping Chair';                 -- true
```

Root cause: `SearchByName`'s SQL used `name % $1` (`WHERE`) and `similarity(name, $1)` (`ORDER BY`) — `pg_trgm`'s whole-string similarity operator, which scores a short query poorly against a much longer name purely because so much of the longer string isn't covered by the query's few trigrams. `word_similarity`/`<%` (and its commutator `%>`) exist specifically for "does the query match some substring of this longer string" — the actual behavior a search box needs. One real subtlety found while fixing this: `word_similarity`'s two arguments are *not* interchangeable the way `<%`/`%>` are — `word_similarity('Camp', 'Camping Chair')` = 0.8, but `word_similarity('Camping Chair', 'Camp')` = 0.3077 (the reversed, wrong comparison) — confirmed by testing both orders directly before writing any code.

Fixed by switching `SearchByName`'s `WHERE` to `name %> $1` and its `ORDER BY` to `word_similarity($1, name) DESC` (query first). Also discovered this SQL had zero repository-level test coverage at all despite being real, non-trivial logic — added `TestProductRepository_SearchByName_PrefixMatch` as a regression test, using a made-up unique token (not a real word) so it can't accidentally pass by matching unrelated data in the shared test database. Verified live: "Camp" now correctly finds both "Camping Chair" and "Camping Tent" through the real UI.

## Open

None currently — every bug/failure found so far has been fixed or designed around. This section stays here for whatever turns up next.
