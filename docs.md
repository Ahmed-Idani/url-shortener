# Backend API

## Run locally

```bash
cp .env.example .env.dev   # then set ADMIN_TOKEN
go run .                   # needs cgo (go-sqlite3)
```

The dashboard is at `http://localhost:3333/admin/`. Production reads `.env.prod` instead: `APP_ENV=prod ./url-shortener`.

## Overview

Every backend endpoint lives under `/api`:

- `/api/public/...` needs no credentials (login and logout).
- `/api/private/...` is admin only. Every route under it is protected.

Short links are served from the root (`GET /{slug}`) and the dashboard pages from `/admin/`.

## Authentication

Every `/api/private` endpoint needs one of:

| Client | How | Responses |
|---|---|---|
| Scripts, curl | `Authorization: Bearer <ADMIN_TOKEN>` header | JSON |
| Dashboard (browser) | `session` cookie set by `POST /api/public/login` | Forms redirect back to the dashboard |

`ADMIN_TOKEN` is the value you set in `.env.dev` (local) or `.env.prod` (production).

Without valid credentials, API clients get `401 {"error": "unauthorized"}` (for any path under `/api/private`, existing or not) and browsers are redirected to `/admin/login`.

Errors are always JSON: `{"error": "message"}`.

## Private endpoints

### `POST /api/private/links` - create a short link

```bash
curl -H "Authorization: Bearer $TOKEN" \
  -d '{"url": "https://github.com", "name": "gh"}' \
  http://localhost:3333/api/private/links
```

| Field | Required | Rules |
|---|---|---|
| `url` | yes | a valid `http` or `https` URL |
| `name` | no | 1-64 letters, digits, `-` or `_`; random 6 characters if empty; `admin` and `api` are reserved |

`201` returns the link:

```json
{
  "id": 3,
  "name": "gh",
  "url": "https://github.com",
  "redirect_url": "http://localhost:3333/gh",
  "created_at": "2026-10-03 22:20:42",
  "clicks": 0,
  "bot_clicks": 0
}
```

Errors: `400` invalid JSON, URL or name · `409` name already taken.

### `GET /api/private/links` - list links

Newest first, each with its total `clicks` and `bot_clicks`.

```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:3333/api/private/links
```

### `GET /api/private/links/{name}/stats` - link statistics

```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:3333/api/private/links/gh/stats
```

```json
{
  "link": { "id": 3, "name": "gh", "...": "..." },
  "by_day": [{ "key": "2026-10-03", "clicks": 12, "bots": 3 }],
  "by_country": [{ "key": "TN", "clicks": 9, "bots": 1 }]
}
```

Days are UTC dates. `404` if the link doesn't exist.

### `DELETE /api/private/links/{name}` - delete a link

Deletes the link and all its clicks. `204` on success, `404` if not found.

```bash
curl -X DELETE -H "Authorization: Bearer $TOKEN" http://localhost:3333/api/private/links/gh
```

## Dashboard endpoints

These are what the dashboard's HTML forms post to. They take form fields and answer with redirects.

| Endpoint | Form fields | Result |
|---|---|---|
| `POST /api/public/login` | `token` | Sets the `session` cookie (30 days) and redirects to `/admin/`. A wrong token shows the login page again (`401`). |
| `POST /api/public/logout` | - | Clears the cookie and redirects to `/admin/login`. |
| `POST /api/private/links` | `url`, `name` | Same as the JSON version (sent with the cookie instead of the token), then redirects to `/admin/?created=<name>`. |
| `POST /api/private/links/{name}/delete` | - | Same as `DELETE`, for HTML forms. Redirects to `/admin/`. |

## Short links

`GET /{slug}` redirects to the link's URL with `301 Moved Permanently` and records a click with:
- the time, the visitor's IP and their country
- browser, OS and device type
- the referring site
- whether it looks like a bot

Unknown slugs return `404`.

The redirect is sent with `Cache-Control: no-store`. Browsers normally cache a 301 and skip the server on later visits, which would lose those clicks. `no-store` makes them come back every time, so every click is counted.
