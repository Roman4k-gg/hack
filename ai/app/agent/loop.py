import asyncio
import json
import re
import uuid
from collections.abc import AsyncIterator, Callable
from datetime import date
from typing import Any

from app import sse
from app.agent.history import ConversationStore
from app.agent.pending import HIDDEN_FIELDS, PendingStore
from app.agent.prompts import build_system_message
from app.agent.run_log import AgentRun
from app.agent.tools import Tool, ToolBackend, ToolResult
from app.auth import User
from app.guardrails.injection import INJECTION_NOTE, find_injection
from app.guardrails.output import sanitize_result
from app.guardrails.pii import mask_pii
from app.providers.base import LLMError, LLMProvider, LLMResponse, ToolCall, ToolSpec

LIMIT_TEXT = (
    "Не получилось собрать ответ за отведённое число шагов. "
    "Попробуй переформулировать вопрос или спроси наставника."
)
LLM_ERROR_TEXT = "AI-модель сейчас недоступна. Попробуй ещё раз чуть позже."
CONFIRMATION_REQUIRED_TEXT = (
    "Нет подтверждённого черновика. Сначала подготовь черновик и дождись, "
    "пока пользователь явно подтвердит его в следующем сообщении."
)
RETRYABLE_CODES = {"TIMEOUT", "UPSTREAM_ERROR"}
TAINTED_TEXT = (
    "Действие заблокировано: в данных этого ответа нашлась подозрительная инструкция. "
    "Действия выполняются только по прямой просьбе пользователя."
)


def chunk_text(text: str, words_per_chunk: int = 3) -> list[str]:
    words = re.findall(r"\S+\s*", text)
    return [
        "".join(words[index : index + words_per_chunk])
        for index in range(0, len(words), words_per_chunk)
    ]


def assistant_tool_message(text: str, calls: list[tuple[str, ToolCall]]) -> dict[str, Any]:
    return {
        "role": "assistant",
        "content": text or None,
        "tool_calls": [
            {
                "id": call_id,
                "type": "function",
                "function": {
                    "name": call.name,
                    "arguments": json.dumps(call.arguments, ensure_ascii=False),
                },
            }
            for call_id, call in calls
        ],
    }


