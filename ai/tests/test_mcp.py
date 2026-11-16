from contextlib import asynccontextmanager

import httpx
import httpx2
from fastapi.testclient import TestClient
from mcp.types import CallToolResult, TextContent

from app.agent.mcp_tools import MCPToolRegistry, default_http_client, parse_result
from app.agent.tools import CoreClient, ToolRegistry
from app.api.chat import build_tools
from app.auth import User
from app.config import Settings
from app.mcp import __main__ as mcp_main
from app.mcp.server import UNAUTHORIZED, bearer_token, build_app, build_server, handle
from tests.conftest import make_token

PLAN = {"next_step": {"title": "Получить доступ к Jira"}}
JIRA = {"document_id": "jira", "title": "Как получить доступ к Jira", "url": "u", "score": 1}


def core_registry(seen: list[str] | None = None) -> ToolRegistry:
    def handler(request: httpx.Request) -> httpx.Response:
        if seen is not None:
            seen.append(request.headers["Authorization"])
        if request.url.path == "/api/v1/onboarding/plan":
            return httpx.Response(200, json=PLAN)
        if request.url.path == "/api/v1/kb/search":
            return httpx.Response(200, json={"items": [JIRA]})
        return httpx.Response(403)

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return ToolRegistry(CoreClient("http://core.test", client=client))


def user_with_token(**claims) -> User:
    return User(
        id="u",
        role="intern",
        token=make_token(**claims),
        scopes=["onboarding:plan:read", "kb:read"],
    )


def test_bearer_token():
    assert bearer_token(None) == ""
    assert bearer_token({"authorization": "Bearer abc "}) == "abc"
    assert bearer_token({"Authorization": "Bearer xyz"}) == "xyz"
    assert bearer_token({"authorization": "Basic abc"}) == ""


async def test_handle_checks_jwt_and_forwards_it():
    seen: list[str] = []
    token = make_token()
    settings = Settings()

    ok = await handle(
        core_registry(seen),
        settings,
        "get_onboarding_plan",
        {},
        {"authorization": f"Bearer {token}"},
    )
    assert ok == {"ok": True, "result": PLAN, "citations": []}
    assert seen == [f"Bearer {token}"]

    assert await handle(core_registry(), settings, "get_onboarding_plan", {}, {}) == UNAUTHORIZED
    bad = {"authorization": "Bearer not-a-jwt"}
    assert await handle(core_registry(), settings, "get_onboarding_plan", {}, bad) == UNAUTHORIZED


async def test_handle_respects_scopes():
    token = make_token(scopes=["kb:read"])
    result = await handle(
        core_registry(),
        Settings(),
        "get_onboarding_plan",
        {},
        {"authorization": f"Bearer {token}"},
    )
    assert result["error"]["code"] == "FORBIDDEN"


async def test_server_lists_tools_with_annotations():
    from mcp.client.client import Client

    async with Client(build_server(core_registry())) as client:
        tools = {tool.name: tool for tool in (await client.list_tools()).tools}
        assert set(tools) == {
            "get_my_profile",
            "get_onboarding_plan",
            "search_knowledge",
            "preview_absence_request",
            "create_absence_request",
            "get_my_requests",
            "preview_access_bundle",
            "create_access_requests",
            "preview_handoff",
            "create_handoff",
        }
        writers = {name for name, tool in tools.items() if not tool.annotations.read_only_hint}
        assert writers == {"create_absence_request", "create_access_requests", "create_handoff"}
        assert "required" not in tools["preview_access_bundle"].input_schema
        assert tools["search_knowledge"].input_schema["required"] == ["query"]
        assert tools["preview_absence_request"].input_schema["required"] == ["date_from"]
        assert set(tools["create_absence_request"].input_schema["required"]) == {
            "preview_id",
            "confirmation_token",
            "idempotency_key",
        }

        result = await client.call_tool("get_my_profile", {})
        assert parse_result(result).error_code == "UNAUTHORIZED"


