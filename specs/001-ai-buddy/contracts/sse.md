# SSE-контракт чата

Источник истины — [`docs/api-contract.md` §5](../../../docs/api-contract.md). Здесь — что именно
отдаёт AI-сервис и в каком порядке. Реализация: [`ai/app/sse.py`](../../../ai/app/sse.py).

Каждое событие — `event: message` и одна строка `data` с JSON, между событиями пустая строка.

| `type` | `data` |
|---|---|
| `delta` | `{"type":"delta","text":"..."}` — кусок ответа, по 3 слова |
| `tool_call` | `{"type":"tool_call","toolCall":{"id","name","status"}}`, `status`: `running` → `success` / `error` с тем же `id` |
| `citation` | `{"type":"citation","citation":{"id","title","url"?}}` |
| `done` | `{"type":"done"}` |
| `error` | `{"type":"error","message":"..."}` |

## Порядок

```
tool_call(running) → tool_call(success|error)   × N, до 5 шагов
delta × M                                         финальный ответ
citation × K                                      документы из search_knowledge, без повторов
done
```

- `error` завершает поток без `done`: LLM недоступна после retry, внутренняя ошибка,
  превышен rate limit (в этом случае HTTP-статус `429` и заголовок `Retry-After: 60`).
- Если инструмент вернул ошибку, поток продолжается: модель получает ошибку и объясняет её.
- Заголовок ответа `X-Trace-Id` совпадает с `trace_id` в логе `agent_run`.

## Пример: вопрос по базе знаний

```
event: message
data: {"type":"tool_call","toolCall":{"id":"call-1","name":"search_knowledge","status":"running"}}

event: message
data: {"type":"tool_call","toolCall":{"id":"call-1","name":"search_knowledge","status":"success"}}

event: message
data: {"type":"delta","text":"Нашёл в документе "}

event: message
data: {"type":"citation","citation":{"id":"55555555-5555-5555-5555-555555555501","title":"Как получить доступ к Jira","url":"https://demo.local/docs/jira-access"}}

event: message
data: {"type":"done"}
```
