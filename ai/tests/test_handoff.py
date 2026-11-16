import json

import httpx
import httpx2
import pytest

from app.agent import tools
from app.agent.loop import Agent
from app.agent.mcp_tools import MCPToolRegistry
from app.agent.tools import CoreClient, ToolRegistry
from app.auth import User
from app.mcp.server import build_app, build_server
from app.providers import MockProvider
from app.providers.base import LLMResponse, ToolCall, ToolSpec
from app.providers.mock import HANDOFF_NEXT_TEXT, HANDOFF_UNAVAILABLE_TEXT
from tests.conftest import make_token

SCOPES = ["kb:read", "request:self"]


@pytest.fixture
def intern_with_requests() -> User:
    return User(id="u-3", role="intern", token="intern-token", scopes=SCOPES)


class FakeCore:
    def __init__(self, handoffs: bool = True) -> None:
        self.handoffs = handoffs
        self.requests: list[tuple[str, dict]] = []

    def __call__(self, request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content) if request.content else {}
        self.requests.append((request.url.path, body))
        if request.url.path == "/api/v1/kb/search":
            return httpx.Response(200, json={"items": []})
        if request.url.path == "/api/v1/handoffs" and self.handoffs:
            return httpx.Response(
                200,
                json={
                    "request_id": "r1",
                    "status": "pending",
                    "external_id": "HND-1",
                    "mentor": {"full_name": "Пётр Иванов"},
                    "next_step": "Пётр ответит сегодня",
                },
            )
        return httpx.Response(404, json={"error": {"message": "нет такого"}})

    def bodies(self, path: str) -> list[dict]:
        return [body for seen, body in self.requests if seen == path]


def registry_for(core: FakeCore) -> ToolRegistry:
    client = httpx.AsyncClient(transport=httpx.MockTransport(core))
    return ToolRegistry(CoreClient("http://core.test", client=client))


async def collect(agent: Agent, user: User, message: str) -> list[dict]:
    return [event async for event in agent.run(user, message)]


def text_of(events: list[dict]) -> str:
    return "".join(event["text"] for event in events if event["type"] == "delta")


def called(events: list[dict]) -> list[str]:
    return [
        event["toolCall"]["name"]
        for event in events
        if event["type"] == "tool_call" and event["toolCall"]["status"] == "running"
    ]


async def test_preview_handoff_builds_local_draft(intern_with_requests: User):
    core = FakeCore()
    result = await registry_for(core).call(
        "preview_handoff",
        {"question": "  Где   взять доступ к стенду?  ", "context": "x" * 1200},
        intern_with_requests,
    )

    assert result.ok
    assert core.requests == []
    assert result.data["draft"]["question"] == "Где взять доступ к стенду?"
    assert len(result.data["payload"]["context"]) == 1000
    assert result.data["preview_id"] and result.data["confirmation_token"]


@pytest.mark.parametrize("question", ["", "  ", "ок"])
async def test_preview_handoff_needs_question(intern_with_requests: User, question: str):
    result = await registry_for(FakeCore()).call(
        "preview_handoff", {"question": question}, intern_with_requests
    )
    assert result.error_code == "VALIDATION_ERROR"


async def test_create_handoff_posts_and_normalizes(intern_with_requests: User):
    core = FakeCore()
    args = {"question": "Вопрос?", "context": "контекст", "idempotency_key": "k1"}

    result = await registry_for(core).call("create_handoff", args, intern_with_requests)

    assert core.bodies("/api/v1/handoffs") == [args]
    assert result.data == {
        "request_id": "r1",
        "status": "pending",
        "external_id": "HND-1",
        "mentor": "Пётр Иванов",
        "next_step": "Пётр ответит сегодня",
    }


async def test_create_handoff_requires_confirmation_and_maps_missing_endpoint(
    intern_with_requests: User,
):
    core = FakeCore(handoffs=False)
    registry = registry_for(core)

    missing = await registry.call("create_handoff", {"question": "Вопрос?"}, intern_with_requests)
    assert missing.error_code == "CONFIRMATION_REQUIRED"
    assert core.requests == []

    args = {"question": "Вопрос?", "idempotency_key": "k1"}
    unavailable = await registry.call("create_handoff", args, intern_with_requests)
    assert unavailable.error_code == "NOT_FOUND"
    assert unavailable.error_message == tools.HANDOFF_UNAVAILABLE_TEXT


@pytest.mark.parametrize(
    ("mentor", "expected"),
    [("Пётр", "Пётр"), (None, "Ольга")],
)
async def test_create_handoff_accepts_mentor_as_text_or_approver(
    intern_with_requests: User, mentor, expected: str
):
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"mentor": mentor, "approver": {"full_name": "Ольга"}})

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    registry = ToolRegistry(CoreClient("http://core.test", client=client))
    args = {"question": "Вопрос?", "idempotency_key": "k1"}

    result = await registry.call("create_handoff", args, intern_with_requests)

    assert result.data["mentor"] == expected
    assert result.data["status"] == "pending"


