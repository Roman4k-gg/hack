#!/usr/bin/env bash
set -uo pipefail

CORE_URL="${CORE_URL:-http://localhost:8081}"
WEB_URL="${WEB_URL:-http://localhost:3000}"

PASSED=0
FAILED=0
SKIPPED=0

ok() {
    printf '  \033[32mOK\033[0m    %s\n' "$1"
    PASSED=$((PASSED + 1))
}

fail() {
    printf '  \033[31mFAIL\033[0m  %s\n' "$1"
    FAILED=$((FAILED + 1))
}

skip() {
    printf '  \033[33mSKIP\033[0m  %s\n' "$1"
    SKIPPED=$((SKIPPED + 1))
}

section() {
    printf '\n\033[1m%s\033[0m\n' "$1"
}

json_get() {
    python3 -c "import json,sys;d=json.load(sys.stdin);print($1)" 2>/dev/null
}

section 'Healthchecks'

if body=$(curl -sS --max-time 10 "$CORE_URL/healthz") && [ "$(json_get "d['status']" <<<"$body")" = "ok" ]; then
    ok "core /healthz отвечает ok"
else
    fail "core /healthz не отвечает (запустите make up)"
fi

if body=$(curl -sS --max-time 10 "$CORE_URL/readyz") && [ "$(json_get "d['status']" <<<"$body")" = "ok" ]; then
    ok "core /readyz: postgres и redis доступны"
else
    fail "core /readyz: $(json_get "d['checks']" <<<"$body" 2>/dev/null || echo 'нет ответа')"
fi

section 'Авторизация'

