import logging
from collections.abc import AsyncIterator
from functools import lru_cache
from typing import Annotated

from fastapi import APIRouter, Depends
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from app import sse
from app.agent.loop import Agent
from app.agent.mcp_tools import MCPToolRegistry
from app.agent.tools import CoreClient, ToolBackend, ToolRegistry
from app.auth import User, current_user
from app.config import Settings, get_settings
from app.providers import build_provider

logger = logging.getLogger(__name__)
router = APIRouter()

INTERNAL_ERROR_TEXT = "Что-то пошло не так. Попробуй ещё раз."


class ChatRequest(BaseModel):
    message: str = Field(min_length=1, max_length=4000)
    user_id: str | None = None
    role: str | None = None


def build_tools(settings: Settings) -> ToolBackend:
    direct = ToolRegistry(CoreClient(settings.core_url, settings.tool_timeout))
    if settings.tools_mode == "mcp":
        return MCPToolRegistry(settings.mcp_url, fallback=direct, timeout=settings.tool_timeout * 2)
    return direct


@lru_cache
def get_agent() -> Agent:
    settings = get_settings()
    return Agent(
        provider=build_provider(settings),
        registry=build_tools(settings),
        max_iterations=settings.max_tool_iterations,
        prompt_version=settings.prompt_version,
        stream_delay=settings.mock_stream_delay if settings.llm_mode == "mock" else 0.0,
    )


async def stream_events(agent: Agent, user: User, message: str) -> AsyncIterator[str]:
    try:
        async for event in agent.run(user, message):
            yield sse.format_event(event)
    except Exception:
        logger.exception("chat stream failed")
        yield sse.format_event(sse.error(INTERNAL_ERROR_TEXT))


@router.post("/api/v1/chat/stream")
async def chat_stream(
    body: ChatRequest,
    user: Annotated[User, Depends(current_user)],
    agent: Annotated[Agent, Depends(get_agent)],
) -> StreamingResponse:
    return StreamingResponse(
        stream_events(agent, user, body.message.strip()),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )
