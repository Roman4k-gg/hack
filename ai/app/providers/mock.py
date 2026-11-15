import json
from typing import Any

from app.providers.base import LLMResponse, ToolCall, ToolSpec

GREETING_TEXT = (
    "Привет! Я AI-buddy — помогаю стажёрам в первые недели. "
    "Могу подсказать следующий шаг по карте адаптации или найти ответ в базе знаний. "
    "Например, спроси: «Что мне делать дальше?» или «Как получить доступ к Jira?»"
)
FALLBACK_TEXT = (
    "Я AI-buddy и сейчас работаю в демо-режиме. "
    "Спроси, что тебе делать дальше, или как получить доступ к Jira."
)
NOT_FOUND_TEXT = (
    "Не нашёл ответа в базе знаний. Лучше уточнить у наставника — "
    "он подскажет и, возможно, мы добавим этот ответ в документы."
)
FORBIDDEN_TEXT = "У тебя нет прав на эти данные. Если они нужны для работы, обратись к наставнику."
TOOL_ERROR_TEXT = "Не получилось получить данные, сервис сейчас недоступен. Попробуй чуть позже."

PLAN_WORDS = (
    "дальше",
    "следующ",
    "план",
    "карт",
    "прогресс",
    "этап",
    "что делать",
    "чем заняться",
    "задачи на",
)
PROFILE_WORDS = ("кто я", "профил", "обо мне", "моя команда", "мой наставник", "моя роль")
GREETING_WORDS = ("привет", "здравствуй", "добрый день", "добрый вечер", "доброе утро", "hello")


def _last_user_index(messages: list[dict[str, Any]]) -> int:
    for index in range(len(messages) - 1, -1, -1):
        if messages[index].get("role") == "user":
            return index
    return -1


def _tool_results(messages: list[dict[str, Any]], start: int) -> list[tuple[str, dict[str, Any]]]:
    names: dict[str, str] = {}
    results: list[tuple[str, dict[str, Any]]] = []
    for message in messages[start + 1 :]:
        for call in message.get("tool_calls") or []:
            names[call.get("id", "")] = call.get("function", {}).get("name", "")
        if message.get("role") == "tool":
            try:
                payload = json.loads(message.get("content") or "{}")
            except ValueError:
                payload = {}
            results.append((names.get(message.get("tool_call_id", ""), ""), payload))
    return results


def _contains(text: str, words: tuple[str, ...]) -> bool:
    return any(word in text for word in words)


def _plan_text(plan: dict[str, Any]) -> str:
    lines: list[str] = []
    intern = plan.get("intern") or {}
    name = str(intern.get("full_name") or "").split(" ")[0]
    progress = plan.get("progress") or {}
    greeting = f"{name}, вот" if name else "Вот"
    lines.append(
        f"{greeting} где ты сейчас: закрыто **{progress.get('done', 0)} "
        f"из {progress.get('total', 0)}** пунктов ({progress.get('percent', 0)}%)."
    )

    next_step = plan.get("next_step")
    if next_step:
        reason = next_step.get("reason")
        suffix = f" — {reason}" if reason else ""
        lines.append(f"\n**Следующий шаг:** {next_step.get('title', '')}{suffix}.")
    else:
        lines.append("\nВсе пункты плана закрыты — отличная работа!")

    blocked = [
        item
        for stage in plan.get("stages") or []
        for item in stage.get("items") or []
        if item.get("status") == "blocked"
    ]
    if blocked:
        lines.append("\nЕсть блокеры:")
        for item in blocked:
            reason = item.get("blocked_reason") or "причина не указана"
            lines.append(f"- {item.get('title', '')} — {reason}")
        lines.append("\nЕсли блокер не снимается, напиши наставнику.")
    return "\n".join(lines)


def _profile_text(profile: dict[str, Any]) -> str:
    parts = [f"Ты — **{profile.get('full_name', 'стажёр')}**"]
    if profile.get("position"):
        parts.append(f"{profile['position']}")
    if profile.get("team_name"):
        parts.append(f"команда {profile['team_name']}")
    return ", ".join(parts) + "."


def _search_text(result: dict[str, Any]) -> str:
    items = result.get("items") or []
    if not items:
        return NOT_FOUND_TEXT
    top = items[0]
    lines = [f"Нашёл в документе «{top.get('title', '')}»:"]
    chunk = str(top.get("chunk") or top.get("summary") or "").strip()
    if chunk:
        lines.append(f"\n{chunk}")
    if top.get("stale"):
        lines.append("\n⚠️ Документ давно не обновлялся — информация может быть устаревшей.")
    others = ", ".join(f"«{item.get('title', '')}»" for item in items[1:])
    if others:
        lines.append(f"\nЕщё может пригодиться: {others}.")
    return "\n".join(lines)


def _answer_from_tool(name: str, payload: dict[str, Any]) -> str:
    if not payload.get("ok"):
        code = (payload.get("error") or {}).get("code")
        return FORBIDDEN_TEXT if code == "FORBIDDEN" else TOOL_ERROR_TEXT
    result = payload.get("result") or {}
    if name == "get_onboarding_plan":
        return _plan_text(result)
    if name == "get_my_profile":
        return _profile_text(result)
    if name == "search_knowledge":
        return _search_text(result)
    return FALLBACK_TEXT


class MockProvider:
    name = "mock"

    async def complete(self, messages: list[dict[str, Any]], tools: list[ToolSpec]) -> LLMResponse:
        user_index = _last_user_index(messages)
        if user_index < 0:
            return LLMResponse(text=FALLBACK_TEXT, model="mock")

        results = _tool_results(messages, user_index)
        if results:
            name, payload = results[-1]
            return LLMResponse(text=_answer_from_tool(name, payload), model="mock")

        question = str(messages[user_index].get("content") or "").strip()
        lowered = question.lower()
        available = {tool.name for tool in tools}

        def call(name: str, arguments: dict[str, Any] | None = None) -> LLMResponse | None:
            if name not in available:
                return None
            return LLMResponse(tool_calls=[ToolCall(id="", name=name, arguments=arguments or {})])

        if _contains(lowered, PROFILE_WORDS):
            response = call("get_my_profile")
        elif _contains(lowered, PLAN_WORDS):
            response = call("get_onboarding_plan")
        elif _contains(lowered, GREETING_WORDS) and len(lowered.split()) <= 4:
            return LLMResponse(text=GREETING_TEXT, model="mock")
        elif question:
            response = call("search_knowledge", {"query": question})
        else:
            response = None
        return response or LLMResponse(text=FALLBACK_TEXT, model="mock")
