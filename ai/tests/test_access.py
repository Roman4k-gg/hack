import json

import httpx
import httpx2
import pytest

from app.agent.loop import Agent
from app.agent.mcp_tools import MCPToolRegistry
from app.agent.tools import CoreClient, ToolRegistry
from app.auth import User
from app.mcp.server import build_app, build_server
from app.providers import MockProvider
from app.providers.base import LLMResponse, ToolSpec
from tests.conftest import make_token

SCOPES = ["kb:read", "access:self", "access:self:write", "request:self"]
PREVIEW = {
    "preview_id": "acc-prev",
    "confirmation_token": "acc-secret",
    "expires_at": "2026-11-15T10:20:07Z",
    "bundle": "intern",
    "items": [
        {"system": "VPN", "reason": "Доступ в корпоративную сеть", "approver": "IT-отдел"},
        {"system": "Jira", "reason": "Работа с задачами команды", "approver": "Наставник"},
    ],
}
CREATED = {
    "requests": [
        {"request_id": "r1", "system": "VPN", "status": "pending", "external_id": "ACC-301"},
        {"request_id": "r2", "system": "Jira", "status": "pending", "external_id": "ACC-302"},
    ],
    "next_step": "Заявки ушли наставнику и в IT-отдел",
}


@pytest.fixture
def intern_with_access() -> User:
    return User(id="u-2", role="intern", token="intern-token", scopes=SCOPES)


class FakeCore:
    def __init__(self) -> None:
        self.requests: list[tuple[str, dict]] = []

    def __call__(self, request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content) if request.content else {}
        self.requests.append((request.url.path, body))
        if request.url.path == "/api/v1/requests/access/preview":
            if body.get("bundle") != "intern":
                return httpx.Response(404, json={"error": {"message": "Пакет не найден"}})
            return httpx.Response(200, json=PREVIEW)
        if request.url.path == "/api/v1/requests/access":
            return httpx.Response(200, json=CREATED)
        return httpx.Response(404)

    def paths(self) -> list[str]:
        return [path for path, _ in self.requests]


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


async def test_preview_access_bundle_defaults_to_intern(intern_with_access: User):
    core = FakeCore()
    result = await registry_for(core).call("preview_access_bundle", {}, intern_with_access)

    assert result.ok
    assert core.requests == [("/api/v1/requests/access/preview", {"bundle": "intern"})]


@pytest.mark.parametrize(
    ("bundle", "code"), [("../admin", "VALIDATION_ERROR"), ("ceo", "NOT_FOUND")]
)
async def test_preview_access_bundle_errors(intern_with_access: User, bundle: str, code: str):
    result = await registry_for(FakeCore()).call(
        "preview_access_bundle", {"bundle": bundle}, intern_with_access
    )
    assert result.error_code == code


async def test_create_access_requests_needs_confirmation(intern_with_access: User):
    core = FakeCore()
    result = await registry_for(core).call("create_access_requests", {}, intern_with_access)

    assert result.error_code == "CONFIRMATION_REQUIRED"
    assert core.requests == []


async def test_access_tools_need_scopes():
    registry = registry_for(FakeCore())
    reader = User(id="u", role="intern", token="t", scopes=["kb:read", "access:self"])

    names = [tool.name for tool in registry.available(reader)]
    assert "preview_access_bundle" in names
    assert "create_access_requests" not in names


ACCESS_TOOLS = [
    ToolSpec(name, "", {"type": "object"})
    for name in ("search_knowledge", "preview_access_bundle", "create_access_requests")
]


async def ask(question: str) -> LLMResponse:
    messages = [{"role": "user", "content": question}]
    return await MockProvider().complete(messages, ACCESS_TOOLS)


async def test_mock_separates_access_bundle_from_how_to_question():
    response = await ask("Оформи мне все доступы")
    assert response.tool_calls[0].name == "preview_access_bundle"
    assert response.tool_calls[0].arguments == {"bundle": "intern"}

    response = await ask("Как получить доступ к Jira?")
    assert response.tool_calls[0].name == "search_knowledge"


async def mock_text(name: str, payload: dict) -> str:
    messages = [
        {"role": "user", "content": "q"},
        {"role": "assistant", "tool_calls": [{"id": "c1", "function": {"name": name}}]},
        {"role": "tool", "tool_call_id": "c1", "content": json.dumps(payload)},
    ]
    return (await MockProvider().complete(messages, ACCESS_TOOLS)).text


async def test_mock_access_texts():
    preview = await mock_text("preview_access_bundle", {"ok": True, "result": PREVIEW})
    assert "**VPN** — Доступ в корпоративную сеть (согласует IT-отдел)" in preview
    assert "подтверждаю" in preview

    empty = await mock_text("preview_access_bundle", {"ok": True, "result": {"items": []}})
    assert "нет доступов" in empty

    created = await mock_text("create_access_requests", {"ok": True, "result": CREATED})
    assert "VPN — на согласовании (ACC-301)" in created
    assert "IT-отдел" in created


async def test_access_demo_scenario(intern_with_access: User):
    core = FakeCore()
    agent = Agent(MockProvider(), registry_for(core))

    first = await collect(agent, intern_with_access, "Запроси мне доступы для стажёра")
    assert tool_names(first) == ["preview_access_bundle"]
    assert "Подготовил пакет доступов" in text_of(first)

    second = await collect(agent, intern_with_access, "Да, отправляй")
    assert tool_names(second) == ["create_access_requests"]
    assert "ACC-302" in text_of(second)
    assert core.requests[-1] == (
        "/api/v1/requests/access",
        {
            "preview_id": "acc-prev",
            "confirmation_token": "acc-secret",
            "idempotency_key": "acc-prev",
        },
    )


async def test_access_flow_over_mcp():
    core = FakeCore()
    server = build_server(registry_for(core))
    app = build_app(server)

    def factory(headers: dict[str, str], timeout: float) -> httpx2.AsyncClient:
        return httpx2.AsyncClient(
            transport=httpx2.ASGITransport(app=app), headers=headers, timeout=timeout
        )

    user = User(id="u-2", role="intern", token=make_token(scopes=SCOPES), scopes=SCOPES)
    registry = MCPToolRegistry(
        "http://mcp.test/mcp", fallback=registry_for(FakeCore()), http_client_factory=factory
    )
    agent = Agent(MockProvider(), registry)
    async with server.session_manager.run():
        await collect(agent, user, "Нужны доступы")
        done = await collect(agent, user, "подтверждаю")

    assert "ACC-301" in text_of(done)
    assert core.paths() == ["/api/v1/requests/access/preview", "/api/v1/requests/access"]
