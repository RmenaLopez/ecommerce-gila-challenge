# Bugs & Failures Log

> Draft — raw notes, meant to be reworded before submission, same as `README.md`. Kept separate from the README on purpose, per a tip from a previous candidate who took this challenge.

A running log of bugs and failures found during development — both fixed and, while they lasted, not yet fixed — rather than folding all of it into the README's decisions section.

## Fixed

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

## Open

None currently — every bug found so far has been fixed. This section stays here for whatever turns up next.