class Agent:
    def __init__(
        self,
        provider: LLMProvider,
        registry: ToolBackend,
        max_iterations: int = 5,
        prompt_version: str = "v1",
        stream_delay: float = 0.0,
        pending: PendingStore | None = None,
        llm_retries: int = 1,
        llm_retry_delay: float = 0.0,
        today: Callable[[], date] = date.today,
        history: ConversationStore | None = None,
    ) -> None:
        self.provider = provider
        self.registry = registry
        self.max_iterations = max_iterations
        self.prompt_version = prompt_version
        self.stream_delay = stream_delay
        self.pending = pending or PendingStore()
        self.llm_retries = llm_retries
        self.llm_retry_delay = llm_retry_delay
        self.today = today
        self.history = history if history is not None else ConversationStore()

    async def _complete(
        self, messages: list[dict[str, Any]], specs: list[ToolSpec], run: AgentRun
    ) -> LLMResponse:
        attempt = 0
        while True:
            try:
                response = await self.provider.complete(messages, specs)
            except LLMError as exc:
                if not exc.retryable or attempt >= self.llm_retries:
                    raise
                attempt += 1
                run.llm_retries += 1
                await asyncio.sleep(self.llm_retry_delay * attempt)
                continue
            run.add_response(response)
            return response

    def _pending_context(self, user: User) -> list[str]:
        action = self.pending.get(user.id)
        if action is None:
            return []
        draft = json.dumps(action.draft, ensure_ascii=False)
        return [
            f"- Ожидает подтверждения пользователя: {draft}",
            f"- Инструмент для подтверждения: {action.tool}",
            "- Вызывай его, только если в этом сообщении пользователь явно согласился.",
        ]

    async def _call_tool(
        self,
        call: ToolCall,
        user: User,
        tools: dict[str, Tool],
        turn_id: str,
        tainted: bool = False,
    ) -> ToolResult:
        tool = tools.get(call.name)
        is_action = tool is not None and (tool.requires_confirmation or bool(tool.confirmed_by))
        if tainted and is_action:
            return ToolResult(
                ok=False, error_code="INJECTION_SUSPECTED", error_message=TAINTED_TEXT
            )
        if tool is not None and tool.requires_confirmation:
            action = self.pending.get(user.id)
            if action is None or action.tool != call.name or action.turn_id == turn_id:
                return ToolResult(
                    ok=False,
                    error_code="CONFIRMATION_REQUIRED",
                    error_message=CONFIRMATION_REQUIRED_TEXT,
                )
            result = await self.registry.call(call.name, action.confirm_args(), user)
            if result.ok or result.error_code not in RETRYABLE_CODES:
                self.pending.pop(user.id)
            return result

        result = await self.registry.call(call.name, call.arguments, user)
        if tool is not None and tool.confirmed_by and result.ok and isinstance(result.data, dict):
            self.pending.put(user.id, tool.confirmed_by, result.data, turn_id)
            draft = {key: value for key, value in result.data.items() if key not in HIDDEN_FIELDS}
            return ToolResult(
                ok=True,
                data={**draft, "awaiting_confirmation": True, "confirm_tool": tool.confirmed_by},
                citations=result.citations,
            )
        return result

    async def _stream_text(self, text: str) -> AsyncIterator[dict[str, Any]]:
        for chunk in chunk_text(text):
            yield sse.delta(chunk)
            if self.stream_delay:
                await asyncio.sleep(self.stream_delay)

    async def run(
        self, user: User, message: str, trace_id: str = ""
    ) -> AsyncIterator[dict[str, Any]]:
        run = AgentRun(
            trace_id=trace_id or uuid.uuid4().hex,
            user_id=user.id,
            role=user.role,
            prompt_version=self.prompt_version,
            message_length=len(message),
        )
        try:
            async for event in self._run(user, message, run):
                yield event
        except Exception as exc:
            run.finish("error", type(exc).__name__)
            raise
        finally:
            run.log()

    async def _run(self, user: User, message: str, run: AgentRun) -> AsyncIterator[dict[str, Any]]:
        tools = {tool.name: tool for tool in self.registry.available(user)}
        specs = [tool.spec for tool in tools.values()]
        turn_id = uuid.uuid4().hex
        run.input_flags = find_injection(message)
        extra = self._pending_context(user)
        if run.input_flags:
            extra.append(INJECTION_NOTE)
        tainted = False
        previous = self.history.messages(user.id)
        run.history_messages = len(previous)
        messages: list[dict[str, Any]] = [
            build_system_message(user, self.prompt_version, today=self.today(), extra=extra),
            *previous,
            {"role": "user", "content": message},
        ]
        citations: dict[str, dict[str, str]] = {}
        call_counter = 0

        for _ in range(self.max_iterations):
            try:
                response = await self._complete(messages, specs, run)
            except LLMError as exc:
                run.finish("llm_error", str(exc))
                yield sse.error(LLM_ERROR_TEXT)
                return

            if not response.tool_calls:
                answer = mask_pii(response.text)
                if not run.input_flags and not tainted:
                    self.history.add(user.id, mask_pii(message), answer)
                async for event in self._stream_text(answer):
                    yield event
                for item in citations.values():
                    yield sse.citation(item["id"], item["title"], item.get("url", ""))
                run.finish("ok")
                yield sse.done()
                return

            calls: list[tuple[str, ToolCall]] = []
            for call in response.tool_calls:
                call_counter += 1
                calls.append((call.id or f"call-{call_counter}", call))
            messages.append(assistant_tool_message(response.text, calls))

            for call_id, call in calls:
                yield sse.tool_call(call_id, call.name, "running")
                started = run.clock()
                result = await self._call_tool(call, user, tools, turn_id, tainted)
                result, flags = sanitize_result(result)
                if flags:
                    tainted = True
                    run.tool_flags.extend(flag for flag in flags if flag not in run.tool_flags)
                run.add_tool(call.name, result.ok, result.error_code, started)
                yield sse.tool_call(call_id, call.name, "success" if result.ok else "error")
                for item in result.citations:
                    citations[item["id"]] = item
                messages.append(
                    {
                        "role": "tool",
                        "tool_call_id": call_id,
                        "content": json.dumps(result.for_model(), ensure_ascii=False),
                    }
                )

        async for event in self._stream_text(LIMIT_TEXT):
            yield event
        run.finish("limit")
        yield sse.done()
