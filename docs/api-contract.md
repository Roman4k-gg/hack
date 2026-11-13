# Контракт API · AI-buddy

Рабочий контракт между `core` (Go) и `web` (React). Описан **по факту реализации**,
а не по архитектурному документу: если код и этот файл расходятся — чинить код
и обновлять файл в одном коммите.

| Что | Где |
|---|---|
| Роуты и сборка зависимостей | `core/internal/api/router.go`, `core/cmd/api/main.go` |
| Ошибки и коды | `core/internal/platform/errors/errors.go` |
| Скоупы и роли | `core/internal/rbac/rbac.go` |
| JWT | `core/internal/auth/jwt.go` |
| Реализация хендлеров | `core/internal/{auth,onboarding,team,chat}/` |
| HTTP-клиент фронта | `web/src/lib/api.ts` |
| Zod-схемы фронта | `web/src/features/{auth,onboarding,team}/types.ts` |
| SSE-парсер фронта | `web/src/features/chat/schemas.ts`, `web/src/lib/sse/parseSSE.ts` |

---

## 1. Общие правила

| Правило | Значение |
|---|---|
| Базовый путь | `/api/v1` |
| Core напрямую | `http://localhost:8081/api/v1` |
| Core через nginx | `http://localhost:3000/api/v1` (фронт ходит так же) |
| Формат | JSON, UTF-8, поля в `snake_case` |
| ID | UUID-строки |
| Даты | RFC3339 UTC (`2026-11-13T10:08:07Z`), даты дедлайнов — `2026-11-11T00:00:00Z` |
| Авторизация | `Authorization: Bearer <jwt>` на всех эндпоинтах, кроме логина |
| Ошибки | единый конверт, см. §4 |
| Идемпотентность | пока не используется (планируется `Idempotency-Key` для мутаций) |

Фронт никогда не ходит в базу и не строит URL руками: всё через `apiFetch()`
из `web/src/lib/api.ts`, который сам подставляет базу, `Authorization` и
`Content-Type`.

---

## 2. Кто во что ходит (карта фронт → бэк)

| Файл фронта | Эндпоинт | Когда |
|---|---|---|
| `features/auth/api.ts` → `login()` | `POST /api/v1/auth/login` | форма входа и демо-кнопки |
| `features/onboarding/api.ts` → `fetchPlan()` | `GET /api/v1/onboarding/plan` | страница `/map` |
| `features/onboarding/api.ts` → `updateItemStatus()` | `PATCH /api/v1/onboarding/items/{id}` | кнопки «В работу / Готово / Блокер» |
| `features/team/api.ts` → `fetchTeamReport()` | `GET /api/v1/team/interns` | страница `/dashboard` |
| `features/chat/hooks/useChatSession.ts` | `POST /api/v1/chat/stream` | отправка сообщения в чат |
| — (пока нет) | `GET /api/v1/me` | зарезервирован для обновления профиля |
| — (пока нет) | `GET /api/v1/team/overview` | отдельный вызов сводки, сейчас сводка приходит в `team/interns` |

---

## 3. Эндпоинты

### 3.1 `POST /api/v1/auth/login`

Mock-логин по email. Пароля нет: любой email из таблицы `users` выдаёт JWT.

**Запрос**

```json
{ "email": "petr@demo.local" }
```

