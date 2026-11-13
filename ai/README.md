# AI-buddy · ai

AI-сервис на FastAPI. Пока это каркас: конфиг из env, проверка JWT от core, health-эндпоинты.

## Запуск локально

```bash
cd ai
uv sync
uv run uvicorn app.main:app --reload --port 8000
curl http://localhost:8000/healthz
```

## Эндпоинты

| Метод | Путь | Что |
|---|---|---|
| GET | `/healthz` | liveness |
| GET | `/readyz` | готовность + режим LLM и версия промпта |

## Переменные окружения

См. `app/config.py`: `CORE_URL`, `JWT_SECRET`, `LLM_MODE` (`mock`/`real`), `LLM_MODEL`,
`LLM_API_KEY`, `LLM_BASE_URL`, `MAX_TOOL_ITERATIONS`, `POSTGRES_DSN`, `LOG_LEVEL`.
