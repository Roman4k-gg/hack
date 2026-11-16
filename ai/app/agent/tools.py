import re
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from datetime import date
from typing import Any, Protocol

import httpx

from app.auth import User
from app.providers.base import ToolSpec

NO_ARGS = {"type": "object", "properties": {}, "additionalProperties": False}


class ToolError(Exception):
    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code
        self.message = message


@dataclass
class ToolResult:
    ok: bool
    data: Any = None
    error_code: str = ""
    error_message: str = ""
    citations: list[dict[str, str]] = field(default_factory=list)

    def for_model(self) -> dict[str, Any]:
        if self.ok:
            return {"ok": True, "result": self.data}
        return {"ok": False, "error": {"code": self.error_code, "message": self.error_message}}


class CoreClient:
    def __init__(
        self, base_url: str, timeout: float = 5.0, client: httpx.AsyncClient | None = None
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self._client = client

    async def _request(self, method: str, path: str, user: User, **kwargs: Any) -> httpx.Response:
        headers = {"Authorization": f"Bearer {user.token}"}
        url = f"{self.base_url}{path}"
        if self._client is not None:
            return await self._client.request(method, url, headers=headers, **kwargs)
        async with httpx.AsyncClient(timeout=self.timeout) as client:
            return await client.request(method, url, headers=headers, **kwargs)

    async def _send(self, method: str, path: str, user: User, **kwargs: Any) -> Any:
        try:
            response = await self._request(method, path, user, **kwargs)
        except httpx.TimeoutException as exc:
            raise ToolError("TIMEOUT", "Сервис не ответил вовремя") from exc
        except httpx.HTTPError as exc:
            raise ToolError("UPSTREAM_ERROR", "Сервис недоступен") from exc

        if response.status_code >= 400:
            code, message = ERROR_BY_STATUS.get(
                response.status_code,
                ("UPSTREAM_ERROR", f"Сервис вернул ошибку {response.status_code}"),
            )
            raise ToolError(code, core_error_message(response) or message)
        try:
            return response.json()
        except ValueError as exc:
            raise ToolError("UPSTREAM_ERROR", "Сервис вернул некорректный ответ") from exc

    async def get(self, path: str, user: User, params: dict[str, Any] | None = None) -> Any:
        return await self._send("GET", path, user, params=params)

    async def post(
        self, path: str, user: User, body: dict[str, Any], timeout: float | None = None
    ) -> Any:
        return await self._send("POST", path, user, json=body, timeout=timeout or self.timeout)


ERROR_BY_STATUS = {
    400: ("VALIDATION_ERROR", "Некорректный запрос"),
    401: ("FORBIDDEN", "Недостаточно прав для этого действия"),
    403: ("FORBIDDEN", "Недостаточно прав для этого действия"),
    404: ("NOT_FOUND", "Данные не найдены"),
    409: ("CONFLICT", "Конфликт с текущим состоянием"),
    504: ("TIMEOUT", "Сервис не ответил вовремя"),
}


def core_error_message(response: httpx.Response) -> str:
    try:
        error = response.json().get("error") or {}
    except (ValueError, AttributeError):
        return ""
    return str(error.get("message") or "") if isinstance(error, dict) else ""


Handler = Callable[[User, dict[str, Any], CoreClient], Awaitable[ToolResult]]


@dataclass(frozen=True)
class Tool:
    spec: ToolSpec
    handler: Handler
    scope: str = ""
    confirmed_by: str = ""
    requires_confirmation: bool = False

    @property
    def name(self) -> str:
        return self.spec.name

    def allowed_for(self, user: User) -> bool:
        return not self.scope or user.has_scope(self.scope)


async def get_my_profile(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    return ToolResult(ok=True, data=await core.get("/api/v1/me", user))


async def get_onboarding_plan(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    return ToolResult(ok=True, data=await core.get("/api/v1/onboarding/plan", user))


STOP_WORDS = {
    "как",
    "где",
    "что",
    "это",
    "мне",
    "меня",
    "мой",
    "моя",
    "мои",
    "нужно",
    "надо",
    "можно",
    "получить",
    "сделать",
    "такое",
    "для",
    "при",
    "или",
    "есть",
    "кто",
    "куда",
    "когда",
    "почему",
    "зачем",
    "сколько",
    "какой",
    "какая",
    "какие",
    "нас",
    "вас",
    "работает",
    "подскажи",
    "расскажи",
    "хочу",
}
SEARCH_LIMIT = 3
RELEVANCE_CUTOFF = 0.5
FULL_TEXT_LIMIT = 2000
DOCUMENT_ID = re.compile(r"^[A-Za-z0-9_-]{1,64}$")
ENDINGS_TEXT = (
    "ами ями ого его ому ему ыми ими ах ях ам ям ов ев ей ой ий ый ая яя ое ее ые ие ом ем "
    "ую юю а я ы и у ю е о"
)
ENDINGS = tuple(sorted(ENDINGS_TEXT.split(), key=len, reverse=True))
CYRILLIC = re.compile(r"^[а-яё-]+$")


def stem(word: str) -> str:
    if len(word) < 5 or not CYRILLIC.match(word):
        return word
    for ending in ENDINGS:
        if word.endswith(ending) and len(word) - len(ending) >= 4:
            return word[: -len(ending)]
    return word


def keywords(query: str, limit: int = 4) -> list[str]:
    words = re.findall(r"[\w-]+", query.lower())
    result: list[str] = []
    for word in words:
        term = stem(word)
        if len(word) >= 3 and word not in STOP_WORDS and term not in result:
            result.append(term)
    return result[:limit]


def search_item(raw: dict[str, Any]) -> dict[str, Any]:
    return {
        "document_id": str(raw.get("document_id", "")),
        "title": raw.get("title", ""),
        "summary": raw.get("summary", ""),
        "url": raw.get("url", ""),
        "chunk": raw.get("chunk", ""),
        "updated_at": raw.get("updated_at", ""),
        "stale": bool(raw.get("stale", False)),
        "score": float(raw.get("score", 0) or 0),
    }


async def with_full_text(item: dict[str, Any], user: User, core: CoreClient) -> dict[str, Any]:
    document_id = item["document_id"]
    if not DOCUMENT_ID.match(document_id):
        return item
    try:
        document = await core.get(f"/api/v1/kb/documents/{document_id}", user)
    except ToolError:
        return item
    content = str(document.get("content") or "").strip()
    if not content:
        return item
    return {
        **item,
        "chunk": content[:FULL_TEXT_LIMIT],
        "stale": bool(document.get("stale", item["stale"])),
        "owner": str(document.get("owner") or ""),
    }


async def search_knowledge(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    query = str(args.get("query") or "").strip()
    if not query:
        raise ToolError("VALIDATION_ERROR", "Пустой поисковый запрос")

    found: dict[str, dict[str, Any]] = {}
    relevance: dict[str, float] = {}
    for term in [query, *keywords(query)]:
        response = await core.get("/api/v1/kb/search", user, params={"q": term, "limit": 5})
        for raw in response.get("items") or []:
            item = search_item(raw)
            document_id = item["document_id"]
            current = found.get(document_id)
            if current is None or item["score"] > current["score"]:
                found[document_id] = item
            relevance[document_id] = relevance.get(document_id, 0.0) + item["score"]
        if found and term == query:
            break

    ranked = sorted(found.values(), key=lambda item: relevance[item["document_id"]], reverse=True)
    best = relevance[ranked[0]["document_id"]] if ranked else 0.0
    items = [
        item
        for item in ranked[:SEARCH_LIMIT]
        if relevance[item["document_id"]] >= best * RELEVANCE_CUTOFF
    ]
    if items:
        items[0] = await with_full_text(items[0], user, core)
    citations = [
        {
            "id": item["document_id"],
            "title": item["title"],
            "url": item["url"],
            **({"stale": True} if item["stale"] else {}),
        }
        for item in items
    ]
    return ToolResult(ok=True, data={"query": query, "items": items}, citations=citations)


ABSENCE_TYPES = ("vacation", "day_off", "sick_leave")
PREVIEW_TIMEOUT = 10.0


def parse_day(value: Any, field_name: str) -> date:
    try:
        return date.fromisoformat(str(value))
    except ValueError as exc:
        message = f"{field_name}: нужна дата в формате ГГГГ-ММ-ДД"
        raise ToolError("VALIDATION_ERROR", message) from exc


async def preview_absence_request(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    date_from = parse_day(args.get("date_from"), "date_from")
    date_to = parse_day(args.get("date_to") or args.get("date_from"), "date_to")
    if date_to < date_from:
        raise ToolError("VALIDATION_ERROR", "Дата окончания раньше даты начала")
    absence_type = str(args.get("absence_type") or "day_off")
    if absence_type not in ABSENCE_TYPES:
        raise ToolError("VALIDATION_ERROR", f"Тип отсутствия должен быть одним из {ABSENCE_TYPES}")
    comment = str(args.get("comment") or "")[:500]
    body = {
        "date_from": date_from.isoformat(),
        "date_to": date_to.isoformat(),
        "absence_type": absence_type,
        "comment": comment,
    }
    data = await core.post("/api/v1/requests/absence/preview", user, body, timeout=PREVIEW_TIMEOUT)
    return ToolResult(ok=True, data=data)


def confirmation_body(args: dict[str, Any]) -> dict[str, str]:
    body = {key: str(args.get(key) or "") for key in CONFIRMATION_FIELDS}
    if not all(body.values()):
        raise ToolError("CONFIRMATION_REQUIRED", "Нужен предпросмотр и подтверждение пользователя")
    return body


async def create_absence_request(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    body = confirmation_body(args)
    return ToolResult(ok=True, data=await core.post("/api/v1/requests/absence", user, body))


BUNDLE_PATTERN = re.compile(r"^[a-z][a-z0-9_-]{0,31}$")


async def preview_access_bundle(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    bundle = str(args.get("bundle") or "intern").strip().lower()
    if not BUNDLE_PATTERN.match(bundle):
        raise ToolError("VALIDATION_ERROR", "Неизвестный пакет доступов")
    data = await core.post(
        "/api/v1/requests/access/preview", user, {"bundle": bundle}, timeout=PREVIEW_TIMEOUT
    )
    return ToolResult(ok=True, data=data)


async def create_access_requests(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    body = confirmation_body(args)
    return ToolResult(ok=True, data=await core.post("/api/v1/requests/access", user, body))


HANDOFF_QUESTION_LIMIT = 1000


async def preview_handoff(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    question = " ".join(str(args.get("question") or "").split())
    if len(question) < 3:
        raise ToolError("VALIDATION_ERROR", "Нужен вопрос, который передать наставнику")
    payload = {
        "question": question[:HANDOFF_QUESTION_LIMIT],
        "context": " ".join(str(args.get("context") or "").split())[:HANDOFF_QUESTION_LIMIT],
    }
    return ToolResult(
        ok=True,
        data={
            "preview_id": uuid.uuid4().hex,
            "confirmation_token": uuid.uuid4().hex,
            "draft": payload,
            "payload": payload,
        },
    )


def person_name(value: Any) -> str:
    if isinstance(value, dict):
        return str(value.get("full_name") or "")
    return str(value or "")


async def create_handoff(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    question = str(args.get("question") or "").strip()
    idempotency_key = str(args.get("idempotency_key") or "").strip()
    if not question or not idempotency_key:
        raise ToolError("CONFIRMATION_REQUIRED", "Нужен предпросмотр и подтверждение пользователя")
    body = {
        "question": question,
        "context": str(args.get("context") or ""),
        "idempotency_key": idempotency_key,
    }
    data = await core.post("/api/v1/handoffs", user, body)
    return ToolResult(
        ok=True,
        data={
            "request_id": data.get("request_id", ""),
            "status": data.get("status", "pending"),
            "external_id": data.get("external_id", ""),
            "mentor": person_name(data.get("mentor") or data.get("approver")),
            "next_step": data.get("next_step", ""),
        },
    )


async def get_my_requests(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    data = await core.get("/api/v1/requests", user)
    items = [
        {
            "request_id": item.get("request_id", ""),
            "kind": item.get("kind", ""),
            "title": item.get("title", ""),
            "status": item.get("status", ""),
            "external_id": item.get("external_id", ""),
            "approver": (item.get("approver") or {}).get("full_name", ""),
            "updated_at": item.get("updated_at", ""),
        }
        for item in (data.get("items") or [])
    ]
    return ToolResult(ok=True, data={"items": items})


CONFIRMATION_FIELDS = ("preview_id", "confirmation_token", "idempotency_key")


DEFAULT_TOOLS = [
    Tool(
        spec=ToolSpec(
            name="get_my_profile",
            description="Профиль текущего пользователя: имя, роль, команда, должность.",
            parameters=NO_ARGS,
        ),
        handler=get_my_profile,
    ),
    Tool(
        spec=ToolSpec(
            name="get_onboarding_plan",
            description=(
                "Карта адаптации стажёра: этапы, пункты и их статусы, прогресс "
                "и следующий шаг. Используй, когда спрашивают, что делать дальше."
            ),
            parameters=NO_ARGS,
        ),
        handler=get_onboarding_plan,
        scope="onboarding:plan:read",
    ),
    Tool(
        spec=ToolSpec(
            name="search_knowledge",
            description=(
                "Поиск по базе знаний команды: доступы, инструменты, процессы, отгулы. "
                "Возвращает документы с цитатой, ссылкой и датой обновления. "
                "В query передавай короткий запрос из ключевых слов, например «доступ Jira»."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "query": {"type": "string", "minLength": 1, "maxLength": 200},
                },
                "required": ["query"],
                "additionalProperties": False,
            },
        ),
        handler=search_knowledge,
        scope="kb:read",
    ),
    Tool(
        spec=ToolSpec(
            name="preview_absence_request",
            description=(
                "Готовит черновик заявки на отгул, отпуск или больничный и показывает правила. "
                "Заявку НЕ создаёт. После черновика попроси пользователя явно подтвердить."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "date_from": {"type": "string", "format": "date"},
                    "date_to": {"type": "string", "format": "date"},
                    "absence_type": {"type": "string", "enum": list(ABSENCE_TYPES)},
                    "comment": {"type": "string", "maxLength": 500},
                },
                "required": ["date_from", "date_to", "absence_type"],
                "additionalProperties": False,
            },
        ),
        handler=preview_absence_request,
        scope="absence:self",
        confirmed_by="create_absence_request",
    ),
    Tool(
        spec=ToolSpec(
            name="create_absence_request",
            description=(
                "Отправляет заявку на отсутствие по последнему черновику. Вызывай только если "
                "пользователь в новом сообщении явно подтвердил черновик. Аргументы не нужны."
            ),
            parameters=NO_ARGS,
        ),
        handler=create_absence_request,
        scope="absence:self:write",
        requires_confirmation=True,
    ),
    Tool(
        spec=ToolSpec(
            name="preview_access_bundle",
            description=(
                "Готовит черновик пакета доступов для роли (VPN, Jira, база знаний и т.д.): "
                "какие системы, зачем и кто согласует. Заявки НЕ создаёт. "
                "После черновика попроси пользователя явно подтвердить."
            ),
            parameters={
                "type": "object",
                "properties": {"bundle": {"type": "string", "default": "intern"}},
                "additionalProperties": False,
            },
        ),
        handler=preview_access_bundle,
        scope="access:self",
        confirmed_by="create_access_requests",
    ),
    Tool(
        spec=ToolSpec(
            name="create_access_requests",
            description=(
                "Отправляет заявки на доступы по последнему черновику пакета. Вызывай только если "
                "пользователь в новом сообщении явно подтвердил черновик. Аргументы не нужны."
            ),
            parameters=NO_ARGS,
        ),
        handler=create_access_requests,
        scope="access:self:write",
        requires_confirmation=True,
    ),
    Tool(
        spec=ToolSpec(
            name="get_my_requests",
            description="Список заявок пользователя (отгулы, доступы) с их статусами.",
            parameters=NO_ARGS,
        ),
        handler=get_my_requests,
        scope="request:self",
    ),
    Tool(
        spec=ToolSpec(
            name="preview_handoff",
            description=(
                "Готовит черновик вопроса наставнику, когда ответа нет в базе знаний или "
                "пользователь просит передать вопрос. Ничего не отправляет. После черновика "
                "попроси пользователя явно подтвердить."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "question": {"type": "string", "maxLength": HANDOFF_QUESTION_LIMIT},
                    "context": {"type": "string", "maxLength": HANDOFF_QUESTION_LIMIT},
                },
                "required": ["question"],
                "additionalProperties": False,
            },
        ),
        handler=preview_handoff,
        scope="request:self",
        confirmed_by="create_handoff",
    ),
    Tool(
        spec=ToolSpec(
            name="create_handoff",
            description=(
                "Отправляет наставнику вопрос из последнего черновика. Вызывай только если "
                "пользователь в новом сообщении явно подтвердил. Аргументы не нужны."
            ),
            parameters=NO_ARGS,
        ),
        handler=create_handoff,
        scope="request:self",
        requires_confirmation=True,
    ),
]


class ToolBackend(Protocol):
    def available(self, user: User) -> list[Tool]: ...

    async def call(self, name: str, args: dict[str, Any], user: User) -> ToolResult: ...


class ToolRegistry:
    def __init__(self, core: CoreClient, tools: list[Tool] | None = None) -> None:
        self.core = core
        self._tools = {tool.name: tool for tool in (tools if tools is not None else DEFAULT_TOOLS)}

    def available(self, user: User) -> list[Tool]:
        return [tool for tool in self._tools.values() if tool.allowed_for(user)]

    async def call(self, name: str, args: dict[str, Any], user: User) -> ToolResult:
        tool = self._tools.get(name)
        if tool is None or not tool.allowed_for(user):
            return ToolResult(
                ok=False,
                error_code="FORBIDDEN",
                error_message=f"Инструмент {name} недоступен",
            )
        try:
            return await tool.handler(user, args, self.core)
        except ToolError as exc:
            return ToolResult(ok=False, error_code=exc.code, error_message=exc.message)
