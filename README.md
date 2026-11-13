# AI-buddy

ИИ-наставник для стажёров (кейс Авито, хакатон Hack&Change 2026).

Стажёр видит карту адаптации, задаёт вопросы в чате и получает потоковый ответ с
карточками вызовов MCP-инструментов. Core отвечает за авторизацию, карту
адаптации, чат-прокси и аудит.

## Стек

| Слой        | Технологии                                   |
|-------------|----------------------------------------------|
| `core`      | Go 1.23, chi, pgx, go-redis, JWT (HS256)      |
| `ai`        | FastAPI, LiteLLM, FastMCP (в разработке)     |
| `web`       | React + TypeScript + Vite + Tailwind (в разработке) |
| Инфра       | Docker Compose, PostgreSQL 16 + pgvector, Redis 7, nginx, Prometheus, Grafana |

## Структура

```
core/                     Go-сервис
  cmd/api                 точка входа HTTP API
  cmd/seed                миграции и демо-данные
  migrations/             SQL-миграции (встраиваются в бинарник)
  internal/api            сборка роутов /api/v1
  internal/auth           mock-логин, JWT, middleware авторизации
  internal/rbac           роли, скоупы, middleware доступа
  internal/onboarding     карта адаптации: план, этапы, пункты
  internal/chat           SSE-прокси к AI-сервису
  internal/audit          запись в audit_log и events
  internal/platform/      config, db, redis, logging, errors, reqctx, httpserver, ratelimit
web/                      фронтенд (Vite + React), Dockerfile для сборки в nginx
ai/, mocks/               сервисы AI и моки (в разработке)
infra/                    nginx, prometheus, grafana
scripts/smoke.sh          сквозная проверка стенда
make.ps1                  PowerShell-обёртка над Makefile для Windows
```

## Быстрый старт

```bash
cp .env.example .env
make up          # postgres, redis, core
make seed        # миграции + демо-данные
```

Проверка:

```bash
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

Логин и профиль:

```bash
TOKEN=$(curl -s -X POST http://localhost:8081/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"dasha@demo.local"}' | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')

curl -s http://localhost:8081/api/v1/me -H "Authorization: Bearer $TOKEN"
curl -s http://localhost:8081/api/v1/onboarding/plan -H "Authorization: Bearer $TOKEN"
```

Полный сброс демо-стенда:

```bash
make demo-reset
```

## Как проверить, что всё работает

Одна команда проверяет весь стенд (healthchecks, логин, профиль, карту адаптации,
PATCH пункта, SSE, аудит и фронтенд):

```bash
make up      # postgres, redis, core
make seed    # миграции + демо-данные
make smoke   # 12 проверок, зелёный вывод
```

`make smoke` печатает `OK` / `FAIL` / `SKIP` по каждой проверке и возвращает
ненулевой код, если что-то упало. Недоступный AI-сервис считается `SKIP`, а не
ошибкой.

Поднять фронтенд рядом с бэкендом:

```bash
docker compose --profile dev up -d --wait web
# http://localhost:3000 — приложение, /api проксируется в core
```

Фронтенд локально (нужны Node 22+ и npm):

```bash
cd web && npm ci && npm run dev
# http://localhost:5173, запросы /api проксируются на http://localhost:8081
```

Отдельные проверки вручную:

```bash
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
curl -X POST http://localhost:3000/api/v1/auth/login \
  -H 'Content-Type: application/json' -d '{"email":"dasha@demo.local"}'
```

## Документация

- [`docs/api-contract.md`](docs/api-contract.md) — контракт между бэкендом и
  фронтендом: что принимает core, что ждёт web, JWT и скоупы, формат ошибок,
  SSE-события, формулы метрик.

## Демо-аккаунты

Пароля нет: mock-логин по email, любой пользователь из базы получает JWT.

| Email               | Роль     | Команда   | Наставник    |
|---------------------|----------|-----------|--------------|
| `dasha@demo.local`  | intern   | Search    | Пётр Иванов  |
| `masha@demo.local`  | intern   | Ads       | Игорь Белов  |
| `timur@demo.local`  | intern   | Search    | Пётр Иванов  |
| `vera@demo.local`   | intern   | Ads       | Игорь Белов  |
| `sonya@demo.local`  | intern   | Platform  | —             |
| `kirill@demo.local` | intern   | Search    | Пётр Иванов  |
| `petr@demo.local`   | mentor   | Search    | —             |
| `igor@demo.local`   | mentor   | Ads       | —             |
| `olga@demo.local`   | manager  | Search    | —             |
| `marina@demo.local` | hr       | —         | —             |
| `admin@demo.local`  | admin    | Platform  | —             |

Интерфейс разный по роли: стажёр после входа попадает на свою карту адаптации,
наставник, руководитель, HR и администратор — на дашборд команды с метриками.

## API

Все ответы с ошибками приходят в едином конвертере:
`{"error":{"code":"...","message":"...","trace_id":"..."}}`.

| Метод | Путь                              | Доступ                    | Описание                              |
|-------|-----------------------------------|---------------------------|---------------------------------------|
| POST  | `/api/v1/auth/login`              | открыт                    | mock-логин, возвращает JWT            |
| GET   | `/api/v1/me`                     | `Authorization: Bearer`  | профиль текущего пользователя         |
| GET   | `/api/v1/onboarding/plan`        | скоуп `onboarding:plan:read`  | этапы, пункты, прогресс, `next_step` |
| PATCH | `/api/v1/onboarding/items/{id}`  | скоуп `onboarding:plan:write` | смена статуса пункта             |
| GET   | `/api/v1/team/interns`           | скоуп `onboarding:team:read` | стажёры с метриками и сводка       |
| GET   | `/api/v1/team/overview`          | скоуп `analytics:team`   | сводные метрики по команде            |
| POST  | `/api/v1/chat/stream`            | скоуп `chat:use`          | SSE-прокси к AI-сервису               |

Смена статуса пункта:

```bash
curl -s -X PATCH http://localhost:8081/api/v1/onboarding/items/<item-id> \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"status":"done"}'
```

Статусы: `pending`, `in_progress`, `done`, `blocked`, `skipped`.
Для `blocked` обязателен `blocked_reason`.

Чат (поток SSE от AI-сервиса):

```bash
curl -N -X POST http://localhost:8081/api/v1/chat/stream \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"message":"где взять доступ к Jira?"}'
```

Core проксирует запрос на `AI_URL + AI_CHAT_PATH` (по умолчанию
`http://ai:8000/api/v1/chat/stream`), пересылает клиентский JWT и отдаёт
события как есть. Ошибки апстрима: `502` (upstream error), `504` (таймаут),
`429` (rate limit).

Запрос к AI-сервису:

```json
{"message": "где взять доступ к Jira?", "user_id": "<uuid>", "role": "intern"}
```

Ответ — SSE, формат событий совпадает с парсером фронтенда
(`web/src/features/chat/schemas.ts`):

```
event: message
data: {"type": "delta", "text": "Чтобы получить доступ к Jira, ..."}

event: message
data: {"type": "citation", "citation": {"id": "jira-access", "title": "Как получить доступ к Jira", "url": "https://demo.local/docs/jira-access"}}

event: message
data: {"type": "tool_call", "toolCall": {"id": "call-1", "name": "get_onboarding_plan", "status": "success"}}

event: done
data: {}
```

Ошибка апстрима приходит на клиент как `502`/`504`/`503` в JSON-конвертере,
а не как SSE-событие.


### Роли и скоупы

| Роль    | Скоупы                                                        |
|---------|---------------------------------------------------------------|
| intern  | `onboarding:plan:read`, `onboarding:plan:write`, `chat:use`, `kb:read` |
| mentor  | как intern                                                     |
| hr      | `onboarding:plan:read`, `kb:read`, `requests:read`, `requests:write` |
| admin   | `*`                                                            |

Свой пункт может менять владелец плана, а также наставник и администратор.

## Порты

| Порт | Сервис           |
|------|------------------|
| 8081 | core (API)       |
| 5432 | PostgreSQL       |
| 6379 | Redis            |
| 8000 | ai               |
| 8001 | mcp              |
| 3000 | web              |
| 8090 | mocks            |
| 9090 | Prometheus       |
| 3001 | Grafana          |

## Переменные окружения

Полный список — в `.env.example`. Основные:

| Переменная       | По умолчанию                | Назначение                       |
|------------------|-----------------------------|----------------------------------|
| `HTTP_ADDR`      | `:8081`                     | адрес core                       |
| `POSTGRES_DSN`   | —                           | обязательное, DSN к PostgreSQL   |
| `REDIS_ADDR`     | `localhost:6379`            | Redis                            |
| `JWT_SECRET`     | `dev-secret-change-me`      | ключ подписи JWT (обязательно менять в `real`) |
| `JWT_TTL`        | `8h`                        | время жизни токена               |
| `JWT_ISSUER`     | `ai-buddy-core`             | issuer в JWT                     |
| `AI_URL`         | `http://ai:8000`            | адрес AI-сервиса                 |
| `AI_CHAT_PATH`   | `/api/v1/chat/stream`       | путь SSE-эндпоинта AI            |
| `CHAT_TIMEOUT`   | `120s`                      | таймаут чат-прокси               |
| `RATE_LIMIT_RPS` | `5`                         | запросов в секунду на пользователя |
| `SEED_DEMO`      | `true`                      | демо-данные при старте seed      |

## Локальная разработка

```bash
cd core
POSTGRES_DSN=postgres://buddy:buddy@localhost:5432/buddy?sslmode=disable \
REDIS_ADDR=localhost:6379 \
go run ./cmd/api
```

Миграции применяются автоматически при старте `core` и при запуске `seed`.

## Тесты и покрытие

```bash
make test-core     # go test ./...
make coverage      # гонки + coverage.out + coverage.html
make lint          # go vet + golangci-lint
make fmt           # gofmt
```

Интеграционные тесты (нужна база) включаются переменной:

```bash
POSTGRES_TEST_DSN=postgres://buddy:buddy@localhost:5432/buddy?sslmode=disable \
  go test ./... -race
```

## CI

`.github/workflows/ci.yml`: форматирование, `go vet`, тесты с `-race` и покрытием,
интеграционные тесты с PostgreSQL, сборка Docker-образа. Задачи Python (`ai`) и
Web включаются автоматически, когда появились соответствующие каталоги.

## Windows

Если нет GNU Make, используйте PowerShell-обёртку:

```powershell
pwsh .\make.ps1 up
pwsh .\make.ps1 seed
pwsh .\make.ps1 test-core
```

## Troubleshooting

**`core` не стартует: `POSTGRES_DSN is required`** — не создан `.env` или не
экспортирована переменная. Скопируйте `.env.example` в `.env`.

**`readyz` отвечает `unavailable`** — проверьте, что PostgreSQL и Redis healthy:
`docker compose ps`. Логи: `docker compose logs core`.

**`login` возвращает 401 «Пользователь не найден»** — база пуста, нужен
`make seed` (или `docker compose --profile demo run --rm seed`).

**`plan` возвращает 404** — у пользователя нет плана адаптации. Демо-план
создаётся seed-ом для `dasha@demo.local`.

**`chat/stream` возвращает 502/504** — AI-сервис не поднят или отвечает дольше
`CHAT_TIMEOUT`. Проверьте `AI_URL` и `docker compose logs ai`.

**`429` на чате** — сработал `RATE_LIMIT_RPS` (по умолчанию 5 запросов в секунду
на пользователя).

**Порт занят** — поменяйте значения в `.env` (`CORE_PORT`, `POSTGRES_PORT`,
`REDIS_PORT`).

## Статус плана

Актуальный статус задач дня 1 — в `plan_day1.md`.
