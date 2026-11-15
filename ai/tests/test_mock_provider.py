import json

import pytest

from app.providers.base import ToolSpec
from app.providers.mock import (
    FALLBACK_TEXT,
    FORBIDDEN_TEXT,
    GREETING_TEXT,
    NOT_FOUND_TEXT,
    TOOL_ERROR_TEXT,
    MockProvider,
)

ALL_TOOLS = [
    ToolSpec(name, "", {"type": "object"})
    for name in ("get_my_profile", "get_onboarding_plan", "search_knowledge")
]

PLAN = {
    "intern": {"full_name": "Дарья Соколова"},
    "progress": {"done": 4, "total": 10, "percent": 40},
    "next_step": {"title": "Получить доступ к Jira", "reason": "Обязательный пункт недели 1"},
    "stages": [
        {
            "items": [
                {
                    "title": "Получить доступ к Jira",
                    "status": "blocked",
                    "blocked_reason": "Ждём IT",
                },
                {"title": "Настроить окружение", "status": "done"},
            ]
        }
    ],
}


def with_tool_result(question: str, name: str, payload: dict) -> list[dict]:
    return [
        {"role": "system", "content": "system"},
        {"role": "user", "content": question},
        {
            "role": "assistant",
            "content": None,
            "tool_calls": [{"id": "call-1", "type": "function", "function": {"name": name}}],
        },
        {"role": "tool", "tool_call_id": "call-1", "content": json.dumps(payload)},
    ]


async def ask(question: str, tools=ALL_TOOLS):
    return await MockProvider().complete([{"role": "user", "content": question}], tools)


@pytest.mark.parametrize(
    ("question", "tool", "arguments"),
    [
        ("Что мне делать дальше?", "get_onboarding_plan", {}),
        ("покажи мой прогресс", "get_onboarding_plan", {}),
        ("Кто я и в какой команде?", "get_my_profile", {}),
        ("Доступ к Jira?", "search_knowledge", {"query": "Доступ к Jira?"}),
        ("Что такое экватор", "search_knowledge", {"query": "Что такое экватор"}),
    ],
)
async def test_mock_picks_tool_by_intent(question: str, tool: str, arguments: dict):
    response = await ask(question)
    assert response.text == ""
    assert [call.name for call in response.tool_calls] == [tool]
    assert response.tool_calls[0].arguments == arguments


async def test_mock_greets_without_tools():
    response = await ask("Привет!")
    assert response.text == GREETING_TEXT
    assert response.tool_calls == []


async def test_mock_falls_back_when_tool_not_allowed():
    response = await ask("Что мне делать дальше?", tools=[])
    assert response.text == FALLBACK_TEXT


async def test_mock_without_user_message():
    response = await MockProvider().complete([{"role": "system", "content": "x"}], ALL_TOOLS)
    assert response.text == FALLBACK_TEXT


async def test_mock_empty_question():
    response = await ask("   ")
    assert response.text == FALLBACK_TEXT


async def test_mock_answers_from_plan():
    messages = with_tool_result("что дальше?", "get_onboarding_plan", {"ok": True, "result": PLAN})
    text = (await MockProvider().complete(messages, ALL_TOOLS)).text

    assert text.startswith("Дарья, вот где ты сейчас")
    assert "**4 из 10**" in text
    assert "**Следующий шаг:** Получить доступ к Jira — Обязательный пункт недели 1." in text
    assert "- Получить доступ к Jira — Ждём IT" in text


async def test_mock_plan_without_next_step():
    plan = {"progress": {"done": 10, "total": 10, "percent": 100}, "next_step": None}
    messages = with_tool_result("что дальше?", "get_onboarding_plan", {"ok": True, "result": plan})
    text = (await MockProvider().complete(messages, ALL_TOOLS)).text

    assert text.startswith("Вот где ты сейчас")
    assert "Все пункты плана закрыты" in text
    assert "блокеры" not in text


async def test_mock_answers_from_profile():
    profile = {"full_name": "Дарья Соколова", "position": "Стажёр", "team_name": "Search"}
    messages = with_tool_result("кто я?", "get_my_profile", {"ok": True, "result": profile})
    text = (await MockProvider().complete(messages, ALL_TOOLS)).text
    assert text == "Ты — **Дарья Соколова**, Стажёр, команда Search."


async def test_mock_answers_from_search():
    result = {
        "items": [
            {
                "title": "Как получить доступ к Jira",
                "chunk": "1. Напишите наставнику.",
                "stale": True,
            },
            {"title": "Настройка VPN", "chunk": ""},
        ]
    }
    messages = with_tool_result("jira?", "search_knowledge", {"ok": True, "result": result})
    text = (await MockProvider().complete(messages, ALL_TOOLS)).text

    assert text.startswith("Нашёл в документе «Как получить доступ к Jira»:")
    assert "1. Напишите наставнику." in text
    assert "устаревшей" in text
    assert "«Настройка VPN»" in text


async def test_mock_search_nothing_found():
    payload = {"ok": True, "result": {"items": []}}
    messages = with_tool_result("экватор?", "search_knowledge", payload)
    assert (await MockProvider().complete(messages, ALL_TOOLS)).text == NOT_FOUND_TEXT


@pytest.mark.parametrize(
    ("code", "text"),
    [("FORBIDDEN", FORBIDDEN_TEXT), ("TIMEOUT", TOOL_ERROR_TEXT)],
)
async def test_mock_explains_tool_errors(code: str, text: str):
    payload = {"ok": False, "error": {"code": code, "message": "x"}}
    messages = with_tool_result("что дальше?", "get_onboarding_plan", payload)
    assert (await MockProvider().complete(messages, ALL_TOOLS)).text == text


async def test_mock_unknown_tool_result_and_broken_json():
    messages = with_tool_result("что дальше?", "mystery", {"ok": True, "result": {}})
    assert (await MockProvider().complete(messages, ALL_TOOLS)).text == FALLBACK_TEXT

    messages[-1]["content"] = "not json"
    assert (await MockProvider().complete(messages, ALL_TOOLS)).text == TOOL_ERROR_TEXT