class ScriptedProvider:
    name = "scripted"

    def __init__(self, responses):
        self.responses = list(responses)
        self.calls = []

    async def complete(self, messages, tools):
        self.calls.append([dict(message) for message in messages])
        return self.responses.pop(0)


def call(name: str, arguments: dict | None = None) -> LLMResponse:
    return LLMResponse(tool_calls=[ToolCall(id="", name=name, arguments=arguments or {})])


async def test_agent_sends_handoff_only_in_next_turn(intern_with_requests: User):
    core = FakeCore()
    provider = ScriptedProvider(
        [
            call("preview_handoff", {"question": "Где стенд?", "context": "искал в базе"}),
            call("create_handoff"),
            LLMResponse(text="Подготовил"),
            call("create_handoff"),
            LLMResponse(text="Отправил"),
        ]
    )
    agent = Agent(provider, registry_for(core))

    await collect(agent, intern_with_requests, "Где стенд?")
    assert core.bodies("/api/v1/handoffs") == []
    preview_for_model = json.loads(provider.calls[1][-1]["content"])["result"]
    assert set(preview_for_model) == {"draft", "awaiting_confirmation", "confirm_tool"}

    await collect(agent, intern_with_requests, "да")
    [body] = core.bodies("/api/v1/handoffs")
    assert body["question"] == "Где стенд?"
    assert body["context"] == "искал в базе"
    assert len(body["idempotency_key"]) == 32
    assert agent.pending.get(intern_with_requests.id) is None


async def test_mock_offers_handoff_when_nothing_found(intern_with_requests: User):
    core = FakeCore()
    agent = Agent(MockProvider(), registry_for(core))

    offer = await collect(agent, intern_with_requests, "Какой пароль от Wi-Fi?")
    assert called(offer) == ["search_knowledge", "preview_handoff"]
    assert text_of(offer).startswith("Не нашёл ответа в базе знаний.")
    assert "> Какой пароль от Wi-Fi?" in text_of(offer)

    sent = await collect(agent, intern_with_requests, "да")
    assert called(sent) == ["create_handoff"]
    assert "Вопрос передан наставнику ✅" in text_of(sent)
    assert "Пётр ответит сегодня" in text_of(sent)


async def test_mock_explains_when_core_has_no_handoffs(intern_with_requests: User):
    agent = Agent(MockProvider(), registry_for(FakeCore(handoffs=False)))

    await collect(agent, intern_with_requests, "Какой пароль от Wi-Fi?")
    sent = await collect(agent, intern_with_requests, "да")

    assert text_of(sent) == HANDOFF_UNAVAILABLE_TEXT
    assert agent.pending.get(intern_with_requests.id) is None


async def test_mock_explicit_handoff_uses_previous_question():
    tools = [ToolSpec("preview_handoff", "", {"type": "object"})]
    messages = [
        {"role": "user", "content": "Во сколько дейлик?"},
        {"role": "assistant", "content": "В 10:00."},
        {"role": "user", "content": "Передай наставнику"},
    ]
    response = await MockProvider().complete(messages, tools)
    assert response.tool_calls[0].arguments == {"question": "Во сколько дейлик?"}

    alone = [{"role": "user", "content": "Спроси у наставника, можно ли работать из дома"}]
    response = await MockProvider().complete(alone, tools)
    assert response.tool_calls[0].arguments["question"].startswith("Спроси у наставника")


async def test_mock_handoff_texts_without_optional_fields():
    messages = [
        {"role": "user", "content": "да"},
        {"role": "assistant", "tool_calls": [{"id": "c", "function": {"name": "create_handoff"}}]},
        {"role": "tool", "tool_call_id": "c", "content": json.dumps({"ok": True, "result": {}})},
    ]
    text = (await MockProvider().complete(messages, [])).text
    assert text == f"Вопрос передан наставнику ✅\n\n{HANDOFF_NEXT_TEXT}"


async def test_handoff_flow_over_mcp():
    core = FakeCore()
    server = build_server(registry_for(core))
    app = build_app(server)

    def factory(headers: dict[str, str], timeout: float) -> httpx2.AsyncClient:
        return httpx2.AsyncClient(
            transport=httpx2.ASGITransport(app=app), headers=headers, timeout=timeout
        )

    user = User(id="u-3", role="intern", token=make_token(scopes=SCOPES), scopes=SCOPES)
    registry = MCPToolRegistry(
        "http://mcp.test/mcp", fallback=registry_for(FakeCore()), http_client_factory=factory
    )
    agent = Agent(MockProvider(), registry)
    async with server.session_manager.run():
        await collect(agent, user, "Какой пароль от Wi-Fi?")
        sent = await collect(agent, user, "подтверждаю")

    assert "HND-1" in text_of(sent)
    assert core.bodies("/api/v1/handoffs")[0]["question"] == "Какой пароль от Wi-Fi?"
