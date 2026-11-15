from collections.abc import Mapping
from typing import Any

import jwt
from mcp.server.mcpserver import Context, MCPServer
from mcp.server.transport_security import TransportSecuritySettings
from mcp.types import ToolAnnotations
from starlette.applications import Starlette
from starlette.requests import Request
from starlette.responses import JSONResponse

from app.agent.tools import DEFAULT_TOOLS, CoreClient, ToolRegistry, ToolResult
from app.auth import decode_token
from app.config import Settings, get_settings

READ_ONLY = ToolAnnotations(
    read_only_hint=True,
    destructive_hint=False,
    idempotent_hint=True,
    open_world_hint=False,
)
PREVIEW = ToolAnnotations(
    read_only_hint=True,
    destructive_hint=False,
    idempotent_hint=False,
    open_world_hint=False,
)
WRITE = ToolAnnotations(
    read_only_hint=False,
    destructive_hint=False,
    idempotent_hint=True,
    open_world_hint=True,
)
CREATE_DESCRIPTION = (
    "Отправить заявку на отсутствие по подтверждённому черновику. "
    "Нужны preview_id и confirmation_token из preview_absence_request и idempotency_key. "
    "Вызывать только после явного согласия пользователя."
)
CREATE_ACCESS_DESCRIPTION = (
    "Отправить заявки на доступы по подтверждённому черновику пакета. "
    "Нужны preview_id и confirmation_token из preview_access_bundle и idempotency_key. "
    "Вызывать только после явного согласия пользователя."
)
INSTRUCTIONS = (
    "Инструменты AI-buddy для стажёров Авито. Каждый вызов выполняется с JWT пользователя "
    "из заголовка Authorization и строго в его правах."
)
UNAUTHORIZED = {
    "ok": False,
    "error": {"code": "UNAUTHORIZED", "message": "Нет или недействителен токен пользователя"},
    "citations": [],
}


def tool_payload(result: ToolResult) -> dict[str, Any]:
    return {**result.for_model(), "citations": result.citations}


def bearer_token(headers: Mapping[str, str] | None) -> str:
    if not headers:
        return ""
    header = headers.get("authorization") or headers.get("Authorization") or ""
    scheme, _, token = header.partition(" ")
    return token.strip() if scheme.lower() == "bearer" else ""


async def handle(
    registry: ToolRegistry,
    settings: Settings,
    name: str,
    args: dict[str, Any],
    headers: Mapping[str, str] | None,
) -> dict[str, Any]:
    token = bearer_token(headers)
    if not token:
        return UNAUTHORIZED
    try:
        user = decode_token(token, settings)
    except jwt.PyJWTError:
        return UNAUTHORIZED
    return tool_payload(await registry.call(name, args, user))


def build_server(
    registry: ToolRegistry | None = None, settings: Settings | None = None
) -> MCPServer:
    settings = settings or get_settings()
    registry = registry or ToolRegistry(CoreClient(settings.core_url, settings.tool_timeout))
    descriptions = {tool.name: tool.spec.description for tool in DEFAULT_TOOLS}
    server = MCPServer(name="ai-buddy", instructions=INSTRUCTIONS, version="0.1.0")

    @server.tool(
        name="get_my_profile",
        description=descriptions["get_my_profile"],
        annotations=READ_ONLY,
    )
    async def get_my_profile(ctx: Context) -> dict[str, Any]:
        return await handle(registry, settings, "get_my_profile", {}, ctx.headers)

    @server.tool(
        name="get_onboarding_plan",
        description=descriptions["get_onboarding_plan"],
        annotations=READ_ONLY,
    )
    async def get_onboarding_plan(ctx: Context) -> dict[str, Any]:
        return await handle(registry, settings, "get_onboarding_plan", {}, ctx.headers)

    @server.tool(
        name="search_knowledge",
        description=descriptions["search_knowledge"],
        annotations=READ_ONLY,
    )
    async def search_knowledge(query: str, ctx: Context) -> dict[str, Any]:
        return await handle(registry, settings, "search_knowledge", {"query": query}, ctx.headers)

    @server.tool(
        name="preview_absence_request",
        description=descriptions["preview_absence_request"],
        annotations=PREVIEW,
    )
    async def preview_absence_request(
        date_from: str,
        ctx: Context,
        date_to: str = "",
        absence_type: str = "day_off",
        comment: str = "",
    ) -> dict[str, Any]:
        args = {
            "date_from": date_from,
            "date_to": date_to,
            "absence_type": absence_type,
            "comment": comment,
        }
        return await handle(registry, settings, "preview_absence_request", args, ctx.headers)

    @server.tool(
        name="create_absence_request",
        description=CREATE_DESCRIPTION,
        annotations=WRITE,
    )
    async def create_absence_request(
        preview_id: str, confirmation_token: str, idempotency_key: str, ctx: Context
    ) -> dict[str, Any]:
        args = {
            "preview_id": preview_id,
            "confirmation_token": confirmation_token,
            "idempotency_key": idempotency_key,
        }
        return await handle(registry, settings, "create_absence_request", args, ctx.headers)

    @server.tool(
        name="preview_access_bundle",
        description=descriptions["preview_access_bundle"],
        annotations=PREVIEW,
    )
    async def preview_access_bundle(ctx: Context, bundle: str = "intern") -> dict[str, Any]:
        args = {"bundle": bundle}
        return await handle(registry, settings, "preview_access_bundle", args, ctx.headers)

    @server.tool(
        name="create_access_requests",
        description=CREATE_ACCESS_DESCRIPTION,
        annotations=WRITE,
    )
    async def create_access_requests(
        preview_id: str, confirmation_token: str, idempotency_key: str, ctx: Context
    ) -> dict[str, Any]:
        args = {
            "preview_id": preview_id,
            "confirmation_token": confirmation_token,
            "idempotency_key": idempotency_key,
        }
        return await handle(registry, settings, "create_access_requests", args, ctx.headers)

    @server.tool(
        name="get_my_requests",
        description=descriptions["get_my_requests"],
        annotations=READ_ONLY,
    )
    async def get_my_requests(ctx: Context) -> dict[str, Any]:
        return await handle(registry, settings, "get_my_requests", {}, ctx.headers)

    @server.custom_route("/healthz", methods=["GET"])
    async def healthz(request: Request) -> JSONResponse:
        return JSONResponse({"status": "ok"})

    return server


def build_app(server: MCPServer | None = None) -> Starlette:
    server = server or build_server()
    return server.streamable_http_app(
        stateless_http=True,
        json_response=True,
        transport_security=TransportSecuritySettings(enable_dns_rebinding_protection=False),
    )