**Ответ `200`**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_at": "2026-11-13T18:44:00.749208339Z",
  "user": {
    "id": "22222222-2222-2222-2222-222222222221",
    "email": "petr@demo.local",
    "full_name": "Пётр Иванов",
    "role": "mentor",
    "position": "Ведущий разработчик",
    "team_id": "11111111-1111-1111-1111-111111111111",
    "team_name": "Search",
    "team_slug": "search",
    "scopes": [
      "onboarding:plan:read", "onboarding:plan:write", "chat:use",
      "kb:read", "onboarding:team:read", "analytics:team"
    ]
  }
}
```

**Ошибки:** `400 VALIDATION_ERROR` (пустой или без `@`), `401 UNAUTHORIZED`
(пользователя нет).

> Фронт: `loginResponseSchema` (`features/auth/types.ts`). Поле `scopes`
> обязательное — на нём построено решение «показывать ли раздел Команда».

---

### 3.2 `GET /api/v1/me`

Профиль текущего пользователя. **Скоуп:** любой валидный токен.

**Ответ `200`** — тот же объект `user`, что и в логине.

**Ошибки:** `401 UNAUTHORIZED` (нет или битый токен).

---

### 3.3 `GET /api/v1/onboarding/plan`

Карта адаптации текущего стажёра из PostgreSQL.

**Скоуп:** `onboarding:plan:read`
**Кто вызывает:** страница `/map` (`features/onboarding/api.ts`)

**Ответ `200`**

```json
{
  "id": "33333333-3333-3333-3333-333333333331",
  "title": "Адаптация в команде Search",
  "starts_on": "2026-11-10T00:00:00Z",
  "stages": [
    {
      "id": "33333333-3333-3333-3333-333333333341",
      "title": "Неделя 1 · Окружение",
      "description": "Доступы, ноутбук и рабочее окружение.",
      "position": 1,
      "items": [
        {
          "id": "44444444-4444-4444-4444-444444444401",
          "stage_id": "33333333-3333-3333-3333-333333333341",
          "title": "Получить ноутбук и доступ к VPN",
          "description": "Выдаёт IT-отдел, доступ приходит на рабочую почту.",
          "kind": "access",
          "status": "done",
          "due_on": "2026-11-11T00:00:00Z",
          "position": 1,
          "completed_at": "2026-11-13T07:53:07.712389Z"
        }
      ]
    }
  ],
  "progress": { "done": 4, "total": 10, "percent": 40 },
  "next_step": {
    "item_id": "44444444-4444-4444-4444-444444444403",
    "title": "Получить доступ к Jira",
    "reason": "Обязательный пункт этапа «Неделя 1 · Окружение»"
  },
  "intern": {
    "id": "22222222-2222-2222-2222-222222222224",
    "full_name": "Дарья Соколова",
    "team_name": "Search",
    "position": "Стажёр-разработчик"
  },
  "updated_at": "2026-11-13T10:08:07.865796Z"
}
```

**Поля, на которые смотрит фронт** (`planSchema`):

| Поле | Тип | Комментарий |
|---|---|---|
| `intern.id` / `full_name` / `team_name` | string | обязательные |
| `stages[].id/title/description/position` | string / number | `position` — порядок этапа |
| `stages[].items[].kind` | `task \| doc \| meeting \| access` | enum, при несовпадении Zod упадёт |
| `stages[].items[].status` | `pending \| in_progress \| done \| blocked \| skipped` | enum, совпадает с CHECK в БД |
| `stages[].items[].blocked_reason` | string \| null | отсутствует, если пусто |
| `progress` | `{done,total,percent}` | `percent` — целое, `done/total*100` |
| `next_step` | `{item_id,title,reason}` \| null | первый незакрытый пункт |

**Ошибки:** `404 NOT_FOUND` (плана нет), `403 FORBIDDEN` (нет скоупа).

---

### 3.4 `PATCH /api/v1/onboarding/items/{id}`

Смена статуса пункта. Пишет в `audit_log` (`onboarding.item_updated`)
и в `events` (`onboarding_item_updated`).

**Скоуп:** `onboarding:plan:write` + владение пунктом (чужой пункт → 403)

**Запрос**

```json
{ "status": "in_progress", "blocked_reason": "" }
```

| Поле | Обязательное | Значения |
|---|---|---|
| `status` | да | `pending \| in_progress \| done \| blocked \| skipped` |
| `blocked_reason` | только для `blocked` | строка, иначе `400 VALIDATION_ERROR` |

**Ответ `200`** — обновлённый пункт (тот же объект, что `items[]` в плане).

**Ошибки:** `400 VALIDATION_ERROR` (неизвестный статус / нет причины блокировки),
`403 FORBIDDEN` (чужой пункт), `404 NOT_FOUND`.

**Как это работает на фронте:** `updateItemStatus()` → оптимистичная правка
кэша react-query (`applyStatus`) → `onError` откатывает кэш, `onSettled`
перезапрашивает план.

---

### 3.5 `GET /api/v1/team/interns`

Список стажёров с метриками + сводка по команде. Это то, что показывает
дашборд наставника.

**Скоуп:** `onboarding:team:read`
**Кто вызывает:** `features/team/api.ts` → `fetchTeamReport()`

**Видимость данных:**

| Роль | Кого видно |
|---|---|
| `mentor` | стажёров своей команды и своих подопечных |
| `manager` | свою команду |
| `hr`, `admin` | всех |

**Ответ `200`**

```json
{
  "interns": [
    {
      "id": "22222222-2222-2222-2222-222222222224",
      "full_name": "Дарья Соколова",
      "email": "dasha@demo.local",
      "position": "Стажёр-разработчик",
      "team": { "id": "11111111-...", "name": "Search", "slug": "search" },
      "mentor": { "id": "22222222-...", "full_name": "Пётр Иванов" },
      "plan": {
        "id": "33333333-...",
        "title": "Адаптация в команде Search",
        "starts_on": "2026-11-10",
        "current_stage": "Неделя 1 · Окружение"
      },
      "progress": {
        "done": 4, "total": 10, "pending": 3,
        "in_progress": 2, "blocked": 1, "skipped": 0, "percent": 40
      },
      "pace": { "expected": 3, "completed": 3, "percent": 100 },
      "blockers": [
        {
          "item_id": "44444444-4444-4444-4444-444444444407",
          "title": "Пройти курс по безопасности",
          "reason": "Ждём подтверждения от HR"
        }
      ],
      "overdue": [
        {
          "item_id": "44444444-...",
          "title": "Пройти курс по безопасности",
          "due_on": "2026-11-10",
          "days_overdue": 3
        }
      ],
      "activity": { "questions": 3, "last_event_at": "2026-11-13T10:08:14Z" },
      "at_risk": true
    }
  ],
  "overview": {
    "interns": 6,
    "avg_progress": 25,
    "avg_pace": 55,
    "blocked": 2,
    "overdue": 5,
    "at_risk": 3,
    "teams": [
      {
        "id": "11111111-...", "name": "Search", "interns": 3,
        "avg_progress": 33, "avg_pace": 66, "blocked": 1, "overdue": 0
      }
    ]
  }
}
```

**Ошибки:** `403 FORBIDDEN` (стажёр), `401 UNAUTHORIZED`.

---

### 3.6 `GET /api/v1/team/overview`

Только сводка, без списка стажёров. **Скоуп:** `analytics:team`.
Формат — объект `overview` из ответа §3.5.

---

### 3.7 `POST /api/v1/chat/stream`

SSE-прокси к AI-сервису. Клиентский JWT пробрасывается апстриму.

**Скоуп:** `chat:use` + rate limit по пользователю
**Кто вызывает:** `features/chat/hooks/useChatSession.ts`

**Запрос**

```json
{ "message": "где взять доступ к Jira?" }
```

**Ответ `200`** — SSE-поток, см. §5.

**Ошибки (не SSE, а JSON-конверт):**

| Статус | Код | Когда |
|---|---|---|
| `400` | `VALIDATION_ERROR` | пустое сообщение |
| `429` | `SERVICE_UNAVAILABLE` | сработал rate limit |
| `502` | `UPSTREAM_ERROR` | AI-сервис ответил ошибкой |
| `503` | `SERVICE_UNAVAILABLE` | AI-сервис недоступен |
| `504` | `TIMEOUT` | AI-сервис не успел ответить за `CHAT_TIMEOUT` |

Фронт при `503`/`504`/`502` автоматически переключается на демо-поток и
показывает плашку «AI-сервис недоступен — показан демо-режим».

**Что core отдаёт апстриму** (AI-сервис должен принять ровно это):

```json
{
  "message": "где взять доступ к Jira?",
  "user_id": "22222222-2222-2222-2222-222222222224",
  "role": "intern"
}
```

---

## 4. Ошибки

Единый конверт (`core/internal/platform/errors/errors.go`):

```json
{
  "error": {
    "code": "FORBIDDEN",
    "message": "Недостаточно прав для этого действия",
    "details": { "role": "intern", "scope": "onboarding:team:read" },
    "trace_id": "621e7447-5b23-4e6b-8d37-429c9750604d"
  }
}
```

`details` — необязательный объект, `trace_id` есть всегда (его же видно в
логах core, по нему можно искать запрос).

| Код | HTTP | Когда |
|---|---|---|
| `VALIDATION_ERROR` | 400 | плохой формат тела, неизвестный статус, нет причины блокировки |
| `UNAUTHORIZED` | 401 | нет токена, токен битый или истёк |
| `FORBIDDEN` | 403 | нет скоупа или нельзя трогать чужой объект |
| `NOT_FOUND` | 404 | нет плана, пункта, пользователя |
| `CONFLICT` | 409 | пока не используется |
| `UPSTREAM_ERROR` | 502 | ошибка AI-сервиса |
| `TIMEOUT` | 504 | AI-сервис не ответил вовремя |
| `SERVICE_UNAVAILABLE` | 429 / 503 | rate limit / сервис недоступен |
| `INTERNAL_ERROR` | 500 | всё остальное |

Сообщения — на русском, для пользователя. Технические детали — в `details`
и в логах.

---

## 5. SSE-контракт чата

Формат зафиксирован фронтом (`web/src/features/chat/schemas.ts`).
AI-сервис обязан отдавать `event: message` и `data` с полем `type`.

| `type` | `data` | Что делает фронт |
|---|---|---|
| `delta` | `{ "type": "delta", "text": "Твой следующий шаг — " }` | дописывает текст в последнее сообщение |
| `citation` | `{ "type": "citation", "citation": { "id", "title", "url"? } }` | добавляет чип-цитату |
| `tool_call` | `{ "type": "tool_call", "toolCall": { "id", "name", "status": "running\|success\|error" } }` | карточка вызова инструмента |
| `done` | `{ "type": "done" }` | стрим закончен, статус → idle |
| `error` | `{ "type": "error", "message": "..." }` | ошибка прямо в потоке |

Пример потока:

```
event: message
data: {"type":"delta","text":"Чтобы получить доступ к Jira, "}

