import httpx
import pytest

from app.agent.tools import CoreClient, ToolRegistry
from app.auth import User


def registry_with(handler) -> ToolRegistry:
    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return ToolRegistry(CoreClient("http://core.test/", client=client))


async def test_get_onboarding_plan_forwards_user_token(intern: User):
    seen = {}

    def handler(request: httpx.Request) -> httpx.Response:
        seen["url"] = str(request.url)
        seen["auth"] = request.headers["Authorization"]
        return httpx.Response(200, json={"next_step": {"title": "Получить доступ к Jira"}})

    result = await registry_with(handler).call("get_onboarding_plan", {}, intern)

    assert result.ok
    assert result.data["next_step"]["title"] == "Получить доступ к Jira"
    assert seen == {
        "url": "http://core.test/api/v1/onboarding/plan",
        "auth": "Bearer intern-token",
    }
    assert result.for_model() == {"ok": True, "result": result.data}


async def test_get_my_profile(intern: User):
    result = await registry_with(lambda request: httpx.Response(200, json={"role": "intern"})).call(
        "get_my_profile", {}, intern
    )
    assert result.ok
    assert result.data == {"role": "intern"}


@pytest.mark.parametrize(
    ("status", "code"),
    [(401, "FORBIDDEN"), (403, "FORBIDDEN"), (404, "NOT_FOUND"), (500, "UPSTREAM_ERROR")],
)
async def test_core_errors_are_mapped(intern: User, status: int, code: str):
    result = await registry_with(lambda request: httpx.Response(status)).call(
        "get_onboarding_plan", {}, intern
    )
    assert not result.ok
    assert result.error_code == code
    assert result.for_model()["error"]["code"] == code


async def test_core_bad_json(intern: User):
    result = await registry_with(lambda request: httpx.Response(200, text="not json")).call(
        "get_onboarding_plan", {}, intern
    )
    assert result.error_code == "UPSTREAM_ERROR"


@pytest.mark.parametrize(
    ("exc", "code"),
    [(httpx.ReadTimeout, "TIMEOUT"), (httpx.ConnectError, "UPSTREAM_ERROR")],
)
async def test_core_network_errors(intern: User, exc, code: str):
    def handler(request: httpx.Request) -> httpx.Response:
        raise exc("boom", request=request)

    result = await registry_with(handler).call("get_onboarding_plan", {}, intern)
    assert result.error_code == code


async def test_unknown_tool_is_forbidden(intern: User):
    result = await registry_with(lambda request: httpx.Response(200)).call("drop_db", {}, intern)
    assert result.error_code == "FORBIDDEN"


async def test_tools_filtered_by_scope():
    registry = registry_with(lambda request: httpx.Response(200, json={}))
    no_plan = User(id="u", role="hr", token="t", scopes=["kb:read"])
    admin = User(id="a", role="admin", token="t", scopes=["*"])

    assert [tool.name for tool in registry.available(no_plan)] == ["get_my_profile"]
    assert {tool.name for tool in registry.available(admin)} == {
        "get_my_profile",
        "get_onboarding_plan",
    }
    result = await registry.call("get_onboarding_plan", {}, no_plan)
    assert result.error_code == "FORBIDDEN"


async def test_core_client_without_injected_client(intern: User, monkeypatch):
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"ok": True})

    original = httpx.AsyncClient

    def patched(*args, **kwargs):
        kwargs["transport"] = httpx.MockTransport(handler)
        return original(*args, **kwargs)

    monkeypatch.setattr(httpx, "AsyncClient", patched)
    assert await CoreClient("http://core.test").get("/api/v1/me", intern) == {"ok": True}
