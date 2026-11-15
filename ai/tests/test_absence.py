import json
from datetime import date

import httpx
import httpx2
import pytest

from app.agent.loop import Agent
from app.agent.mcp_tools import MCPToolRegistry
from app.agent.pending import PendingStore
from app.agent.tools import CoreClient, ToolRegistry
from app.auth import User
from app.mcp.server import build_app, build_server
from app.providers import MockProvider
from app.providers.base import LLMResponse, ToolCall, ToolSpec
from app.providers.mock import ASK_DATE_TEXT, CONFIRMATION_TEXT, DECLINED_TEXT
from app.providers.mock_dates import absence_type, find_dates, today_from_system
from tests.conftest import make_token

SCOPES = ["kb:read", "absence:self", "absence:self:write", "request:self"]
PREVIEW = {
    "preview_id": "prev-1",
    "confirmation_token": "secret-token",
    "expires_at": "2026-11-20T10:10:00Z",
    "draft": {
        "date_from": "2026-11-20",
        "date_to": "2026-11-20",
        "absence_type": "day_off",
        "comment": "",
    },
    "policy_notes": ["Отгул согласует руководитель"],
}
CREATED = {
    "request_id": "req-1",
    "kind": "absence",
    "title": "Отгул 20.11.2026",
    "status": "pending",
    "external_id": "HR-101",
    "approver": {"full_name": "Иван Петров"},
    "next_step": "Дождись согласования руководителя.",
}
REQUESTS = {
    "items": [
        {
            "request_id": "req-1",
            "kind": "absence",
            "title": "Отгул 20.11.2026",
            "status": "approved",
            "external_id": "HR-101",
            "approver": {"full_name": "Иван Петров"},
            "updated_at": "2026-11-16T10:00:00Z",
        }
    ]
}


@pytest.fixture
def employee() -> User:
    return User(id="u-1", role="intern", token="intern-token", scopes=SCOPES)


class FakeCore:
    def __init__(self, create_status: int = 200) -> None:
        self.create_status = create_status
        self.requests: list[tuple[str, str, dict]] = []

    def __call__(self, request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content) if request.content else {}
        self.requests.append((request.method, request.url.path, body))
        if request.url.path == "/api/v1/requests/absence/preview":
            return httpx.Response(200, json=PREVIEW)
        if request.url.path == "/api/v1/requests/absence":
            if self.create_status != 200:
                return httpx.Response(
                    self.create_status, json={"error": {"message": "Токен уже использован"}}
                )
            return httpx.Response(200, json=CREATED)
        if request.url.path == "/api/v1/requests":
            return httpx.Response(200, json=REQUESTS)
        return httpx.Response(404)

    def paths(self) -> list[str]:
        return [path for _, path, _ in self.requests]


def registry_for(core: FakeCore) -> ToolRegistry:
    client = httpx.AsyncClient(transport=httpx.MockTransport(core))
    return ToolRegistry(CoreClient("http://core.test", client=client))


async def collect(agent: Agent, user: User, message: str) -> list[dict]:
    return [event async for event in agent.run(user, message)]


def text_of(events: list[dict]) -> str:
    return "".join(event["text"] for event in events if event["type"] == "delta")


def tool_names(events: list[dict]) -> list[str]:
    return [
        event["toolCall"]["name"]
        for event in events
        if event["type"] == "tool_call" and event["toolCall"]["status"] == "running"
    ]


async def test_preview_sends_normalized_body(employee: User):
    core = FakeCore()
    result = await registry_for(core).call(
        "preview_absence_request",
        {"date_from": "2026-11-20", "absence_type": "day_off", "comment": "x" * 600},
        employee,
    )

    assert result.ok
    assert result.data == PREVIEW
    method, path, body = core.requests[0]
    assert (method, path) == ("POST", "/api/v1/requests/absence/preview")
    assert body["date_to"] == "2026-11-20"
    assert len(body["comment"]) == 500


