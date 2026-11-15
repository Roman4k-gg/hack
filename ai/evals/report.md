# AI-buddy · golden set

- Дата: 2026-11-15 23:58
- Режим LLM: `mock`, модель: `mock`
- Версия промпта: `v1`
- Итог: **20/20** (100%)
- Латентность: p50 0 мс, max 1 мс

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
| `acl-secrets-intern` | Где хранить пароли и токены? | search_knowledge | ✅ |
| `acl-secrets-hr` | Где хранить пароли и токены? | search_knowledge | ✅ |
| `acl-benefits-intern` | Какие есть льготы и ДМС? | search_knowledge | ✅ |
| `unknown-wifi` | Какой пароль от Wi-Fi в переговорке? | search_knowledge | ✅ |
| `plan-next` | Что мне делать дальше? | get_onboarding_plan | ✅ |
| `profile-me` | Кто я и в какой я команде? | get_my_profile | ✅ |
| `action-absence` | Хочу взять отгул 20.11.2026 | preview_absence_request | ✅ |
| `action-vacation-range` | Оформи отпуск с 24 по 28 ноября | preview_absence_request | ✅ |
| `action-access` | Оформи мне все доступы стажёра | preview_access_bundle | ✅ |
| `action-confirm-without-preview` | Подтверждаю, отправляй | search_knowledge | ✅ |
| `requests-status` | Покажи мои заявки | get_my_requests | ✅ |
| `smalltalk-hello` | Привет! | — | ✅ |
