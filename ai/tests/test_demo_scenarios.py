import httpx

from app.agent.loop import Agent
from app.agent.tools import CoreClient, ToolRegistry
from app.auth import User
from app.providers import MockProvider

PLAN = {
    "intern": {"full_name": "Дарья Соколова"},
    "progress": {"done": 4, "total": 10, "percent": 40},
    "next_step": {"title": "Получить доступ к Jira", "reason": "Обязательный пункт"},
    "stages": [],
}
JIRA = {
    "document_id": "55555555-5555-5555-5555-555555555501",
    "title": "Как получить доступ к Jira",
    "url": "https://demo.local/docs/jira-access",
    "chunk": "1. Напишите наставнику в чат.",
    "score": 1,
}


def fake_core(request: httpx.Request) -> httpx.Response:
    if request.url.path == "/api/v1/onboarding/plan":
        return httpx.Response(200, json=PLAN)
    if request.url.path == "/api/v1/kb/search":
        hit = "jira" in request.url.params["q"].lower()
        return httpx.Response(200, json={"items": [JIRA] if hit else []})
    return httpx.Response(404)


def demo_agent() -> Agent:
    client = httpx.AsyncClient(transport=httpx.MockTransport(fake_core))
    return Agent(MockProvider(), ToolRegistry(CoreClient("http://core.test", client=client)))


async def run(user: User, question: str) -> list[dict]:
    return [event async for event in demo_agent().run(user, question)]


def text_of(events: list[dict]) -> str:
    return "".join(event["text"] for event in events if event["type"] == "delta")


async def test_next_step_scenario(intern: User):
    events = await run(intern, "Что мне делать дальше?")

    assert events[0] == {
        "type": "tool_call",
        "toolCall": {"id": "call-1", "name": "get_onboarding_plan", "status": "running"},
    }
    assert events[1]["toolCall"]["status"] == "success"
    assert "**Следующий шаг:** Получить доступ к Jira" in text_of(events)
    assert events[-1] == {"type": "done"}


async def test_jira_question_scenario(intern: User):
    events = await run(intern, "Где взять доступ к Jira?")

    names = {event["toolCall"]["name"] for event in events if event["type"] == "tool_call"}
    assert names == {"search_knowledge"}
    assert "Нашёл в документе «Как получить доступ к Jira»" in text_of(events)
    assert {
        "type": "citation",
        "citation": {
            "id": JIRA["document_id"],
            "title": "Как получить доступ к Jira",
            "url": "https://demo.local/docs/jira-access",
        },
    } in events
    assert events[-1] == {"type": "done"}


async def test_unknown_question_suggests_mentor(intern: User):
    events = await run(intern, "Что такое экватор?")
    assert "наставник" in text_of(events)
    assert not [event for event in events if event["type"] == "citation"]
