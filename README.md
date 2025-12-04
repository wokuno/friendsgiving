# Friendsgiving Menu Service

A lightweight Go backend that keeps track of who is contributing what dish to the Friendsgiving meal and broadcasts menu updates over Server-Sent Events. It seeds a simple `menu.json` file, serves the menu via REST, and exposes a push channel for live UI clients.

## Features

- Persistent menu storage backed by `menu.json` (created with a basic default menu if missing).
- CRUD-friendly REST endpoints for listing, adding, and removing dishes.
- SSE stream at `/api/menu/stream` so frontends can react instantly to menu changes.
- Static file serving from the `src/` directory, so you can pair any client assets with the API in one process.

## Getting started

### Prerequisites

- Go 1.25.2 or newer (the module is already configured for that version).

### Running locally

```bash
cd /Users/wokuno/Desktop/friendsgiving
go run ./src/main.go
```

The server listens on `http://localhost:8080`, serves static assets from `src/`, and exposes the API under `/api/menu`.

## API reference

### `GET /api/menu`
Returns the current menu as JSON.

### `POST /api/menu`
Adds a dish. Request body example:

```json
{
  "dish": "Brussels Sprouts",
  "who": "Jamie"
}
```

Responds with the updated menu and a `201 Created` status.

### `DELETE /api/menu?id={menuItemID}`
Deletes the menu item with the provided `id` query parameter.

### `GET /api/menu/stream`
Opens a Server-Sent Events stream. Every menu change (add/delete) sends an event named `menu` with the updated menu JSON, so UI clients stay in sync.

## Testing

```bash
go test ./...
```

Tests live under `test/` and ensure that adding or removing an item not only persists the change but also broadcasts it to connected observers.

## Project layout

- `/src/main.go` wires the HTTP server to the `app` package.
- `/src/app/app.go` implements the menu logic, persistence helpers, and SSE broadcaster.
- `/src/index.html` is the client entry point that connects to the API and streams menu updates.
- `/menu.json` is the persisted menu file; the app bootstraps it with a default list when missing.
- `/test` contains a few helper tests that exercise the broadcast flows.

## Notes

- You can point a frontend to `http://localhost:8080/api/menu/stream` to receive live updates whenever guests add or remove dishes.
- The server writes the menu in pretty-printed JSON so it stays human readable if you want to edit it manually.

## License

This project is distributed under the terms of the MIT License. See `LICENSE` for details.
