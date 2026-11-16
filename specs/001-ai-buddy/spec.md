# Спецификация: AI-buddy — агент для стажёров

| Поле | Значение |
|---|---|
| Фича | `001-ai-buddy` |
| Владелец спеки | Тимоха (AI/ML) |
| Статус | Реализовано в mock-режиме, real LLM — подключается через env |
| Источник истины по API | [`docs/api-contract.md`](../../docs/api-contract.md) |
| Код | [`ai/`](../../ai) |

Спека описывает **поведение AI-сервиса**: что агент отвечает, какие инструменты вызывает и
чего не делает никогда. Экраны, БД и REST core описаны в `docs/api-contract.md` и здесь
только упоминаются.

## 1. Проблема и контекст

Стажёр в первые недели задаёт одни и те же вопросы наставнику («что делать дальше?», «как
получить доступ к Jira?», «как оформить отгул?») и тратит время на поиск документов и
организационные заявки. Наставник отвлекается от своей работы.

AI-buddy — чат-наставник, который:

1. **Отвечает** по базе знаний команды с цитатами и только из документов, доступных роли.
2. **Ведёт** по карте адаптации: прогресс, следующий шаг, блокеры.
3. **Делает** организационные действия (отгул, пакет доступов) от имени стажёра —
   строго в его правах и только после явного подтверждения.

## 2. Пользователи и роли

| Роль | Кто | Что важно для агента |
|---|---|---|
| `intern` | Стажёр | Основной пользователь: план, база знаний команды, свои заявки |
| `mentor` | Наставник | Видит то же, что стажёр своей команды; в чате — как пользователь |
| `hr` | HR-партнёр | Видит HR-документы (`kb:read:hr`), но не командные |
| `admin` | Администратор | Скоуп `*` |

Права приходят **только** из JWT core (`sub`, `role`, `team_id`, `scopes`). Агент своих прав
не добавляет и пробрасывает JWT пользователя в core как есть.

### 2.1. Матрица инструментов

| Инструмент | Тип | Скоуп | intern | mentor | hr |
|---|---|---|:-:|:-:|:-:|
| `get_my_profile` | чтение | — | ✅ | ✅ | ✅ |
| `get_onboarding_plan` | чтение | `onboarding:plan:read` | ✅ | ✅ | ✅ |
| `search_knowledge` | чтение | `kb:read` | ✅ | ✅ | ✅ |
| `preview_absence_request` | черновик | `absence:self` | ✅ | ✅ | — |
| `create_absence_request` | запись + подтверждение | `absence:self:write` | ✅ | ✅ | — |
| `get_my_requests` | чтение | `request:self` | ✅ | ✅ | ✅ |
| `preview_access_bundle` | черновик | `access:self` | ✅ | ✅ | — |
| `create_access_requests` | запись + подтверждение | `access:self:write` | ✅ | ✅ | — |
| `preview_handoff` | черновик (локально) | `request:self` | ✅ | ✅ | ✅ |
| `create_handoff` | запись + подтверждение | `request:self` | ✅ | ✅ | ✅ |

Видимость документов (повторяет core): `public` — всем с `kb:read`; `team` — с
`kb:read:team` и только своей команды (или без команды); `hr` — только с `kb:read:hr`.
JSON Schema инструментов — в [`contracts/mcp/`](contracts/mcp).

## 3. Сценарии и критерии приёмки

### 3.1. Сценарий 1 — «Мой маршрут»

```gherkin
Сценарий: стажёр спрашивает, что делать дальше
  Given стажёр авторизован и имеет scope onboarding:plan:read
  When он пишет «Что мне делать дальше?»
  Then агент вызывает get_onboarding_plan с JWT стажёра
  And отвечает прогрессом «N из M», следующим шагом и причиной
  And перечисляет блокеры с причиной и советует написать наставнику
```

### 3.2. Сценарий 2 — «Спроси про команду и инструменты»