def test_healthz():
    with TestClient(build_app(build_server(core_registry()))) as client:
        assert client.get("/healthz").json() == {"status": "ok"}


@asynccontextmanager
async def mcp_over_http():
    seen: list[str] = []
    server = build_server(core_registry(seen))
    app = build_app(server)

    def factory(headers: dict[str, str], timeout: float) -> httpx2.AsyncClient:
        return httpx2.AsyncClient(
            transport=httpx2.ASGITransport(app=app), headers=headers, timeout=timeout
        )

    async with server.session_manager.run():
        registry = MCPToolRegistry(
            "http://mcp.test/mcp", fallback=core_registry(), http_client_factory=factory
        )
        yield registry, seen


async def test_mcp_round_trip_forwards_user_jwt():
    user = user_with_token()
    async with mcp_over_http() as (registry, seen):
        plan = await registry.call("get_onboarding_plan", {}, user)
        search = await registry.call("search_knowledge", {"query": "jira"}, user)

    assert plan.ok
    assert plan.data == PLAN
    assert seen[0] == f"Bearer {user.token}"
    assert search.ok
    assert search.citations == [{"id": "jira", "title": "Как получить доступ к Jira", "url": "u"}]


async def test_mcp_rejects_tool_outside_user_scopes():
    user = User(id="u", role="hr", token=make_token(scopes=["kb:read"]), scopes=["kb:read"])
    async with mcp_over_http() as (registry, seen):
        result = await registry.call("get_onboarding_plan", {}, user)

    assert result.error_code == "FORBIDDEN"
    assert seen == []
    assert [tool.name for tool in registry.available(user)] == [
        "get_my_profile",
        "search_knowledge",
    ]


async def test_mcp_unavailable_falls_back_to_direct_calls():
    def broken(headers: dict[str, str], timeout: float) -> httpx2.AsyncClient:
        def fail(request: httpx2.Request) -> httpx2.Response:
            raise httpx2.ConnectError("down", request=request)

        return httpx2.AsyncClient(transport=httpx2.MockTransport(fail))

    seen: list[str] = []
    registry = MCPToolRegistry(
        "http://mcp.test/mcp", fallback=core_registry(seen), http_client_factory=broken
    )
    user = user_with_token()

    result = await registry.call("get_onboarding_plan", {}, user)
    assert result.ok
    assert result.data == PLAN
    assert seen == [f"Bearer {user.token}"]


def text_result(text: str, is_error: bool = False) -> CallToolResult:
    return CallToolResult(content=[TextContent(type="text", text=text)], is_error=is_error)


def test_parse_result_variants():
    ok = parse_result(text_result('{"ok": true, "result": {"a": 1}, "citations": []}'))
    assert ok.ok
    assert ok.data == {"a": 1}

    error = parse_result(text_result('{"ok": false, "error": {"code": "NOT_FOUND"}}'))
    assert error.error_code == "NOT_FOUND"
    assert error.error_message == "Ошибка инструмента"

    assert parse_result(text_result("not json")).error_code == "UPSTREAM_ERROR"
    assert parse_result(text_result('{"ok": true}', is_error=True)).error_code == "UPSTREAM_ERROR"


def test_build_tools_by_mode():
    assert isinstance(build_tools(Settings(tools_mode="mcp")), MCPToolRegistry)
    assert isinstance(build_tools(Settings(tools_mode="direct")), ToolRegistry)


async def test_default_http_client_sets_headers():
    async with default_http_client({"Authorization": "Bearer t"}, 3.0) as client:
        assert client.headers["Authorization"] == "Bearer t"


def test_main_runs_uvicorn(monkeypatch):
    calls = {}
    monkeypatch.setattr(mcp_main.uvicorn, "run", lambda app, **kwargs: calls.update(kwargs))
    mcp_main.main()
    assert calls == {"host": "0.0.0.0", "port": 8001, "log_level": "info"}
