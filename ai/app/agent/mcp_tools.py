import json
import logging
from collections.abc import AsyncIterator, Callable
from contextlib import asynccontextmanager
from typing import Any

import httpx2
from mcp.client.client import Client
from mcp.client.streamable_http import streamable_http_client
from mcp.types import CallToolResult, TextContent

from app.agent.tools import Tool, ToolRegistry, ToolResult
from app.auth import User

logger = logging.getLogger(__name__)

HttpClientFactory = Callable[[dict[str, str], float], httpx2.AsyncClient]


def default_http_client(headers: dict[str, str], timeout: float) -> httpx2.AsyncClient:
    return httpx2.AsyncClient(headers=headers, timeout=timeout)


def parse_result(result: CallToolResult) -> ToolResult:
    payload: Any = result.structured_content
    if not isinstance(payload, dict):
        text = "".join(item.text for item in result.content if isinstance(item, TextContent))
        try:
            payload = json.loads(text)
        except ValueError:
            payload = None
    if result.is_error or not isinstance(payload, dict):
        return ToolResult(
            ok=False,
            error_code="UPSTREAM_ERROR",
            error_message="MCP-сервер вернул ошибку",
        )
    if payload.get("ok"):
        return ToolResult(
            ok=True,
            data=payload.get("result"),
            citations=list(payload.get("citations") or []),
        )
    error = payload.get("error") or {}
    return ToolResult(
        ok=False,
        error_code=str(error.get("code") or "UPSTREAM_ERROR"),
        error_message=str(error.get("message") or "Ошибка инструмента"),
    )


class MCPToolRegistry:
    def __init__(
        self,
        url: str,
        fallback: ToolRegistry,
        timeout: float = 10.0,
        http_client_factory: HttpClientFactory = default_http_client,
    ) -> None:
        self.url = url
        self.fallback = fallback
        self.timeout = timeout
        self.http_client_factory = http_client_factory

    def available(self, user: User) -> list[Tool]:
        return self.fallback.available(user)

    @asynccontextmanager
    async def _client(self, user: User) -> AsyncIterator[Client]:
        headers = {"Authorization": f"Bearer {user.token}"}
        async with self.http_client_factory(headers, self.timeout) as http:
            transport = streamable_http_client(self.url, http_client=http)
            async with Client(transport, read_timeout_seconds=self.timeout) as client:
                yield client

    async def call(self, name: str, args: dict[str, Any], user: User) -> ToolResult:
        if name not in {tool.name for tool in self.available(user)}:
            return ToolResult(
                ok=False,
                error_code="FORBIDDEN",
                error_message=f"Инструмент {name} недоступен",
            )
        try:
            async with self._client(user) as client:
                result = await client.call_tool(name, args)
        except Exception as exc:
            logger.warning("mcp call %s failed, using direct core call: %s", name, exc)
            return await self.fallback.call(name, args, user)
        return parse_result(result)
