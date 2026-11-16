# REST-контракты

Источник истины по REST — [`docs/api-contract.md`](../../../docs/api-contract.md) (ведёт Рома).
Отдельный `rest.yaml` не дублируем, чтобы не было двух версий правды.

## Что отдаёт AI-сервис (`ai`, порт 8000)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/healthz` | `{"status":"ok"}` |
| `GET` | `/readyz` | `{"status":"ok","llm_mode":"mock","prompt_version":"v1"}` |
| `POST` | `/api/v1/chat/stream` | Тело `{"message", "user_id"?, "role"?}` (§3.7), заголовок `Authorization: Bearer <JWT core>`, ответ — SSE ([sse.md](sse.md)) |
| `DELETE` | `/api/v1/chat/history` | Очистить историю диалога и ожидающий черновик пользователя, `204` |

Ответы `/api/v1/chat/stream` кроме SSE: `401` — нет или плохой JWT, `422` — пустое сообщение
или длиннее 4000 символов, `429` — rate limit (SSE-событие `error`).

## Что AI-сервис и MCP вызывают в core

| Инструмент | Запрос в core | Раздел контракта |
|---|---|---|
| `get_my_profile` | `GET /api/v1/me` | §3.2 |
| `get_onboarding_plan` | `GET /api/v1/onboarding/plan` | §3.3 |
| `search_knowledge` | `GET /api/v1/kb/search?q=&limit=5`, затем `GET /api/v1/kb/documents/{id}` для лучшего | §3.8, §3.9, §10.1, §10.2 |
| `preview_absence_request` | `POST /api/v1/requests/absence/preview` | §3.10, §10.3 |
| `create_absence_request` | `POST /api/v1/requests/absence` | §3.11, §10.4 |
| `get_my_requests` | `GET /api/v1/requests` | §3.12, §10.5 |
| `preview_access_bundle` | `POST /api/v1/requests/access/preview` | §3.14, §10.6 |
| `create_access_requests` | `POST /api/v1/requests/access` | §3.15, §10.6 |
| `preview_handoff` | — (черновик собирается в AI) | — |
| `create_handoff` | `POST /api/v1/handoffs` | предложение, в core пока нет |

### `POST /api/v1/handoffs` — предложение для core

Вопрос стажёра наставнику. Скоуп `request:self`, наставник — из профиля пользователя.

```json
{"question": "Какой пароль от Wi-Fi?", "context": "", "idempotency_key": "<preview_id>"}
```

Ответ `200`/`201`:

```json
{"request_id": "...", "status": "pending", "external_id": "HND-101",
 "mentor": {"full_name": "Пётр Иванов"}, "next_step": "Наставник ответит в мессенджере"}
```

Повтор с тем же `idempotency_key` возвращает ту же запись. Пока эндпоинта нет, core отвечает
`404`, агент говорит, что передать не получилось, и советует написать наставнику напрямую.

## MCP (`mcp`, порт 8001)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/healthz` | `{"status":"ok"}` |
| `POST` | `/mcp` | MCP Streamable HTTP, stateless, JSON-ответы. JWT пользователя — в `Authorization` |

Каждый инструмент MCP возвращает `{"ok": true, "result": ..., "citations": [...]}` или
`{"ok": false, "error": {"code", "message"}, "citations": []}`. JSON Schema аргументов —
в [`mcp/`](mcp): `llm_parameters` видит модель, `mcp_input_schema` принимает MCP-сервер.
Файлы генерируются из кода командой `uv run python -m evals.export_contracts`, тест
`ai/tests/test_specs.py` падает, если они разошлись с кодом.