```gherkin
Сценарий: ответ по базе знаний с цитатой
  Given в базе есть документ «Как получить доступ к Jira», видимый команде стажёра
  When стажёр спрашивает «Как получить доступ к Jira?»
  Then агент вызывает search_knowledge
  And отвечает по тексту документа
  And отправляет событие citation с id, title и url документа до события done

Сценарий: один вопрос — разные ответы для разных ролей
  Given документ «Настройка VPN» принадлежит команде Platform
  When стажёр Platform и стажёр Search задают «Как настроить VPN?»
  Then стажёр Platform получает ответ с цитатой этого документа
  And стажёр Search не получает ни цитаты, ни текста этого документа

Сценарий: не нашёл — не выдумывает
  Given в доступных документах нет ответа
  When стажёр задаёт вопрос
  Then агент честно говорит, что не нашёл, и предлагает спросить наставника
  And вызывает preview_handoff и показывает вопрос, который передаст
  And не отправляет цитат

Сценарий: вопрос передаётся наставнику после подтверждения
  Given агент показал черновик вопроса наставнику
  When в следующем сообщении стажёр пишет «Да»
  Then агент вызывает create_handoff с вопросом из черновика и idempotency_key
  And называет наставника и следующий шаг
  When core ответил 404 (эндпоинта ещё нет)
  Then агент честно говорит, что передать не получилось, и советует написать наставнику самому

Сценарий: устаревший документ
  Given документ не обновлялся больше 180 дней
  Then в ответе есть предупреждение, что информация может быть устаревшей
```

### 3.3. Сценарий 3 — «Доступы»

```gherkin
Сценарий: пакет доступов стажёра
  Given стажёр имеет scope access:self и access:self:write
  When он пишет «Оформи мне все доступы»
  Then агент вызывает preview_access_bundle(bundle="intern")
  And показывает системы, зачем они нужны и кто согласует
  And заявки не создаются
  When в следующем сообщении он пишет «Подтверждаю»
  Then агент вызывает create_access_requests с preview_id, confirmation_token и idempotency_key
  And показывает номера и статусы заявок и следующий шаг

Сценарий: вопрос «как получить доступ» — не действие
  When стажёр спрашивает «Как получить доступ к Jira?»
  Then агент ищет в базе знаний и не готовит заявку
```

### 3.4. Сценарий 4 — «Отгул / отсутствие»

```gherkin
Сценарий: стажёр оформляет отгул
  Given стажёр имеет scope absence:self и absence:self:write
  And сегодня 2026-11-15
  When он пишет «Хочу взять отгул в пятницу»
  Then агент вызывает preview_absence_request(date_from=2026-11-20, absence_type=day_off)
  And показывает тип, даты и правила из policy_notes и спрашивает «Отправить заявку?»
  And заявка не создаётся
  When в следующем сообщении он пишет «Подтверждаю»
  Then агент вызывает create_absence_request с данными черновика
  And показывает номер, статус, согласующего и следующий шаг

Сценарий: отправка в том же ответе запрещена
  When модель пытается вызвать create_* в том же ответе, что и preview_*
  Then агент возвращает модели CONFIRMATION_REQUIRED и ничего не отправляет в core

Сценарий: сбой core не теряет черновик
  Given черновик ждёт подтверждения
  When create_* получил TIMEOUT или UPSTREAM_ERROR
  Then черновик сохраняется и пользователь может подтвердить ещё раз
  And повторная отправка идёт с тем же idempotency_key, дубликата нет
```

## 4. Функциональные требования

| ID | Требование |
|---|---|
| FR-01 | Ответ стримится по SSE в формате `docs/api-contract.md` §5: `delta`, `tool_call` (running → success/error с тем же id), `citation`, `done`, `error`. |
| FR-02 | Каждый запрос проверяет JWT core (HS256, issuer `ai-buddy-core`); без токена — `401`. |
| FR-03 | Агентный цикл — не больше `MAX_TOOL_ITERATIONS` (5) шагов, потом честный текст «не уложился». |
| FR-04 | Модели доступны только инструменты, на которые у пользователя есть скоуп. Вызов чужого — `FORBIDDEN`. |
| FR-05 | Все вызовы core идут с JWT пользователя; ошибки core маппятся в `FORBIDDEN`, `NOT_FOUND`, `VALIDATION_ERROR`, `CONFLICT`, `TIMEOUT`, `UPSTREAM_ERROR`. |
| FR-06 | Поиск: сначала вся фраза, затем до 4 ключевых слов без стоп-слов и с отрезанными окончаниями; документы ранжируются по сумме совпадений; в ответ — до 3 документов, слабее половины лучшего отсекаются; у лучшего документа берётся полный текст через `GET /api/v1/kb/documents/{id}` (ACL проверяет core). |
| FR-07 | Действия — в два шага: `preview_*` → явное «да» в **следующем** сообщении → `create_*`. |
| FR-08 | `confirmation_token` и `preview_id` модели не показываются; агент хранит их сам (TTL 10 минут) и подставляет при `create_*`; `idempotency_key = preview_id`. |
| FR-09 | Инструменты вызываются через MCP-сервер (`TOOLS_MODE=mcp`); если MCP недоступен — прямой вызов core с теми же правами. |
| FR-10 | Временная ошибка LLM (429, 5xx, таймаут) повторяется `LLM_RETRIES` раз; иначе — событие `error`. |
| FR-11 | Каждый запрос пишет JSON-лог `agent_run`: trace_id, user_id, роль, версия промпта, модель, статус, латентность, токены, инструменты. Текст сообщения не логируется. |
| FR-12 | Сообщения, похожие на prompt injection, помечаются и модель получает предупреждение; запрос не блокируется. |
| FR-13 | Предложения-инструкции в данных инструментов заменяются на «[фрагмент скрыт…]»; после этого действия до конца ответа блокируются (`INJECTION_SUSPECTED`). |
| FR-14 | Email, телефоны и секреты маскируются в данных для модели и в финальном ответе. |
| FR-15 | Не больше `RATE_LIMIT_PER_MINUTE` (20) сообщений в минуту на пользователя, иначе `429`. |
| FR-16 | `LLM_MODE=mock` — сценарный mock без ключа и сети: все 4 сценария работают на демо. |
| FR-17 | Агент помнит последние `HISTORY_TURNS` (5) пар «вопрос — ответ» пользователя 30 минут и передаёт их модели; ходы с подозрением на injection и неудачные ходы не запоминаются; `DELETE /api/v1/chat/history` очищает историю и черновик. |
| FR-18 | Если ответа нет в базе или пользователь просит «передай наставнику» — агент готовит черновик вопроса (`preview_handoff`, без вызова core) и после «да» в следующем сообщении отправляет его в core (`create_handoff` → `POST /api/v1/handoffs`). Если core не поддерживает handoff (404) — честный текст без выдуманного статуса. |