@pytest.mark.parametrize(
    "args",
    [
        {"date_from": "завтра"},
        {"date_from": "2026-11-22", "date_to": "2026-11-20"},
        {"date_from": "2026-11-20", "absence_type": "holiday"},
    ],
)
async def test_preview_validates_arguments(employee: User, args: dict):
    core = FakeCore()
    result = await registry_for(core).call("preview_absence_request", args, employee)

    assert result.error_code == "VALIDATION_ERROR"
    assert core.requests == []


async def test_create_requires_confirmation_fields(employee: User):
    core = FakeCore()
    result = await registry_for(core).call("create_absence_request", {}, employee)

    assert result.error_code == "CONFIRMATION_REQUIRED"
    assert core.requests == []


async def test_create_maps_core_error_message(employee: User):
    core = FakeCore(create_status=400)
    args = {"preview_id": "p", "confirmation_token": "t", "idempotency_key": "p"}
    result = await registry_for(core).call("create_absence_request", args, employee)

    assert result.error_code == "VALIDATION_ERROR"
    assert result.error_message == "Токен уже использован"


async def test_get_my_requests_is_normalized(employee: User):
    result = await registry_for(FakeCore()).call("get_my_requests", {}, employee)

    assert result.data["items"][0]["approver"] == "Иван Петров"
    assert result.data["items"][0]["status"] == "approved"


def test_pending_store_expires_and_hides_token():
    now = [100.0]
    store = PendingStore(ttl=10, clock=lambda: now[0])
    store.put("u", "create_absence_request", PREVIEW, "turn-1")

    action = store.get("u")
    assert action is not None
    assert "confirmation_token" not in action.draft
    assert action.confirm_args() == {
        "preview_id": "prev-1",
        "confirmation_token": "secret-token",
        "idempotency_key": "prev-1",
    }

    now[0] = 111.0
    assert store.get("u") is None
    assert store.get("missing") is None
    store.pop("missing")


class ScriptedProvider:
    name = "scripted"

    def __init__(self, responses: list[LLMResponse]) -> None:
        self.responses = list(responses)
        self.calls: list[list[dict]] = []

    async def complete(self, messages: list[dict], tools: list[ToolSpec]) -> LLMResponse:
        self.calls.append([dict(message) for message in messages])
        return self.responses.pop(0)


def call(name: str, arguments: dict | None = None) -> LLMResponse:
    return LLMResponse(tool_calls=[ToolCall(id="", name=name, arguments=arguments or {})])


async def test_create_in_same_turn_as_preview_is_rejected(employee: User):
    core = FakeCore()
    provider = ScriptedProvider(
        [
            call("preview_absence_request", {"date_from": "2026-11-20"}),
            call("create_absence_request"),
            LLMResponse(text="ok"),
        ]
    )
    agent = Agent(provider, registry_for(core))

    await collect(agent, employee, "отгул 20.11")

    assert core.paths() == ["/api/v1/requests/absence/preview"]
    preview_message = json.loads(provider.calls[1][-1]["content"])
    assert preview_message["result"]["awaiting_confirmation"] is True
    assert "confirmation_token" not in json.dumps(provider.calls[1], ensure_ascii=False)
    rejected = json.loads(provider.calls[2][-1]["content"])
    assert rejected["error"]["code"] == "CONFIRMATION_REQUIRED"
    assert agent.pending.get(employee.id) is not None


async def test_create_in_next_turn_uses_hidden_token(employee: User):
    core = FakeCore()
    provider = ScriptedProvider(
        [
            call("preview_absence_request", {"date_from": "2026-11-20"}),
            LLMResponse(text="Отправить?"),
            call("create_absence_request"),
            LLMResponse(text="Готово"),
        ]
    )
    agent = Agent(provider, registry_for(core))

    await collect(agent, employee, "отгул 20.11")
    await collect(agent, employee, "подтверждаю")

    system = provider.calls[2][0]["content"]
    assert "Инструмент для подтверждения: create_absence_request" in system
    assert "secret-token" not in system
    assert core.requests[-1][2] == {
        "preview_id": "prev-1",
        "confirmation_token": "secret-token",
        "idempotency_key": "prev-1",
    }
    assert agent.pending.get(employee.id) is None


