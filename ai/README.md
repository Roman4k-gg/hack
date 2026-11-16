# AI-buddy · ai

Два сервиса из одного образа:

- **ai** (порт 8000) — агентный цикл: принимает сообщение из чата, вызывает LLM и инструменты, стримит ответ в SSE;
- **mcp** (порт 8001) — MCP-сервер с инструментами, которые ходят в core с JWT пользователя.

```
web → core → ai ──MCP──► mcp ──REST + JWT──► core
              └──► LLM (mock | OpenAI-совместимая)
```

## Запуск локально

```bash
cd ai
uv sync
uv run python -m app.mcp
uv run uvicorn app.main:app --reload --port 8000
```

Без запущенного MCP-сервера ai сам переключается на прямые вызовы core.

## Эндпоинты

| Сервис | Метод | Путь | Что |
|---|---|---|---|
| ai | GET | `/healthz` | liveness |
| ai | GET | `/readyz` | готовность, режим LLM, версия промпта |
| ai | POST | `/api/v1/chat/stream` | SSE-ответ чата, формат — `docs/api-contract.md` §5 |
| mcp | GET | `/healthz` | liveness |
| mcp | POST | `/mcp` | MCP Streamable HTTP |

## Инструменты

| Tool | Core | Скоуп |
|---|---|---|
| `get_my_profile` | `GET /api/v1/me` | — |
| `get_onboarding_plan` | `GET /api/v1/onboarding/plan` | `onboarding:plan:read` |
| `search_knowledge` | `GET /api/v1/kb/search` | `kb:read` |
| `preview_absence_request` | `POST /api/v1/requests/absence/preview` | `absence:self` |
| `create_absence_request` | `POST /api/v1/requests/absence` | `absence:self:write` |
| `get_my_requests` | `GET /api/v1/requests` | `request:self` |
| `preview_access_bundle` | `POST /api/v1/requests/access/preview` | `access:self` |
| `create_access_requests` | `POST /api/v1/requests/access` | `access:self:write` |

Действия идут в два шага: `preview_*` возвращает черновик, токен подтверждения агент прячет
от модели. `create_*` выполняется только в следующем сообщении после явного «да».

## Режимы

| Переменная | Значения | По умолчанию |
|---|---|---|
| `LLM_MODE` | `mock` — сценарный mock без ключа, `real` — OpenAI-совместимое API | `mock` |
| `TOOLS_MODE` | `mcp` — инструменты через MCP-сервер, `direct` — напрямую в core | `mcp` |

Остальные переменные — в `app/config.py`: `CORE_URL`, `MCP_URL`, `JWT_SECRET`, `LLM_MODEL`,
`LLM_API_KEY`, `LLM_BASE_URL`, `LLM_RETRIES`, `LLM_RETRY_DELAY`, `MAX_TOOL_ITERATIONS`,
`TOOL_TIMEOUT`, `LOG_LEVEL`, `LOG_JSON`, `RATE_LIMIT_PER_MINUTE`.

## Guardrails

- **Вход:** длина сообщения до 4000 символов, не больше `RATE_LIMIT_PER_MINUTE` сообщений в минуту
  на пользователя (иначе `429`), эвристика prompt injection (`app/guardrails/injection.py`). Она не
  блокирует запрос, а добавляет модели предупреждение и пишет флаги в `agent_run.injection_input`.
- **Данные инструментов:** предложения, похожие на инструкции для модели, заменяются на
  «[фрагмент скрыт…]». Если такое нашлось, действия (`preview_*`, `create_*`) до конца ответа
  блокируются с ошибкой `INJECTION_SUSPECTED`.
- **Выход:** email, телефоны и секреты (`sk-…`, `ghp_…`, JWT, `пароль: …`) маскируются и в данных
  для модели, и в финальном ответе (`app/guardrails/pii.py`).
- Мутации — только через preview и подтверждение в следующем сообщении.

## Логи

Логи пишутся в stderr одной JSON-строкой: `ts`, `level`, `service`, `logger`, `message`.
На каждый запрос чата — запись `agent_run`: `trace_id` (из `X-Trace-Id` / `X-Request-ID`,
возвращается в ответе), `user_id`, `role`, `prompt_version`, `model`, `status`
(`ok` / `limit` / `llm_error` / `error` / `cancelled`), `latency_ms`, `tool_iterations`,
`llm_retries`, токены и `tool_calls` с `latency_ms` и `error_code`. Текст сообщения не пишется.
Временные ошибки LLM (429, 5xx, таймаут) повторяются `LLM_RETRIES` раз.

## Evals

`evals/golden.jsonl` — эталонные вопросы: база знаний, ACL по ролям, «не знаю», план, профиль,
действия с подтверждением. Каждый кейс задаёт, какие инструменты и с какими аргументами
должны вызваться, какие документы обязаны или не имеют права попасть в цитаты и что должно
или не должно быть в ответе. Core подменяется `evals/fake_core.py` — копия seed-документов
и правил видимости core.

```bash
uv run python -m evals.run
uv run python -m evals.run --suite injection
uv run python -m evals.run --mode real --min-pass 0.8
```

`evals/injection.jsonl` — 21 кейс атак: через сообщение пользователя, через «отравленные»
документы, утечки ПДн и секретов, плюс безобидные вопросы против ложных срабатываний.

Отчёты пишутся в `evals/report.md` и `evals/injection_report.md`. В CI golden set прогоняется в mock-режиме внутри pytest.

## Проверки

```bash
uv run ruff check --fix . && uv run ruff format .
uv run pytest
```