event: message
data: {"type":"tool_call","toolCall":{"id":"call-1","name":"get_onboarding_plan","status":"success"}}

event: message
data: {"type":"citation","citation":{"id":"jira-access","title":"Как получить доступ к Jira","url":"https://demo.local/docs/jira-access"}}

event: message
data: {"type":"done"}
```

Правила:

- core проксирует события **как есть**, без разбора — значит формат должен
  совпадать с фронтовым парсером;
- `event: message` — имя события; допускается короткая форма без `type` в data
  (парсер сам подставит `type` из имени события);
- парсер умеет `\n` и `\r\n`, несколько `data:`-строк склеиваются;
- пустые строки между событиями обязательны (это разделитель SSE).

---

## 6. JWT и скоупы

**Claims** (`core/internal/auth/jwt.go`):

| Claim | Что |
|---|---|
| `sub` | id пользователя |
| `iss` | `ai-buddy-core` (или `JWT_ISSUER`) |
| `iat`, `exp` | выдача и истечение |
| `email` | почта |
| `role` | `intern \| mentor \| manager \| hr \| admin` |
| `team_id` | команда пользователя |
| `scopes` | список прав |

**Скоупы по ролям** (`core/internal/rbac/rbac.go`):

| Скоуп | intern | mentor | manager | hr | admin |
|---|:-:|:-:|:-:|:-:|:-:|
| `onboarding:plan:read` | ✅ | ✅ | ✅ | ✅ | ✅ (`*`) |
| `onboarding:plan:write` | ✅ | ✅ | ✅ | — | ✅ |
| `chat:use` | ✅ | ✅ | ✅ | — | ✅ |
| `kb:read` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `onboarding:team:read` | — | ✅ | ✅ | ✅ | ✅ |
| `analytics:team` | — | ✅ | ✅ | ✅ | ✅ |
| `requests:read` / `requests:write` | — | — | ✅ | ✅ | ✅ |

`admin` получает `*` — проходит любую проверку `rbac.RequireScopes`.

Проверка прав — **два уровня**:

1. `rbac.RequireScopes(...)` на роуте — есть ли скоуп вообще;
2. object-level в сервисе — можно ли трогать именно этот объект
   (например, `onboarding` не даёт менять чужой пункт).

---

## 7. Как считаются метрики

Реализация — `core/internal/team/service.go`.

| Метрика | Формула |
|---|---|
| `progress.percent` | `done / total * 100` (целое) |
| `progress.*` | разбивка пунктов по статусам: `pending`, `in_progress`, `blocked`, `skipped` |
| `pace.percent` | `completed / expected * 100`, где `expected` — пункты с `due_on <= сегодня`, `completed` — из них закрытые. Если дедлайнов ещё не было — берётся `progress.percent` |
| `overdue[]` | пункты с `due_on < сегодня` и статусом не `done`/`skipped`; `days_overdue` — разница в днях |
| `blockers[]` | пункты со статусом `blocked`, с причиной из `blocked_reason` |
| `at_risk` | `true`, если есть блокеры **или** просроченные пункты |
| `plan.current_stage` | этап первого незакрытого пункта, иначе последний этап |
| `activity.questions` | количество событий `chat%` в `events` по пользователю |
| `activity.last_event_at` | последнее событие пользователя |
| `overview.avg_progress`, `avg_pace` | средние по видимым стажёрам |
| `overview.at_risk` | количество стажёров с `at_risk = true` |

---

## 8. Проверка контракта руками

```bash
# токен
TOKEN=$(curl -s -X POST http://localhost:8081/api/v1/auth/login \
  -H 'Content-Type: application/json' -d '{"email":"petr@demo.local"}' \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')

# профиль и скоупы
curl -s http://localhost:8081/api/v1/me -H "Authorization: Bearer $TOKEN"

# карта стажёра
curl -s http://localhost:8081/api/v1/onboarding/plan -H "Authorization: Bearer $TOKEN"

# метрики команды (наставник)
curl -s http://localhost:8081/api/v1/team/interns -H "Authorization: Bearer $TOKEN"

# 403 для стажёра
DTOKEN=$(curl -s -X POST http://localhost:8081/api/v1/auth/login \
  -H 'Content-Type: application/json' -d '{"email":"dasha@demo.local"}' \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')
curl -s -o /dev/null -w '%{http_code}\n' \
  http://localhost:8081/api/v1/team/interns -H "Authorization: Bearer $DTOKEN"   # 403

# смена статуса пункта
curl -s -X PATCH http://localhost:8081/api/v1/onboarding/items/<item-id> \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":"done"}'
```

Полный прогон стенда: `make smoke` (14 проверок, включая метрики команды и 403).

---

## 9. Если меняешь контракт

1. Меняешь хендлер в `core/internal/...` **и** Zod-схему в
   `web/src/features/.../types.ts` в одном коммите — иначе фронт упадёт на парсинге.
2. Обновляешь этот файл в том же коммите.
3. Добавляешь или правишь тест: Go — `core/internal/<pkg>/*_test.go`,
   фронт — Zod-схема + `make smoke`.
4. SSE-схему меняешь только в `web/src/features/chat/schemas.ts` и в этом файле,
   плюс синхронно с AI-сервисом (T-04).

---

## 10. Чего в контракте ещё нет

| Эндпоинт | Статус |
|---|---|---|
| `GET /api/v1/kb/search` | не реализован |
| `POST /api/v1/requests/absence/preview` + `/requests/absence` |
| `POST /api/v1/requests/access/preview` + `/requests/access` |
| `GET /api/v1/notifications`, `/handoffs`, `/admin/*` |
| MCP-инструменты | не реализованы |

До их появления в этом файле будет пусто — добавлять по мере реализации.
