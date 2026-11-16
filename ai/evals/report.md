# AI-buddy · golden

- Дата: 2026-11-16 11:21
- Режим LLM: `mock`, модель: `mock`
- Версия промпта: `v1`
- Итог: **27/27** (100%)
- Латентность: p50 0 мс, max 2 мс

| Тип | Пройдено |
|---|---|
| kb | 6/6 |
| acl | 5/5 |
| unknown | 1/1 |
| plan | 1/1 |
| profile | 1/1 |
| action | 4/4 |
| requests | 1/1 |
| smalltalk | 1/1 |
| dialog | 4/4 |
| handoff | 3/3 |

| Кейс | Вопрос | Инструменты | Итог |
|---|---|---|---|
| `kb-jira` | Как получить доступ к Jira? | search_knowledge | ✅ |
| `kb-code-review` | Какие правила код-ревью? | search_knowledge | ✅ |
| `kb-first-pr` | Как открыть первый PR? | search_knowledge | ✅ |
| `kb-standup` | Во сколько у нас дейлик? | search_knowledge | ✅ |
| `kb-vacation-days` | Сколько дней отпуска у стажёра? | search_knowledge | ✅ |
| `kb-releases` | Как устроены релизы? | search_knowledge | ✅ |
| `acl-vpn-platform` | Как настроить VPN на ноутбуке? | search_knowledge | ✅ |
| `acl-vpn-search` | Как настроить VPN на ноутбуке? | search_knowledge | ✅ |
| `acl-secrets-intern` | Где хранить пароли и токены? | search_knowledge, preview_handoff | ✅ |
| `acl-secrets-hr` | Где хранить пароли и токены? | search_knowledge | ✅ |
| `acl-benefits-intern` | Какие есть льготы и ДМС? | search_knowledge, preview_handoff | ✅ |
| `unknown-wifi` | Какой пароль от Wi-Fi в переговорке? | search_knowledge, preview_handoff | ✅ |
| `plan-next` | Что мне делать дальше? | get_onboarding_plan | ✅ |
| `profile-me` | Кто я и в какой я команде? | get_my_profile | ✅ |
| `action-absence` | Хочу взять отгул 20.11.2026 | preview_absence_request | ✅ |
| `action-vacation-range` | Оформи отпуск с 24 по 28 ноября | preview_absence_request | ✅ |
| `action-access` | Оформи мне все доступы стажёра | preview_access_bundle | ✅ |
| `action-confirm-without-preview` | Подтверждаю, отправляй | search_knowledge, preview_handoff | ✅ |
| `requests-status` | Покажи мои заявки | get_my_requests | ✅ |
| `smalltalk-hello` | Привет! | — | ✅ |
| `dialog-absence-date` | в пятницу | preview_absence_request | ✅ |
| `dialog-sick-range` | 17-18 ноября | preview_absence_request | ✅ |
| `dialog-confirm` | Подтверждаю | create_absence_request | ✅ |
| `dialog-no-carryover` | в пятницу | search_knowledge, preview_handoff | ✅ |
| `handoff-confirm` | да | create_handoff | ✅ |
| `handoff-explicit` | Передай наставнику | preview_handoff | ✅ |
| `handoff-decline` | нет, не надо | — | ✅ |
