import logging
import re
import uuid
from collections.abc import AsyncIterator
from functools import lru_cache
from typing import Annotated

from fastapi import APIRouter, Depends, Header
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from app import sse
from app.agent.history import ConversationStore
from app.agent.loop import Agent
from app.agent.mcp_tools import MCPToolRegistry
from app.agent.tools import CoreClient, ToolBackend, ToolRegistry
from app.auth import User, current_user
from app.config import Settings, get_settings
from app.guardrails.rate_limit import RateLimiter
from app.providers import build_provider

logger = logging.getLogger(__name__)
router = APIRouter()

INTERNAL_ERROR_TEXT = "Что-то пошло не так. Попробуй ещё раз."
TRACE_PATTERN = re.compile(r"^[A-Za-z0-9._-]{1,64}$")
RATE_LIMIT_TEXT = "Слишком много сообщений подряд. Подожди минуту и попробуй снова."


class ChatRequest(BaseModel):
    message: str = Field(min_length=1, max_length=4000)
    user_id: str | None = None
    role: str | None = None
    conversation_id: str | None = Field(default=None, max_length=64)


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
        llm_retries=settings.llm_retries,
        llm_retry_delay=settings.llm_retry_delay,
        history=ConversationStore(max_turns=settings.history_turns, ttl=settings.history_ttl),
    )


@lru_cache
def get_rate_limiter() -> RateLimiter:
    return RateLimiter(limit=get_settings().rate_limit_per_minute)


async def single_event(event: dict) -> AsyncIterator[str]:
    yield sse.format_event(event)


def trace_id_from(*candidates: str | None) -> str:
    for value in candidates:
        if value and TRACE_PATTERN.match(value):
            return value
    return uuid.uuid4().hex


async def stream_events(
    agent: Agent, user: User, message: str, trace_id: str = "", conversation_id: str = ""
) -> AsyncIterator[str]:
    try:
        async for event in agent.run(
            user, message, trace_id=trace_id, conversation_id=conversation_id
        ):
            yield sse.format_event(event)
    except Exception:
        logger.exception("chat stream failed", extra={"fields": {"trace_id": trace_id}})
        yield sse.format_event(sse.error(INTERNAL_ERROR_TEXT))


@router.post("/api/v1/chat/stream")
async def chat_stream(
    body: ChatRequest,
    user: Annotated[User, Depends(current_user)],
    agent: Annotated[Agent, Depends(get_agent)],
    limiter: Annotated[RateLimiter, Depends(get_rate_limiter)],
    x_request_id: Annotated[str | None, Header()] = None,
    x_trace_id: Annotated[str | None, Header()] = None,
) -> StreamingResponse:
    trace_id = trace_id_from(x_trace_id, x_request_id)
    if not limiter.allow(user.id):
        return StreamingResponse(
            single_event(sse.error(RATE_LIMIT_TEXT)),
            status_code=429,
            media_type="text/event-stream",
            headers={"Retry-After": "60", "X-Trace-Id": trace_id},
        )
    return StreamingResponse(
        stream_events(
            agent, user, body.message.strip(), trace_id, (body.conversation_id or "").strip()
        ),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
            "X-Trace-Id": trace_id,
        },
    )


@router.delete("/api/v1/chat/history", status_code=204)
async def clear_history(
    user: Annotated[User, Depends(current_user)],
    agent: Annotated[Agent, Depends(get_agent)],
) -> None:
    agent.history.clear(user.id)
    agent.pending.clear(user.id)
