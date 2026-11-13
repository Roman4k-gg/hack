import json

import httpx
import pytest

from app.config import Settings
from app.providers import (
    LLMError,
    MockProvider,
    OpenAICompatProvider,
    ToolSpec,
    build_provider,
)

TOOL = ToolSpec(
    name="get_onboarding_plan",
    description="Карта адаптации",
    parameters={"type": "object", "properties": {}},
)


def make_provider(handler, api_key: str = "sk-test") -> OpenAICompatProvider:
    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return OpenAICompatProvider(
        base_url="https://llm.test/v1/", api_key=api_key, model="test-model", client=client
    )


def test_build_provider_by_mode():
    assert isinstance(build_provider(Settings(llm_mode="mock")), MockProvider)
    assert isinstance(build_provider(Settings(llm_mode="real")), OpenAICompatProvider)


async def test_mock_provider_echoes_question():
    response = await MockProvider().complete([{"role": "user", "content": "Привет"}], [])
    assert "Привет" in response.text
    assert response.tool_calls == []


async def test_mock_provider_without_question():
    response = await MockProvider().complete([], [])
    assert response.text.startswith("Я AI-buddy")


async def test_openai_provider_parses_tool_calls():
    seen = {}

    def handler(request: httpx.Request) -> httpx.Response:
        seen["url"] = str(request.url)
        seen["auth"] = request.headers["Authorization"]
        seen["body"] = json.loads(request.content)
        return httpx.Response(
            200,
            json={
                "model": "test-model",
                "choices": [
                    {
                        "message": {
                            "content": None,
                            "tool_calls": [
                                {
                                    "id": "call_1",
                                    "type": "function",
                                    "function": {
                                        "name": "get_onboarding_plan",
                                        "arguments": "{}",
                                    },
                                },
                                {
                                    "id": "call_2",
                                    "type": "function",
                                    "function": {"name": "broken", "arguments": "not json"},
                                },
                            ],
                        }
                    }
                ],
                "usage": {"prompt_tokens": 10, "completion_tokens": 5},
            },
        )

    provider = make_provider(handler)
    response = await provider.complete([{"role": "user", "content": "что дальше?"}], [TOOL])

    assert seen["url"] == "https://llm.test/v1/chat/completions"
    assert seen["auth"] == "Bearer sk-test"
    assert seen["body"]["tools"][0]["function"]["name"] == "get_onboarding_plan"
    assert response.text == ""
    assert [call.name for call in response.tool_calls] == ["get_onboarding_plan", "broken"]
    assert response.tool_calls[1].arguments == {}
    assert response.usage == {"input_tokens": 10, "output_tokens": 5}


async def test_openai_provider_text_without_tools():
    def handler(request: httpx.Request) -> httpx.Response:
        assert "tools" not in json.loads(request.content)
        return httpx.Response(200, json={"choices": [{"message": {"content": "Привет!"}}]})

    response = await make_provider(handler).complete([{"role": "user", "content": "hi"}], [])
    assert response.text == "Привет!"
    assert response.tool_calls == []


async def test_openai_provider_requires_key():
    provider = make_provider(lambda request: httpx.Response(200), api_key="")
    with pytest.raises(LLMError) as exc:
        await provider.complete([], [])
    assert exc.value.retryable is False


@pytest.mark.parametrize(("status", "retryable"), [(429, True), (500, True), (401, False)])
async def test_openai_provider_http_errors(status: int, retryable: bool):
    provider = make_provider(lambda request: httpx.Response(status))
    with pytest.raises(LLMError) as exc:
        await provider.complete([], [])
    assert exc.value.retryable is retryable


async def test_openai_provider_bad_payload():
    provider = make_provider(lambda request: httpx.Response(200, json={"choices": []}))
    with pytest.raises(LLMError):
        await provider.complete([], [])


async def test_openai_provider_timeout():
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout", request=request)

    with pytest.raises(LLMError, match="вовремя"):
        await make_provider(handler).complete([], [])


async def test_openai_provider_network_error():
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("down", request=request)

    with pytest.raises(LLMError, match="недоступна"):
        await make_provider(handler).complete([], [])
