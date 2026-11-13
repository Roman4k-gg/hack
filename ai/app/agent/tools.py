from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any

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

    async def get(self, path: str, user: User) -> Any:
        try:
            response = await self._request("GET", path, user)
        except httpx.TimeoutException as exc:
            raise ToolError("TIMEOUT", "Сервис не ответил вовремя") from exc
        except httpx.HTTPError as exc:
            raise ToolError("UPSTREAM_ERROR", "Сервис недоступен") from exc

        if response.status_code in (401, 403):
            raise ToolError("FORBIDDEN", "Недостаточно прав для этого действия")
        if response.status_code == 404:
            raise ToolError("NOT_FOUND", "Данные не найдены")
        if response.status_code >= 400:
            raise ToolError("UPSTREAM_ERROR", f"Сервис вернул ошибку {response.status_code}")
        try:
            return response.json()
        except ValueError as exc:
            raise ToolError("UPSTREAM_ERROR", "Сервис вернул некорректный ответ") from exc


Handler = Callable[[User, dict[str, Any], CoreClient], Awaitable[ToolResult]]


@dataclass(frozen=True)
class Tool:
    spec: ToolSpec
    handler: Handler
    scope: str = ""

    @property
    def name(self) -> str:
        return self.spec.name

    def allowed_for(self, user: User) -> bool:
        return not self.scope or user.has_scope(self.scope)


async def get_my_profile(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    return ToolResult(ok=True, data=await core.get("/api/v1/me", user))


async def get_onboarding_plan(user: User, args: dict[str, Any], core: CoreClient) -> ToolResult:
    return ToolResult(ok=True, data=await core.get("/api/v1/onboarding/plan", user))


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
]


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
