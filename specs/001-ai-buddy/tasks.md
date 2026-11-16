# Задачи: AI-сервис и MCP

Владелец — Тимоха. ID T-01…T-12 — из [`plan_day1.md`](../../plan_day1.md), дальше — новые.
`[x]` — сделано и закоммичено, в скобках — коммит.

## День 1

- [x] **T-01** SDD-спека `specs/001-ai-buddy/` и тест связи спеки с кодом
- [x] **T-02** Каркас `ai/`: FastAPI, конфиг, `/healthz`, `/readyz`
- [x] **T-03** Интерфейс провайдера, mock LLM и real-провайдер (`7933db6`). Вместо LiteLLM — свой
  httpx-клиент к OpenAI-совместимому API, см. [research.md](research.md)
- [x] **T-04** Агентный цикл и SSE-события (`ce0cd6a`)
- [x] **T-05** MCP-сервер: `get_my_profile`, `get_onboarding_plan`, `search_knowledge` (`562288f`).
  Вместо FastMCP — `MCPServer` из MCP SDK 2.x
- [x] **T-06** Проброс JWT пользователя из AI в MCP и в core (`562288f`)
- [x] **T-07** Сценарный mock LLM для демо (`a568309`)
- [x] **T-08** Логи `agent_run`, таймауты, retry, обработка ошибок инструментов (`c435a3f`)
- [x] **T-09** Запуск `ai` и `mcp` в Docker Compose вместе с core: все 4 сценария через шлюз core и MCP,
  smoke 32/32 (`4dd8670`)
- [x] **T-10** Golden-вопросы: 20 кейсов, fake core, отчёт; стемминг в поиске (`c3051d1`)
- [x] **T-11** Dockerfile AI-сервиса
- [x] **T-12** pytest-тесты на tool loop (сейчас 170+ тестов, покрытие 99%)

## Сценарии 3–4 и качество

- [x] **T-13** Отгул и пакет доступов: preview → подтверждение → create, скрытый токен,
  разбор дат, `get_my_requests` (`f9b9484`)
- [x] **T-14** Guardrails: injection, очистка документов, маскирование ПДн, rate limit,
  injection set 21 кейс (`a4f3f84`)

## Осталось

- [ ] **T-15** Подключить real LLM: ключ в `.env`, прогон golden и injection на real, правка промпта
- [ ] **T-16** Репетиция демо: 4 сценария в mock и real
- [ ] **T-17** (stretch) AI-cron: утренний дайджест и напоминания о зависших заявках
- [x] **T-18** Handoff наставнику, когда ответ не найден или пользователь просит: `preview_handoff` →
  «да» → `create_handoff`. AI-часть готова, в core нужен `POST /api/v1/handoffs`
  ([contracts/rest.md](contracts/rest.md)), до этого агент честно говорит, что передать не вышло
- [x] **T-19** История диалога: последние 5 пар «вопрос — ответ», 30 минут, сброс через
  `DELETE /api/v1/chat/history`; многоходовые кейсы в golden. Summary — не делали
- [ ] **T-20** (stretch) Черновики в Redis для нескольких экземпляров `ai`

## Проверки перед коммитом

```bash
cd ai
uv run ruff check --fix . && uv run ruff format .
uv run pytest
uv run python -m evals.run
uv run python -m evals.run --suite injection
```