@pytest.mark.parametrize(("status", "kept"), [(502, True), (400, False)])
async def test_pending_kept_only_for_retryable_errors(employee: User, status: int, kept: bool):
    core = FakeCore(create_status=status)
    provider = ScriptedProvider(
        [
            call("preview_absence_request", {"date_from": "2026-11-20"}),
            LLMResponse(text="Отправить?"),
            call("create_absence_request"),
            LLMResponse(text="Ошибка"),
        ]
    )
    agent = Agent(provider, registry_for(core))

    await collect(agent, employee, "отгул 20.11")
    await collect(agent, employee, "да")

    assert (agent.pending.get(employee.id) is not None) is kept


def test_find_dates_formats():
    today = date(2026, 11, 15)

    assert find_dates("отгул 2026-11-20", today) == [date(2026, 11, 20)]
    assert find_dates("отгул 20.11", today) == [date(2026, 11, 20)]
    assert find_dates("отгул 10.11", today) == [date(2027, 11, 10)]
    assert find_dates("отгул 01.02.2027", today) == [date(2027, 2, 1)]
    assert find_dates("отпуск с 20 по 24 ноября", today) == [
        date(2026, 11, 20),
        date(2026, 11, 24),
    ]
    assert find_dates("отпуск 20-22 ноября", today) == [date(2026, 11, 20), date(2026, 11, 22)]
    assert find_dates("отгул 3 декабря", today) == [date(2026, 12, 3)]
    assert find_dates("отгул завтра", today) == [date(2026, 11, 16)]
    assert find_dates("отгул послезавтра", today) == [date(2026, 11, 17)]
    assert find_dates("заболел сегодня", today) == [today]
    assert find_dates("отгул в пятницу", today) == [date(2026, 11, 20)]
    assert find_dates("отгул в воскресенье", today) == [date(2026, 11, 22)]
    assert find_dates("отгул 31.02", today) == []
    assert find_dates("нужен отгул", today) == []


def test_today_and_absence_type():
    system = [{"role": "system", "content": "Контекст:\n- Сегодня: 2026-11-15 (воскресенье)"}]
    assert today_from_system(system) == date(2026, 11, 15)
    assert today_from_system([]) == date.today()
    assert absence_type("я заболел") == "sick_leave"
    assert absence_type("хочу в отпуск") == "vacation"
    assert absence_type("отгул") == "day_off"


ABSENCE_TOOLS = [
    ToolSpec(name, "", {"type": "object"})
    for name in (
        "search_knowledge",
        "preview_absence_request",
        "create_absence_request",
        "get_my_requests",
    )
]
SYSTEM = {"role": "system", "content": "- Сегодня: 2026-11-15 (воскресенье)"}
PENDING_SYSTEM = {
    "role": "system",
    "content": "- Сегодня: 2026-11-15\n- Инструмент для подтверждения: create_absence_request",
}


async def ask(question: str, system: dict = SYSTEM) -> LLMResponse:
    messages = [system, {"role": "user", "content": question}]
    return await MockProvider().complete(messages, ABSENCE_TOOLS)


async def test_mock_previews_absence_with_dates():
    response = await ask("Хочу отпуск с 20 по 24 ноября")

    assert response.tool_calls[0].name == "preview_absence_request"
    assert response.tool_calls[0].arguments == {
        "date_from": "2026-11-20",
        "date_to": "2026-11-24",
        "absence_type": "vacation",
        "comment": "",
    }


async def test_mock_absence_without_dates():
    assert (await ask("Нужен отгул")).text == ASK_DATE_TEXT
    response = await ask("Как оформить отгул?")
    assert response.tool_calls[0].name == "search_knowledge"


async def test_mock_confirms_and_declines_pending():
    assert (await ask("Да, подтверждаю", PENDING_SYSTEM)).tool_calls[0].name == (
        "create_absence_request"
    )
    assert (await ask("нет, передумал", PENDING_SYSTEM)).text == DECLINED_TEXT
    assert (await ask("подтверждаю")).tool_calls[0].name == "search_knowledge"


