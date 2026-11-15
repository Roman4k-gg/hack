import json
import logging
import sys

import httpx
import pytest
from fastapi.testclient import TestClient

from app.agent.loop import Agent
from app.agent.run_log import AgentRun
from app.agent.tools import CoreClient, ToolRegistry
from app.api.chat import get_agent, trace_id_from
from app.auth import User
from app.main import app
from app.observability import JsonFormatter, log_event, setup_logging
from app.providers.base import LLMError, LLMResponse, ToolCall
from tests.conftest import make_token


class ScriptedProvider:
    name = "scripted"

    def __init__(self, responses):
        self.responses = list(responses)

    async def complete(self, messages, tools):
        item = self.responses.pop(0)
        if isinstance(item, Exception):
            raise item
        return item


def plan_registry(status: int = 200) -> ToolRegistry:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(status, json={"next_step": {"title": "Jira"}})

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return ToolRegistry(CoreClient("http://core.test", client=client))


def agent_runs(caplog: pytest.LogCaptureFixture) -> list[dict]:
    return [
        record.fields
        for record in caplog.records
        if getattr(record, "fields", {}).get("event") == "agent_run"
    ]


async def collect(agent: Agent, user: User, message: str = "что дальше?", **kwargs) -> list:
    return [event async for event in agent.run(user, message, **kwargs)]


async def test_agent_run_is_logged_with_tokens_and_tools(intern: User, caplog):
    caplog.set_level(logging.INFO)
    provider = ScriptedProvider(
        [
            LLMResponse(
                tool_calls=[ToolCall(id="c1", name="get_onboarding_plan")],
                usage={"input_tokens": 100, "output_tokens": 10},
                model="gpt-test",
            ),
            LLMResponse(text="Ответ", usage={"input_tokens": 150, "output_tokens": 20}),
        ]
    )
    await collect(Agent(provider, plan_registry()), intern, trace_id="trace-1")

    [run] = agent_runs(caplog)
    assert run["trace_id"] == "trace-1"
    assert run["user_id"] == intern.id
    assert run["status"] == "ok"
    assert run["model"] == "gpt-test"
    assert run["prompt_version"] == "v1"
    assert run["tool_iterations"] == 2
    assert (run["input_tokens"], run["output_tokens"]) == (250, 30)
    assert run["message_length"] == len("что дальше?")
    assert run["tool_calls"][0]["name"] == "get_onboarding_plan"
    assert run["tool_calls"][0]["status"] == "success"
    assert "что дальше" not in json.dumps(run, ensure_ascii=False)


@pytest.mark.parametrize(
    ("responses", "status", "level"),
    [
        ([LLMError("down", retryable=False)], "llm_error", logging.WARNING),
        ([LLMError("503"), LLMResponse(text="ok")], "ok", logging.INFO),
    ],
)
async def test_agent_run_status_and_retries(intern: User, caplog, responses, status, level):
    caplog.set_level(logging.INFO)
    await collect(Agent(ScriptedProvider(responses), plan_registry()), intern)

    [run] = agent_runs(caplog)
    assert run["status"] == status
    assert run["llm_retries"] == len(responses) - 1
    record = next(r for r in caplog.records if getattr(r, "fields", {}).get("event"))
    assert record.levelno == level


async def test_agent_run_logs_limit_and_tool_errors(intern: User, caplog):
    caplog.set_level(logging.INFO)
    calls = [LLMResponse(tool_calls=[ToolCall(id="", name="get_onboarding_plan")])] * 2
    agent = Agent(ScriptedProvider(calls), plan_registry(status=403), max_iterations=2)
    await collect(agent, intern)

    [run] = agent_runs(caplog)
    assert run["status"] == "limit"
    assert [tool["error_code"] for tool in run["tool_calls"]] == ["FORBIDDEN", "FORBIDDEN"]


async def test_agent_run_logs_crash_and_cancel(intern: User, caplog):
    caplog.set_level(logging.INFO)

    class Crashing:
        name = "crash"

        async def complete(self, messages, tools):
            raise RuntimeError("boom")

    with pytest.raises(RuntimeError):
        await collect(Agent(Crashing(), plan_registry()), intern)

    stream = Agent(ScriptedProvider([LLMResponse(text="один два три четыре")]), plan_registry())
    events = stream.run(intern, "hi")
    await anext(events)
    await events.aclose()

    statuses = [run["status"] for run in agent_runs(caplog)]
    assert statuses == ["error", "cancelled"]
    assert agent_runs(caplog)[0]["error"] == "RuntimeError"


def test_agent_run_latency_uses_clock():
    ticks = iter([1.0, 1.5, 2.25])
    run = AgentRun("t", "u", "intern", "v1", 3, clock=lambda: next(ticks))
    run.add_tool("get_my_profile", True, "", started=1.0)
    assert run.tools[0]["latency_ms"] == 500
    assert run.as_dict()["latency_ms"] == 1250


def test_json_formatter_and_setup(capsys):
    record = logging.LogRecord("app.x", logging.ERROR, "", 0, "agent_run", None, None)
    record.fields = {"event": "agent_run", "trace_id": "t"}
    try:
        raise ValueError("bad")
    except ValueError:
        record.exc_info = sys.exc_info()
    payload = json.loads(JsonFormatter().format(record))
    assert payload["service"] == "ai"
    assert payload["level"] == "error"
    assert payload["trace_id"] == "t"
    assert "ValueError" in payload["error"]

    root = logging.getLogger()
    saved = (root.handlers[:], root.level)
    try:
        setup_logging("debug", json_logs=True)
        log_event(logging.getLogger("app.test"), "hello", user_id="u")
        assert json.loads(capsys.readouterr().err.strip())["user_id"] == "u"
        setup_logging("info", json_logs=False)
        assert not isinstance(root.handlers[0].formatter, JsonFormatter)
    finally:
        root.handlers, root.level = saved


def test_trace_id_from_headers():
    assert trace_id_from("abc-1", "other") == "abc-1"
    assert trace_id_from(None, "req.2") == "req.2"
    assert len(trace_id_from("bad header\n", None)) == 32


def test_chat_returns_trace_id_header():
    agent = Agent(ScriptedProvider([LLMResponse(text="Привет")]), plan_registry())
    app.dependency_overrides[get_agent] = lambda: agent
    try:
        response = TestClient(app).post(
            "/api/v1/chat/stream",
            json={"message": "привет"},
            headers={"Authorization": f"Bearer {make_token()}", "X-Request-ID": "req-42"},
        )
    finally:
        app.dependency_overrides.clear()
    assert response.headers["X-Trace-Id"] == "req-42"
