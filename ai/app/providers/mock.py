from typing import Any

from app.providers.base import LLMResponse, ToolSpec

FALLBACK_TEXT = (
    "Я AI-buddy и сейчас работаю в демо-режиме без внешней модели. "
    "Спроси, что тебе делать дальше, или как получить доступ к Jira."
)


def _last_user_text(messages: list[dict[str, Any]]) -> str:
    for message in reversed(messages):
        if message.get("role") == "user":
            return str(message.get("content") or "")
    return ""


class MockProvider:
    name = "mock"

    async def complete(self, messages: list[dict[str, Any]], tools: list[ToolSpec]) -> LLMResponse:
        question = _last_user_text(messages).strip()
        text = FALLBACK_TEXT if not question else f"Ты спросил: «{question}». {FALLBACK_TEXT}"
        return LLMResponse(text=text, model="mock")
