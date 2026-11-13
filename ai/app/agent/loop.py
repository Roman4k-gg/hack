import asyncio
import json
import re
from collections.abc import AsyncIterator
from typing import Any

from app import sse
from app.agent.prompts import build_system_message
from app.agent.tools import ToolRegistry
from app.auth import User
from app.providers.base import LLMError, LLMProvider, ToolCall

LIMIT_TEXT = (
    "Не получилось собрать ответ за отведённое число шагов. "
    "Попробуй переформулировать вопрос или спроси наставника."
)
LLM_ERROR_TEXT = "AI-модель сейчас недоступна. Попробуй ещё раз чуть позже."


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
        registry: ToolRegistry,
        max_iterations: int = 5,
        prompt_version: str = "v1",
        stream_delay: float = 0.0,
    ) -> None:
        self.provider = provider
        self.registry = registry
        self.max_iterations = max_iterations
        self.prompt_version = prompt_version
        self.stream_delay = stream_delay

    async def _stream_text(self, text: str) -> AsyncIterator[dict[str, Any]]:
        for chunk in chunk_text(text):
            yield sse.delta(chunk)
            if self.stream_delay:
                await asyncio.sleep(self.stream_delay)

    async def run(self, user: User, message: str) -> AsyncIterator[dict[str, Any]]:
        tools = self.registry.available(user)
        specs = [tool.spec for tool in tools]
        messages: list[dict[str, Any]] = [
            build_system_message(user, self.prompt_version),
            {"role": "user", "content": message},
        ]
        citations: dict[str, dict[str, str]] = {}
        call_counter = 0

        for _ in range(self.max_iterations):
            try:
                response = await self.provider.complete(messages, specs)
            except LLMError:
                yield sse.error(LLM_ERROR_TEXT)
                return

            if not response.tool_calls:
                async for event in self._stream_text(response.text):
                    yield event
                for item in citations.values():
                    yield sse.citation(item["id"], item["title"], item.get("url", ""))
                yield sse.done()
                return

            calls: list[tuple[str, ToolCall]] = []
            for call in response.tool_calls:
                call_counter += 1
                calls.append((call.id or f"call-{call_counter}", call))
            messages.append(assistant_tool_message(response.text, calls))

            for call_id, call in calls:
                yield sse.tool_call(call_id, call.name, "running")
                result = await self.registry.call(call.name, call.arguments, user)
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
        yield sse.done()
