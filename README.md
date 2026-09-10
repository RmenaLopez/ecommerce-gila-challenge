# E-Commerce Challenge

> The CSV file was downloaded on September 5, 2026

An "enterprise grade" e-commerce app built as a technical interview exercise: product CRUD, CSV import, product search, a purchase flow, a local database, a UI, and Docker support. Backend is Go, frontend is ClojureScript.

## Tech Stack

- **Backend:** Go 1.22 (stdlib `net/http` routing), PostgreSQL 16 (`pg_trgm` for fuzzy search), `pgx/v5`, `golang-migrate`
- **Frontend:** ClojureScript, `shadow-cljs`, Reagent, re-frame, `reitit-frontend` (routing), `cljs-ajax`
- **Infra:** Docker Compose (dev / test / full profiles), nginx (production frontend)

## Prerequisites

- Go 1.22+
- Node.js 18+ and npm
- Java 11+ (required by the ClojureScript/shadow-cljs toolchain)
- Docker and Docker Compose

## Project layout

- `backend/` — Go REST API, layered as `domain` → `repository` (interfaces) → `repository/postgres` (implementations) → `service` → `api` (HTTP handlers)
- `frontend/` — ClojureScript SPA (shadow-cljs + Reagent)

## Running locally

Backend + Postgres (migrations applied automatically):
```
docker compose up --build
```

Frontend, separately, with hot reload:
```
cd frontend
npm install
npm run watch
```

Full stack instead — production frontend build served via nginx:
```
docker compose --profile full up --build
```

Backend only, without Docker:
```
cd backend
export DATABASE_URL=postgres://ecommerce:ecommerce@localhost:5434/ecommerce?sslmode=disable
go run ./cmd/api
```

Migrations (`backend/migrations/`, `golang-migrate` format) apply automatically in every Docker path above. Run by hand only when needed:
```
migrate -path backend/migrations -database "$DATABASE_URL" up
```

## Running tests

Backend (`go test ./...`, inside Docker — native `go test` crashes on this machine with a pgx/dyld linker issue unrelated to this code):
```
docker compose --profile test run --rm test
```

Frontend (`cljs.test`, event-handler logic — view rendering isn't tested):
```
cd frontend
npx shadow-cljs compile test
```

## Architecture & decisions

### Backend structure

The backend is split into layers: domain, repository, service, and API. Business logic doesn't depend on HTTP or the database directly, so the service layer can be tested against fake repositories without needing Postgres running at all. Routing is handled with Go's standard library rather than a router package, since a project this size doesn't really need one.

### Database: Postgres, raw SQL via `pgx`, `golang-migrate`

Postgres was picked mainly because it's one of the most widely used databases, handles concurrency well, and is closer to a real production setup than something like SQLite would be.

Raw SQL through `pgx` is used instead of an ORM. A few of the required features — trigram search, bulk upsert, a transactional stock decrement — get awkward through most ORMs, so hand written queries were the simpler path. It was also a chance to show direct SQL skills rather than leaning entirely on a framework.

### Product search: `pg_trgm`, and word- vs whole-string similarity

Search runs on Postgres's trigram extension (`pg_trgm`) instead of standing up a separate search engine. Matching is word-level rather than whole-string, so a short term like "Camp" still finds "Camping Chair" — a whole-string match would score that too low to count as a hit.

### CSV import: validation before it ever reaches the database

Every row is validated before it reaches the database, so bad numbers, missing fields, or malformed values get rejected on the way in rather than causing problems later. That same check is what keeps the sample CSV's SQL-injection and script-injection test values harmless — they just get stored and displayed as plain text instead of running as code.

### Testing philosophy

Tests weren't a requirement for this challenge, but a real enterprise product needs a solid test suite, especially one that's going to be worked on by a team or handed off over time. The tests here aren't exhaustive, but they cover the riskiest logic and leave a pattern to build on as new features get added.

### Frontend architecture

State lives in one shared place only when it actually needs to be shared — the cart is the clearest example, since it's used from both the product list and the cart page. Everything else, like form fields, stays local to whichever component owns it. Each page also gets a real URL (product list, new/edit product, cart) instead of one hidden view flag, so refreshing or sharing a link behaves the way people expect.

Search runs when the user submits it rather than live as they type — mostly a time-saving call for this project rather than a strong opinion either way. CSV import and delete stick to ordinary web patterns too: a real file upload for import, and an inline confirmation step before deleting a product instead of a disruptive browser popup.

### Docker / infra

Running the project day to day only starts the database and backend, with the frontend run separately so it gets fast reload during development. A second mode builds and serves the whole stack through nginx, closer to a real deployment. A third spins up a separate, disposable database just for running the backend test suite, so tests never touch real dev data.

## Known gaps

- UI was kept minimal to achieve an MVP.
- Test coverage is selective for this project, as mentioned above, for a real enterprise grade app more tests would be needed.
- The cart is frontend only and ephemeral, this is because no user/auth system is in place.
- `product-edit-page` looks up the product being edited from the already-loaded `:products` list rather than fetching it individually — correct in practice, but a genuinely invalid product id in the URL shows "Loading..." forever rather than a real not-found message.
- Since there are no users there's also no user roles, so any user can create, edit, remove and purchase products.