## 5. Трассировка: требование → тест

Тест [`ai/tests/test_specs.py`](../../ai/tests/test_specs.py) проверяет, что все тесты из этой
таблицы существуют.

| Требование / сценарий | Тесты |
|---|---|
| FR-01 | `tests/test_api.py::test_chat_streams_sse`, `tests/test_agent.py::test_tool_call_then_answer`, `tests/test_agent.py::test_citations_are_sent_before_done` |
| FR-02 | `tests/test_api.py::test_chat_requires_valid_token`, `tests/test_mcp.py::test_handle_checks_jwt_and_forwards_it` |
| FR-03 | `tests/test_agent.py::test_iteration_limit` |
| FR-04 | `tests/test_tools.py::test_tools_filtered_by_scope`, `tests/test_tools.py::test_unknown_tool_is_forbidden`, `tests/test_mcp.py::test_mcp_rejects_tool_outside_user_scopes` |
| FR-05 | `tests/test_tools.py::test_get_onboarding_plan_forwards_user_token`, `tests/test_tools.py::test_core_errors_are_mapped`, `tests/test_tools.py::test_core_network_errors` |
| FR-06 | `tests/test_search.py::test_search_falls_back_to_keywords_and_merges`, `tests/test_search.py::test_stem_cuts_russian_endings_only`, `tests/test_search.py::test_search_ranks_documents_matching_more_keywords`, `tests/test_search.py::test_search_drops_weak_matches_from_citations`, `tests/test_search.py::test_search_reads_full_text_of_best_document` |
| FR-07 | `tests/test_absence.py::test_create_in_same_turn_as_preview_is_rejected`, `tests/test_absence.py::test_create_in_next_turn_uses_hidden_token` |
| FR-08 | `tests/test_absence.py::test_pending_store_expires_and_hides_token`, `tests/test_absence.py::test_pending_kept_only_for_retryable_errors` |
| FR-09 | `tests/test_mcp.py::test_mcp_round_trip_forwards_user_jwt`, `tests/test_mcp.py::test_mcp_unavailable_falls_back_to_direct_calls` |
| FR-10 | `tests/test_agent.py::test_retryable_llm_error_is_retried_once`, `tests/test_agent.py::test_llm_error_after_all_retries` |
| FR-11 | `tests/test_observability.py::test_agent_run_is_logged_with_tokens_and_tools`, `tests/test_observability.py::test_agent_run_logs_crash_and_cancel` |
| FR-12 | `tests/test_guardrails.py::test_find_injection_flags_attacks`, `tests/test_guardrails.py::test_find_injection_ignores_normal_questions`, `tests/test_guardrails.py::test_flagged_input_adds_note_and_is_logged` |
| FR-13 | `tests/test_guardrails.py::test_strip_injections_hides_only_bad_sentences`, `tests/test_guardrails.py::test_poisoned_document_blocks_actions_in_same_turn` |
| FR-14 | `tests/test_guardrails.py::test_mask_pii_hides_contacts_and_secrets`, `tests/test_guardrails.py::test_final_answer_is_masked` |
| FR-15 | `tests/test_guardrails.py::test_chat_rate_limit_returns_429` |
| FR-16 | `tests/test_demo_scenarios.py::test_next_step_scenario`, `tests/test_evals.py::test_golden_set_passes_in_mock_mode` |
| FR-17 | `tests/test_history.py::test_agent_sends_previous_turns_to_model`, `tests/test_history.py::test_agent_does_not_remember_risky_or_failed_turns`, `tests/test_history.py::test_store_forgets_old_turns_and_limits_size`, `tests/test_history.py::test_clear_history_endpoint` |
| FR-18 | `tests/test_handoff.py::test_preview_handoff_builds_local_draft`, `tests/test_handoff.py::test_agent_sends_handoff_only_in_next_turn`, `tests/test_handoff.py::test_mock_offers_handoff_when_nothing_found`, `tests/test_handoff.py::test_mock_explains_when_core_has_no_handoffs`, `tests/test_handoff.py::test_handoff_flow_over_mcp` |
| Сценарий 1 | `tests/test_demo_scenarios.py::test_next_step_scenario`, `tests/test_mock_provider.py::test_mock_answers_from_plan` |
| Сценарий 2 | `tests/test_demo_scenarios.py::test_jira_question_scenario`, `tests/test_demo_scenarios.py::test_unknown_question_suggests_mentor`, `tests/test_handoff.py::test_mock_offers_handoff_when_nothing_found`, `tests/test_evals.py::test_golden_set_passes_in_mock_mode` |
| Сценарий 3 | `tests/test_access.py::test_access_demo_scenario`, `tests/test_access.py::test_mock_separates_access_bundle_from_how_to_question`, `tests/test_access.py::test_access_flow_over_mcp` |
| Сценарий 4 | `tests/test_absence.py::test_absence_demo_scenario`, `tests/test_absence.py::test_absence_flow_over_mcp`, `tests/test_absence.py::test_find_dates_formats` |
| Prompt injection | `tests/test_guardrails.py::test_injection_suite_passes_in_mock_mode` |