LOGIN=$(curl -sS --max-time 10 -X POST "$CORE_URL/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d '{"email":"dasha@demo.local"}')

TOKEN=$(json_get "d['token']" <<<"$LOGIN")

if [ -n "${TOKEN:-}" ]; then
    ok "mock-логин dasha@demo.local выдал JWT"
else
    fail "логин не выдал токен: $LOGIN"
fi

AUTH=(-H "Authorization: Bearer $TOKEN")

if body=$(curl -sS --max-time 10 "${AUTH[@]}" "$CORE_URL/api/v1/me") && \
    [ "$(json_get "d['role']" <<<"$body")" = "intern" ] && \
    [ "$(json_get "d['team_name']" <<<"$body")" = "Search" ]; then
    ok "GET /api/v1/me: роль intern, команда Search"
else
    fail "GET /api/v1/me вернул: $body"
fi

code=$(curl -sS --max-time 10 -o /dev/null -w '%{http_code}' "$CORE_URL/api/v1/me")
if [ "$code" = "401" ]; then
    ok "GET /api/v1/me без токена → 401"
else
    fail "GET /api/v1/me без токена → $code (ожидался 401)"
fi

section 'Карта адаптации'

PLAN=$(curl -sS --max-time 10 "${AUTH[@]}" "$CORE_URL/api/v1/onboarding/plan")

STAGES=$(json_get "len(d['stages'])" <<<"$PLAN")
ITEMS=$(json_get "sum(len(s['items']) for s in d['stages'])" <<<"$PLAN")
NEXT=$(json_get "d['next_step']['title']" <<<"$PLAN")

if [ "${STAGES:-0}" -ge 3 ] && [ "${ITEMS:-0}" -ge 8 ] && [ "${ITEMS:-0}" -le 12 ]; then
    ok "GET /api/v1/onboarding/plan: этапов $STAGES, пунктов $ITEMS"
else
    fail "GET /api/v1/onboarding/plan вернул: $PLAN"
fi

if [ -n "${NEXT:-}" ]; then
    ok "next_step: $NEXT"
else
    fail "в плане нет next_step"
fi

ITEM_ID=$(json_get "next((i['id'] for i in d['stages'][0]['items'] if i['status'] not in ('done','skipped')), '')" <<<"$PLAN")

if [ -n "${ITEM_ID:-}" ]; then
    BODY=$(curl -sS --max-time 10 -X PATCH "$CORE_URL/api/v1/onboarding/items/$ITEM_ID" \
        "${AUTH[@]}" -H 'Content-Type: application/json' \
        -d '{"status":"in_progress"}')

    if [ "$(json_get "d['status']" <<<"$BODY")" = "in_progress" ]; then
        ok "PATCH /api/v1/onboarding/items/{id}: статус in_progress"
    else
        fail "PATCH пункта вернул: $BODY"
    fi

    BODY=$(curl -sS --max-time 10 -X PATCH "$CORE_URL/api/v1/onboarding/items/$ITEM_ID" \
        "${AUTH[@]}" -H 'Content-Type: application/json' \
        -d '{"status":"blocked"}')

    if [ "$(json_get "d['error']['code']" <<<"$BODY")" = "VALIDATION_ERROR" ]; then
        ok "PATCH со статусом blocked без причины → 400"
    else
        fail "PATCH со статусом blocked без причины вернул: $BODY"
    fi
else
    skip "PATCH пункта: нет незакрытых пунктов"
fi

section 'Метрики команды'

MENTOR_LOGIN=$(curl -sS --max-time 10 -X POST "$CORE_URL/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d '{"email":"petr@demo.local"}')

MENTOR_TOKEN=$(json_get "d['token']" <<<"$MENTOR_LOGIN")

if [ -n "${MENTOR_TOKEN:-}" ]; then
    TEAM=$(curl -sS --max-time 10 "$CORE_URL/api/v1/team/interns" \
        -H "Authorization: Bearer $MENTOR_TOKEN")

    COUNT=$(json_get "len(d['interns'])" <<<"$TEAM")
    METRICS=$(json_get "all('progress' in m and 'pace' in m for m in d['interns'])" <<<"$TEAM")
    AT_RISK=$(json_get "d['overview']['at_risk']" <<<"$TEAM")

    if [ "${COUNT:-0}" -gt 0 ] && [ "${METRICS:-}" = "True" ]; then
        ok "GET /api/v1/team/interns: стажёров $COUNT, у всех прогресс и успеваемость, в риске $AT_RISK"
    else
        fail "GET /api/v1/team/interns вернул: $TEAM"
    fi

    code=$(curl -sS --max-time 10 -o /dev/null -w '%{http_code}' \
        "$CORE_URL/api/v1/team/interns" "${AUTH[@]}")

    if [ "$code" = "403" ]; then
        ok "стажёр не видит метрики команды → 403"
    else
        fail "стажёр запросил метрики команды → $code (ожидался 403)"
    fi
else
    fail "mock-логин petr@demo.local не выдал токен"
fi

section 'База знаний'

KB_QUERY=$(python3 -c 'import urllib.parse;print(urllib.parse.quote("доступ к Jira"))' 2>/dev/null || echo 'access')

KB=$(curl -sS --max-time 10 "$CORE_URL/api/v1/kb/search?q=$KB_QUERY" "${AUTH[@]}")

KB_COUNT=$(json_get "d['total']" <<<"$KB")
KB_TITLE=$(json_get "d['items'][0]['title']" <<<"$KB")

if [ "${KB_COUNT:-0}" -gt 0 ]; then
    ok "GET /api/v1/kb/search: найдено $KB_COUNT, первый «$KB_TITLE»"
else
    fail "GET /api/v1/kb/search ничего не нашёл: $KB"
fi

KB_DOC=$(json_get "d['items'][0]['document_id']" <<<"$KB")

if [ -n "${KB_DOC:-}" ]; then
    BODY=$(curl -sS --max-time 10 "$CORE_URL/api/v1/kb/documents/$KB_DOC" "${AUTH[@]}")

    if [ "$(json_get "d['document_id']" <<<"$BODY")" = "$KB_DOC" ]; then
        ok "GET /api/v1/kb/documents/{id}: документ отдаётся с владельцем и датой"
    else
        fail "GET /api/v1/kb/documents/{id} вернул: $BODY"
    fi
fi

OTHER_TOKEN=$(json_get "d['token']" <<<"$(curl -sS --max-time 10 -X POST "$CORE_URL/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"email":"vera@demo.local"}')")

if [ -n "${OTHER_TOKEN:-}" ]; then
    OTHER=$(curl -sS --max-time 10 "$CORE_URL/api/v1/kb/search?q=$KB_QUERY" -H "Authorization: Bearer $OTHER_TOKEN")
    OTHER_COUNT=$(json_get "d['total']" <<<"$OTHER")
    OTHER_TEAM=$(json_get "d['items'][0]['team']" <<<"$OTHER")

    if [ "${OTHER_COUNT:-0}" -lt "${KB_COUNT:-0}" ] && [ "${OTHER_TEAM:-}" != "Search" ]; then
        ok "ACL: стажёр из Ads не видит team-документы Search (найдено $OTHER_COUNT)"
    else
        fail "ACL нарушен: стажёр из Ads видит $OTHER_COUNT документов, первый «$OTHER_TEAM»"
    fi
else
    fail "mock-логин vera@demo.local не выдал токен"
fi

section 'Чат (SSE)'

STREAM=$(curl -sS --max-time 15 -N -X POST "$CORE_URL/api/v1/chat/stream" \
    "${AUTH[@]}" -H 'Content-Type: application/json' \
    -d '{"message":"где взять доступ к Jira?"}' 2>/dev/null)

if grep -q '^event:' <<<"$STREAM"; then
    ok "POST /api/v1/chat/stream отдаёт SSE-события"
else
    code=$(json_get "d['error']['code']" <<<"$STREAM")
    if [ -n "${code:-}" ]; then
        skip "AI-сервис недоступен ($code) — ждём T-04, SSE-прокси проверен отдельно"
    else
        fail "чат вернул: $STREAM"
    fi
fi

section 'Аудит'

if command -v docker >/dev/null 2>&1 && docker compose ps postgres >/dev/null 2>&1; then
    COUNTS=$(docker compose exec -T postgres psql -U buddy -d buddy -tAc \
        "SELECT (SELECT count(*) FROM audit_log), (SELECT count(*) FROM events)" 2>/dev/null)

    AUDIT=$(cut -d'|' -f1 <<<"$COUNTS")
    EVENTS=$(cut -d'|' -f2 <<<"$COUNTS")

    if [ "${AUDIT:-0}" -gt 0 ] && [ "${EVENTS:-0}" -gt 0 ]; then
        ok "audit_log: $AUDIT записей, events: $EVENTS записей"
    else
        fail "audit_log/events пусты ($COUNTS)"
    fi
else
    skip "docker недоступен — проверка audit_log/events пропущена"
fi

section 'Фронтенд'

if body=$(curl -sS --max-time 10 "$WEB_URL/" 2>/dev/null) && grep -q 'id="root"' <<<"$body"; then
    ok "web на $WEB_URL отдаёт приложение"

    WEB_LOGIN=$(curl -sS --max-time 10 -X POST "$WEB_URL/api/v1/auth/login" \
        -H 'Content-Type: application/json' \
        -d '{"email":"dasha@demo.local"}')

    if [ -n "$(json_get "d['token']" <<<"$WEB_LOGIN")" ]; then
        ok "nginx проксирует /api на core"
    else
        fail "nginx-прокси /api не работает: $WEB_LOGIN"
    fi
else
    skip "web не поднят (make up-dev или docker compose --profile dev up -d web)"
fi

printf '\n\033[1mИтог:\033[0m %d OK, %d FAIL, %d SKIP\n' "$PASSED" "$FAILED" "$SKIPPED"

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi
