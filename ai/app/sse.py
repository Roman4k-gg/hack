import json
from typing import Any


def format_event(data: dict[str, Any]) -> str:
    return f"event: message\ndata: {json.dumps(data, ensure_ascii=False)}\n\n"


def delta(text: str) -> dict[str, Any]:
    return {"type": "delta", "text": text}


def tool_call(call_id: str, name: str, status: str) -> dict[str, Any]:
    return {"type": "tool_call", "toolCall": {"id": call_id, "name": name, "status": status}}


def citation(citation_id: str, title: str, url: str = "") -> dict[str, Any]:
    payload: dict[str, Any] = {"id": citation_id, "title": title}
    if url:
        payload["url"] = url
    return {"type": "citation", "citation": payload}


def done() -> dict[str, Any]:
    return {"type": "done"}


def error(message: str) -> dict[str, Any]:
    return {"type": "error", "message": message}
