# Diamonds Academy

Go + SQLite + templ + htmx + Tailwind ile yazılmış, 3 panelli (frontend / admin / REST API) eğitim platformu iskeleti.

## Stack

- **Go 1.26**, `chi` router
- **SQLite** (`modernc.org/sqlite`, CGO yok), `golang-migrate` (embed)
- **templ** type-safe component templating + `htmx`
- **scs** session yönetimi (sqlite store) + **bcrypt**
- **slog** structured logging
- **air** hot reload

## Hızlı başlangıç

```bash
cp .env.example .env
export PATH="$(go env GOPATH)/bin:$PATH"   # templ + air için (bir kez)

make dev      # hot reload
# veya
make run      # tek sefer
```

Aç: http://127.0.0.1:8080

- `/`            → Dashboard (login **zorunlu değil**, herkes açabilir)
- `/login`       → Giriş
- `/admin`       → Yönetim paneli (sadece admin)
- `/api/v1/health` → JSON health

## Mimari

```
cmd/server          → entrypoint, graceful shutdown
internal/
  config           → env + .env yükleme
  logger           → slog
  db               → sqlite open + embed migrations
  auth             → user CRUD, bcrypt, EnsureAdmin
  session          → scs + sqlite3 store
  middleware       → Logger, RequireAuth, RequireAdmin
  server           → chi router (frontend / admin / api)
  handlers/
    frontend       → dashboard + login/logout
    admin          → kullanıcı listesi
    api            → REST (v1)
  views/
    layouts        → Base + DiamondLogo
    components     → Sidebar
    pages          → Dashboard, Login, Admin
web/static         → statik (eski tasarım: legacy-index.html)
```

## Komutlar

| Komut         | Açıklama                                |
| ------------- | --------------------------------------- |
| `make dev`    | `templ generate --watch` + `air`        |
| `make run`    | Tek seferlik çalıştır                   |
| `make templ`  | `.templ` → `_templ.go` üret             |
| `make build`  | Prod binary (`bin/diamonds`)            |
| `make tidy`   | `go mod tidy`                           |
| `make clean`  | `bin/`, `tmp/`, generated `_templ.go`   |

## Login akışı (şu an)

- Dashboard public.
- `/login` → form → `bcrypt` doğrulama → session → admin ise `/admin`, değilse `/`.
- `EnsureAdmin` ilk açılışta `.env` içindeki `ADMIN_EMAIL` / `ADMIN_PASSWORD` ile seed admin oluşturur (idempotent).
- Sonradan dashboard'u kapatmak için `web.Get("/")` rotasını `RequireAuth` group'una taşımak yeterli.

## Migration ekleme

```bash
# yeni migration: internal/db/migrations/0002_xxx.up.sql + .down.sql
# migrate, embed FS üzerinden up'ı otomatik koşar.
```
# diamondacademy
