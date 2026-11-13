import json
from typing import Any

import httpx

from app.providers.base import LLMError, LLMResponse, ToolCall, ToolSpec


def _parse_arguments(raw: Any) -> dict[str, Any]:
    if isinstance(raw, dict):
        return raw
    if not raw:
        return {}
    try:
        parsed = json.loads(raw)
    except (TypeError, ValueError):
        return {}
    return parsed if isinstance(parsed, dict) else {}


class OpenAICompatProvider:
    name = "openai"

    def __init__(
        self,
        base_url: str,
        api_key: str,
        model: str,
        timeout: float = 30.0,
        temperature: float = 0.2,
        max_tokens: int = 800,
        client: httpx.AsyncClient | None = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.model = model
        self.timeout = timeout
        self.temperature = temperature
        self.max_tokens = max_tokens
        self._client = client

    def _payload(self, messages: list[dict[str, Any]], tools: list[ToolSpec]) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "model": self.model,
            "messages": messages,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
        }
        if tools:
            payload["tools"] = [tool.to_openai() for tool in tools]
            payload["tool_choice"] = "auto"
        return payload

    async def _post(self, payload: dict[str, Any]) -> httpx.Response:
        headers = {"Authorization": f"Bearer {self.api_key}"}
        url = f"{self.base_url}/chat/completions"
        if self._client is not None:
            return await self._client.post(url, json=payload, headers=headers)
        async with httpx.AsyncClient(timeout=self.timeout) as client:
            return await client.post(url, json=payload, headers=headers)

    async def complete(self, messages: list[dict[str, Any]], tools: list[ToolSpec]) -> LLMResponse:
        if not self.api_key:
            raise LLMError("LLM_API_KEY не задан", retryable=False)
        try:
            response = await self._post(self._payload(messages, tools))
        except httpx.TimeoutException as exc:
            raise LLMError("LLM не ответила вовремя") from exc
        except httpx.HTTPError as exc:
            raise LLMError("LLM недоступна") from exc

        if response.status_code >= 400:
            retryable = response.status_code == 429 or response.status_code >= 500
            raise LLMError(f"LLM вернула ошибку {response.status_code}", retryable=retryable)

        try:
            data = response.json()
            message = data["choices"][0]["message"]
        except (ValueError, KeyError, IndexError, TypeError) as exc:
            raise LLMError("LLM вернула некорректный ответ") from exc

        tool_calls = [
            ToolCall(
                id=call.get("id", ""),
                name=call.get("function", {}).get("name", ""),
                arguments=_parse_arguments(call.get("function", {}).get("arguments")),
            )
            for call in message.get("tool_calls") or []
        ]
        usage = data.get("usage") or {}
        return LLMResponse(
            text=message.get("content") or "",
            tool_calls=tool_calls,
            usage={
                "input_tokens": int(usage.get("prompt_tokens", 0)),
                "output_tokens": int(usage.get("completion_tokens", 0)),
            },
            model=data.get("model", self.model),
        )