async def test_mock_lists_requests():
    assert (await ask("Покажи мои заявки")).tool_calls[0].name == "get_my_requests"


def answer(name: str, payload: dict) -> list[dict]:
    return [
        SYSTEM,
        {"role": "user", "content": "q"},
        {"role": "assistant", "tool_calls": [{"id": "c1", "function": {"name": name}}]},
        {"role": "tool", "tool_call_id": "c1", "content": json.dumps(payload)},
    ]


async def mock_text(name: str, payload: dict) -> str:
    return (await MockProvider().complete(answer(name, payload), ABSENCE_TOOLS)).text


async def test_mock_answer_texts():
    preview = await mock_text(
        "preview_absence_request",
        {
            "ok": True,
            "result": {
                "draft": {
                    "date_from": "2026-11-20",
                    "date_to": "2026-11-24",
                    "absence_type": "vacation",
                    "comment": "Поездка",
                },
                "policy_notes": ["Отпуск согласует руководитель"],
            },
        },
    )
    assert "20.11.2026 – 24.11.2026" in preview
    assert "Отпуск" in preview and "Поездка" in preview
    assert "подтверждаю" in preview

    created = await mock_text("create_absence_request", {"ok": True, "result": CREATED})
    assert "HR-101" in created and "на согласовании" in created and "Иван Петров" in created

    listed = await mock_text(
        "get_my_requests",
        {"ok": True, "result": {"items": [{"title": "Отгул", "status": "approved"}]}},
    )
    assert "Отгул — согласована" in listed
    empty = await mock_text("get_my_requests", {"ok": True, "result": {"items": []}})
    assert empty == "У тебя пока нет заявок."

    required = {"ok": False, "error": {"code": "CONFIRMATION_REQUIRED"}}
    assert await mock_text("create_absence_request", required) == CONFIRMATION_TEXT
    invalid = {"ok": False, "error": {"code": "VALIDATION_ERROR", "message": "плохая дата"}}
    assert "плохая дата" in await mock_text("preview_absence_request", invalid)

    bad_date = {"ok": True, "result": {"draft": {"date_from": "скоро"}}}
    assert "скоро" in await mock_text("preview_absence_request", bad_date)


async def test_absence_demo_scenario(employee: User):
    core = FakeCore()
    agent = Agent(MockProvider(), registry_for(core))

    first = await collect(agent, employee, "Хочу взять отгул 20.11.2026")
    assert tool_names(first) == ["preview_absence_request"]
    assert "Подготовил черновик заявки" in text_of(first)
    assert "/api/v1/requests/absence" not in core.paths()

    second = await collect(agent, employee, "Подтверждаю")
    assert tool_names(second) == ["create_absence_request"]
    assert "Заявка отправлена ✅" in text_of(second)
    assert core.requests[-1][2]["idempotency_key"] == "prev-1"

    third = await collect(agent, employee, "Подтверждаю")
    assert tool_names(third) == ["search_knowledge"]

    status = await collect(agent, employee, "Что с моей заявкой? Покажи мои заявки")
    assert tool_names(status) == ["get_my_requests"]
    assert "согласована" in text_of(status)


async def test_absence_flow_over_mcp():
    core = FakeCore()
    server = build_server(registry_for(core))
    app = build_app(server)

    def factory(headers: dict[str, str], timeout: float) -> httpx2.AsyncClient:
        return httpx2.AsyncClient(
            transport=httpx2.ASGITransport(app=app), headers=headers, timeout=timeout
        )

    user = User(id="u-1", role="intern", token=make_token(scopes=SCOPES), scopes=SCOPES)
    registry = MCPToolRegistry(
        "http://mcp.test/mcp", fallback=registry_for(FakeCore()), http_client_factory=factory
    )
    agent = Agent(MockProvider(), registry)
    async with server.session_manager.run():
        await collect(agent, user, "Отгул 20.11.2026")
        done = await collect(agent, user, "да, отправляй")

    assert "HR-101" in text_of(done)
    assert core.paths() == ["/api/v1/requests/absence/preview", "/api/v1/requests/absence"]
    assert core.requests[1][2]["confirmation_token"] == "secret-token"
