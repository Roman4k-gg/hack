import json

from app import sse


def parse(raw: str) -> dict:
    lines = raw.split("\n")
    assert lines[0] == "event: message"
    assert lines[1].startswith("data: ")
    assert raw.endswith("\n\n")
    return json.loads(lines[1][len("data: ") :])


def test_format_event_keeps_cyrillic():
    raw = sse.format_event(sse.delta("Привет"))
    assert "Привет" in raw
    assert parse(raw) == {"type": "delta", "text": "Привет"}


def test_tool_call_event():
    assert sse.tool_call("call-1", "get_onboarding_plan", "running") == {
        "type": "tool_call",
        "toolCall": {"id": "call-1", "name": "get_onboarding_plan", "status": "running"},
    }


def test_citation_with_and_without_url():
    assert sse.citation("jira", "Jira", "https://demo.local/jira")["citation"]["url"]
    assert "url" not in sse.citation("jira", "Jira")["citation"]


def test_done_and_error():
    assert sse.done() == {"type": "done"}
    assert sse.error("упало") == {"type": "error", "message": "упало"}
