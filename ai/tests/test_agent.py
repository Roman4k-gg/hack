import json

import httpx

from app.agent.loop import LIMIT_TEXT, LLM_ERROR_TEXT, Agent, chunk_text
from app.agent.tools import CoreClient, Tool, ToolRegistry, ToolResult
from app.auth import User
from app.providers.base import LLMError, LLMResponse, ToolCall, ToolSpec


class ScriptedProvider:
    name = "scripted"

    def __init__(self, responses):
        self.responses = list(responses)
        self.calls = []

    async def complete(self, messages, tools):
        self.calls.append({"messages": [dict(m) for m in messages], "tools": tools})
        item = self.responses.pop(0) if self.responses else self.responses_default()
        if isinstance(item, Exception):
            raise item
        return item

    def responses_default(self):
        return LLMResponse(tool_calls=[ToolCall(id="", name="get_onboarding_plan")])


def plan_registry(status: int = 200) -> ToolRegistry:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(status, json={"next_step": {"title": "Получить доступ к Jira"}})

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return ToolRegistry(CoreClient("http://core.test", client=client))


async def collect(agent: Agent, user: User, message: str = "что дальше?") -> list[dict]:
    return [event async for event in agent.run(user, message)]


def text_of(events: list[dict]) -> str:
    return "".join(event["text"] for event in events if event["type"] == "delta")


def test_chunk_text_keeps_all_text():
    text = "Твой следующий шаг — получить доступ к Jira."
    chunks = chunk_text(text, words_per_chunk=2)
    assert "".join(chunks) == text
    assert len(chunks) == 4
    assert chunk_text("") == []


async def test_plain_answer_streams_text_and_done(intern: User):
    provider = ScriptedProvider([LLMResponse(text="Привет! Я AI-buddy.")])
    events = await collect(Agent(provider, plan_registry()), intern)

    assert text_of(events) == "Привет! Я AI-buddy."
    assert events[-1] == {"type": "done"}
    system, user_message = provider.calls[0]["messages"]
    assert system["role"] == "system"
    assert "AI-buddy" in system["content"]
    assert user_message == {"role": "user", "content": "что дальше?"}
    assert [tool.name for tool in provider.calls[0]["tools"]] == [
        "get_my_profile",
        "get_onboarding_plan",
    ]


async def test_tool_call_then_answer(intern: User):
    provider = ScriptedProvider(
        [
            LLMResponse(tool_calls=[ToolCall(id="call_a", name="get_onboarding_plan")]),
            LLMResponse(text="Следующий шаг — получить доступ к Jira."),
        ]
    )
    events = await collect(Agent(provider, plan_registry()), intern)

    tool_events = [event for event in events if event["type"] == "tool_call"]
    assert [event["toolCall"]["status"] for event in tool_events] == ["running", "success"]
    assert {event["toolCall"]["id"] for event in tool_events} == {"call_a"}
    assert text_of(events) == "Следующий шаг — получить доступ к Jira."
    assert events[-1] == {"type": "done"}

    second_messages = provider.calls[1]["messages"]
    assert second_messages[2]["tool_calls"][0]["id"] == "call_a"
    tool_message = second_messages[3]
    assert tool_message["role"] == "tool"
    assert tool_message["tool_call_id"] == "call_a"
    assert json.loads(tool_message["content"])["result"]["next_step"]["title"] == (
        "Получить доступ к Jira"
    )


async def test_tool_error_is_reported_and_loop_continues(intern: User):
    provider = ScriptedProvider(
        [
            LLMResponse(tool_calls=[ToolCall(id="", name="get_onboarding_plan")]),
            LLMResponse(text="Не получилось открыть карту."),
        ]
    )
    events = await collect(Agent(provider, plan_registry(status=403)), intern)

    statuses = [event["toolCall"]["status"] for event in events if event["type"] == "tool_call"]
    assert statuses == ["running", "error"]
    assert events[0]["toolCall"]["id"] == "call-1"
    tool_message = provider.calls[1]["messages"][3]
    assert json.loads(tool_message["content"])["error"]["code"] == "FORBIDDEN"
    assert events[-1] == {"type": "done"}


async def test_iteration_limit(intern: User):
    provider = ScriptedProvider([])
    events = await collect(Agent(provider, plan_registry(), max_iterations=2), intern)

    assert len(provider.calls) == 2
    assert text_of(events) == LIMIT_TEXT
    assert events[-1] == {"type": "done"}


async def test_llm_error_yields_error_event(intern: User):
    provider = ScriptedProvider([LLMError("down")])
    events = await collect(Agent(provider, plan_registry()), intern)
    assert events == [{"type": "error", "message": LLM_ERROR_TEXT}]


async def test_citations_are_sent_before_done(intern: User):
    async def search(user, args, core):
        return ToolResult(
            ok=True,
            data={"title": "Jira"},
            citations=[{"id": "jira", "title": "Как получить доступ к Jira", "url": "u"}],
        )

    tool = Tool(spec=ToolSpec("search_knowledge", "поиск", {"type": "object"}), handler=search)
    registry = ToolRegistry(CoreClient("http://core.test"), tools=[tool])
    provider = ScriptedProvider(
        [
            LLMResponse(tool_calls=[ToolCall(id="c1", name="search_knowledge")]),
            LLMResponse(text="Вот инструкция."),
        ]
    )
    events = await collect(Agent(provider, registry, stream_delay=0.001), intern)

    assert events[-2] == {
        "type": "citation",
        "citation": {"id": "jira", "title": "Как получить доступ к Jira", "url": "u"},
    }
    assert events[-1] == {"type": "done"}
