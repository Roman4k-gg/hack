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

## Режимы

| Переменная | Значения | По умолчанию |
|---|---|---|
| `LLM_MODE` | `mock` — сценарный mock без ключа, `real` — OpenAI-совместимое API | `mock` |
| `TOOLS_MODE` | `mcp` — инструменты через MCP-сервер, `direct` — напрямую в core | `mcp` |

Остальные переменные — в `app/config.py`: `CORE_URL`, `MCP_URL`, `JWT_SECRET`, `LLM_MODEL`,
`LLM_API_KEY`, `LLM_BASE_URL`, `MAX_TOOL_ITERATIONS`, `TOOL_TIMEOUT`, `LOG_LEVEL`.

## Проверки

```bash
uv run ruff check --fix . && uv run ruff format .
uv run pytest
```
