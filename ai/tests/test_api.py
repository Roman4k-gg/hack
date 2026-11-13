import json

import httpx
import pytest
from fastapi.testclient import TestClient

from app.agent.loop import Agent
from app.agent.tools import CoreClient, ToolRegistry
from app.api.chat import INTERNAL_ERROR_TEXT, get_agent
from app.main import app
from app.providers import MockProvider
from tests.conftest import make_token

def parse_stream(body: str) -> list[dict]:
    events = []
    for block in body.strip().split("\n\n"):
        lines = block.split("\n")
        assert lines[0] == "event: message"
        events.append(json.loads(lines[1][len("data: ") :]))
    return events

@pytest.fixture
def client():
    core = CoreClient(
        "http://core.test",
        client=httpx.AsyncClient(transport=httpx.MockTransport(lambda r: httpx.Response(200))),
    )
    app.dependency_overrides[get_agent] = lambda: Agent(MockProvider(), ToolRegistry(core))
    yield TestClient(app)
    app.dependency_overrides.clear()

def test_health(client: TestClient):
    assert client.get("/healthz").json() == {"status": "ok"}
    ready = client.get("/readyz").json()
    assert ready["status"] == "ok"
    assert ready["llm_mode"] == "mock"

def test_chat_streams_sse(client: TestClient):
    response = client.post(
        "/api/v1/chat/stream",
        json={"message": "Привет", "user_id": "u", "role": "intern"},
        headers={"Authorization": f"Bearer {make_token()}"},
    )

    assert response.status_code == 200
    assert response.headers["content-type"].startswith("text/event-stream")
    assert response.headers["x-accel-buffering"] == "no"
    events = parse_stream(response.text)
    assert events[-1] == {"type": "done"}
    assert "Привет" in "".join(e["text"] for e in events if e["type"] == "delta")

@pytest.mark.parametrize(
    "header",
    [None, "Basic abc", "Bearer not-a-jwt", f"Bearer {make_token(iss='someone-else')}"],
)
def test_chat_requires_valid_token(client: TestClient, header):
    headers = {"Authorization": header} if header else {}
    response = client.post("/api/v1/chat/stream", json={"message": "hi"}, headers=headers)
    assert response.status_code == 401

def test_chat_rejects_empty_message(client: TestClient):
    response = client.post(
        "/api/v1/chat/stream",
        json={"message": ""},
        headers={"Authorization": f"Bearer {make_token()}"},
    )
    assert response.status_code == 422

def test_chat_unexpected_error_becomes_error_event(client: TestClient):
    class Broken:
        async def run(self, user, message):
            raise RuntimeError("boom")
            yield

    app.dependency_overrides[get_agent] = lambda: Broken()
    response = client.post(
        "/api/v1/chat/stream",
        json={"message": "hi"},
        headers={"Authorization": f"Bearer {make_token()}"},
    )
    assert parse_stream(response.text) == [{"type": "error", "message": INTERNAL_ERROR_TEXT}]

def test_get_agent_builds_from_settings():
    get_agent.cache_clear()
    agent = get_agent()
    assert isinstance(agent.provider, MockProvider)
    assert agent.max_iterations == 5
    get_agent.cache_clear()