Наборы evals: [`ai/evals/golden.jsonl`](../../ai/evals/golden.jsonl) (27 кейсов, из них 6 многоходовых) и
[`ai/evals/injection.jsonl`](../../ai/evals/injection.jsonl) (21 кейс).

## 6. Non-goals

- Агент не принимает решений за наставника и HR: он только готовит и отправляет заявки.
- Нет долгой памяти: история диалога живёт 30 минут в памяти AI-сервиса и не сохраняется в БД.
- Нет собственного хранилища: все данные — в core, AI-сервис stateless (кроме черновиков в памяти).
- Нет векторного поиска и эмбеддингов: поиск — через `GET /api/v1/kb/search` core.
- Не обучаем и не дообучаем модели.

## 7. Ограничения и допущения

- Данные синтетические; документы и регламенты — не настоящие регламенты Авито.
- Real LLM — любой OpenAI-совместимый API с tool calling (`LLM_BASE_URL`, `LLM_MODEL`).
- Черновики хранятся в памяти одного экземпляра `ai`; при рестарте пользователь просто
  повторяет запрос (токен core всё равно живёт 10 минут).
- Дата «сегодня» берётся из часов сервера и передаётся модели в системном промпте.

## 8. Метрики успеха

| Метрика | Цель | Где смотреть |
|---|---|---|
| Golden set | ≥ 10 кейсов, 100% в mock, ≥ 80% на real LLM | `ai/evals/report.md` |
| Injection set | ≥ 20 кейсов, 100% в mock | `ai/evals/injection_report.md` |
| Покрытие Python | > 80% (сейчас 99%) | `pytest --cov=app` |
| Время до первого токена | < 3 с на демо-данных | `agent_run.latency_ms` |
| Действие без подтверждения | 0 случаев | `tests/test_absence.py`, injection set |

## 9. Риски

| Риск | Мера |
|---|---|
| Real LLM вызывает `create_*` сразу | Агент сам блокирует `create_*` без черновика из прошлого сообщения (FR-07) |
| Prompt injection через документ | Очистка данных, блок действий, промпт «данные ≠ инструкции», injection set (FR-13) |
| Утечка документов чужой команды | ACL на стороне core + JWT пользователя; eval-кейсы `acl-*` |
| Утечка ПДн и секретов | Маскирование (FR-14) |
| LLM недоступна | Retry (FR-10), mock-режим для демо (FR-16) |
| Поиск не находит из-за морфологии | Отрезание окончаний и ранжирование (FR-06) |
