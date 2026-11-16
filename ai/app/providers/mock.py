import json
import re
from datetime import date
from typing import Any

from app.guardrails.injection import INJECTION_NOTE
from app.providers.base import LLMResponse, ToolCall, ToolSpec
from app.providers.mock_dates import absence_type, find_dates, today_from_system

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
INJECTION_TEXT = (
    "Я не могу менять свои правила, раскрывать служебные инструкции или выполнять действия "
    "в обход подтверждения. Могу помочь с планом адаптации, базой знаний или заявками."
)
TAINTED_TEXT = (
    "В найденном документе была подозрительная инструкция, поэтому я ничего не выполнял. "
    "Если нужна заявка — попроси меня прямо."
)

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
REQUEST_WORDS = ("мои заявк", "статус заяв", "что с заявк", "где заявк", "мою заявку", "моей заявк")
ACCESS_WORDS = (
    "доступы",
    "пакет доступ",
    "оформи доступ",
    "запроси доступ",
    "заявку на доступ",
    "заявки на доступ",
    "выдать доступ",
)
ABSENCE_WORDS = ("отгул", "отпуск", "больничн", "выходной", "отсутств", "не выйду", "заболел")
HOW_WORDS = ("как ", "правил", "политик", "порядок", "сколько", "можно ли")
CONFIRM_WORDS = {
    "да",
    "подтверждаю",
    "подтвердить",
    "подтверждаем",
    "отправляй",
    "отправить",
    "ок",
    "окей",
    "давай",
    "согласен",
    "согласна",
    "верно",
}
DECLINE_WORDS = ("нет", "отмен", "не надо", "передумал", "стоп")
HANDOFF_WORDS = (
    "передай наставник",
    "передать наставник",
    "спроси наставник",
    "спроси у наставник",
    "задай вопрос наставник",
    "эскалир",
)
NOT_FOUND_SHORT_TEXT = "Не нашёл ответа в базе знаний."
HANDOFF_NEXT_TEXT = "Наставник ответит в чате или на почте."
HANDOFF_UNAVAILABLE_TEXT = (
    "Передача вопросов наставнику пока не подключена в системе. "
    "Напиши наставнику напрямую — вопрос уже сформулирован выше."
)
PENDING_PATTERN = re.compile(r"Инструмент для подтверждения: (\w+)")

ASK_DATE_TEXT = (
    "На какую дату оформить? Напиши, например: «отгул в пятницу», "
    "«отгул 20.11» или «отпуск с 20 по 24 ноября»."
)
DECLINED_TEXT = "Хорошо, ничего не отправляю. Если передумаешь — просто напиши ещё раз."
CONFIRMATION_TEXT = (
    "Сначала покажу черновик заявки, а отправлю только после твоего подтверждения. "
    "Напиши ещё раз, что нужно оформить: отгул с датами или пакет доступов."
)
ABSENCE_LABELS = {"vacation": "Отпуск", "day_off": "Отгул", "sick_leave": "Больничный"}
STATUS_LABELS = {
    "draft": "черновик",
    "pending": "на согласовании",
    "approved": "согласована",
    "rejected": "отклонена",
    "completed": "выполнена",
    "failed": "ошибка",
    "cancelled": "отменена",
}


def _last_user_index(messages: list[dict[str, Any]]) -> int:
    for index in range(len(messages) - 1, -1, -1):
        if messages[index].get("role") == "user":
            return index
    return -1


def _previous_exchange(messages: list[dict[str, Any]], user_index: int) -> tuple[str, str]:
    earlier = [message for message in messages[:user_index] if message.get("role") != "system"]
    if len(earlier) < 2:
        return "", ""
    question, answer = earlier[-2], earlier[-1]
    if question.get("role") != "user" or answer.get("role") != "assistant":
        return "", ""
    return str(question.get("content") or ""), str(answer.get("content") or "")


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


def _ru_date(value: Any) -> str:
    try:
        return date.fromisoformat(str(value)[:10]).strftime("%d.%m.%Y")
    except ValueError:
        return str(value)


def _preview_text(preview: dict[str, Any]) -> str:
    draft = preview.get("draft") or {}
    date_from, date_to = draft.get("date_from"), draft.get("date_to")
    dates = _ru_date(date_from)
    if date_to and date_to != date_from:
        dates = f"{dates} – {_ru_date(date_to)}"
    kind = ABSENCE_LABELS.get(str(draft.get("absence_type")), "Отсутствие")
    lines = ["Подготовил черновик заявки:", f"- **Тип:** {kind}", f"- **Даты:** {dates}"]
    if draft.get("comment"):
        lines.append(f"- **Комментарий:** {draft['comment']}")
    notes = preview.get("policy_notes") or []
    if notes:
        lines.append("\nПравила:")
        lines.extend(f"- {note}" for note in notes)
    lines.append("\nОтправить заявку? Напиши «подтверждаю» — или «нет», если передумал.")
    return "\n".join(lines)


def _created_text(request: dict[str, Any]) -> str:
    status = STATUS_LABELS.get(str(request.get("status")), str(request.get("status", "")))
    lines = ["Заявка отправлена ✅"]
    if request.get("external_id"):
        lines.append(f"- **Номер:** {request['external_id']}")
    lines.append(f"- **Статус:** {status}")
    approver = (request.get("approver") or {}).get("full_name")
    if approver:
        lines.append(f"- **Согласует:** {approver}")
    if request.get("next_step"):
        lines.append(f"\n{request['next_step']}")
    return "\n".join(lines)


def _access_preview_text(preview: dict[str, Any]) -> str:
    items = preview.get("items") or []
    if not items:
        return "Для твоей роли нет доступов, которые нужно запрашивать."
    lines = ["Подготовил пакет доступов:"]
    for item in items:
        approver = f" (согласует {item['approver']})" if item.get("approver") else ""
        reason = f" — {item['reason']}" if item.get("reason") else ""
        lines.append(f"- **{item.get('system', '')}**{reason}{approver}")
    lines.append("\nОтправить заявки? Напиши «подтверждаю» — или «нет», если не нужно.")
    return "\n".join(lines)


def _access_created_text(result: dict[str, Any]) -> str:
    lines = ["Заявки на доступы отправлены ✅"]
    for item in result.get("requests") or []:
        status = STATUS_LABELS.get(str(item.get("status")), str(item.get("status", "")))
        number = f" ({item['external_id']})" if item.get("external_id") else ""
        lines.append(f"- {item.get('system', '')} — {status}{number}")
    if result.get("next_step"):
        lines.append(f"\n{result['next_step']}")
    return "\n".join(lines)


def _requests_text(result: dict[str, Any]) -> str:
    items = result.get("items") or []
    if not items:
        return "У тебя пока нет заявок."
    lines = ["Твои заявки:"]
    for item in items:
        status = STATUS_LABELS.get(str(item.get("status")), str(item.get("status", "")))
        number = f" ({item['external_id']})" if item.get("external_id") else ""
        approver = f", согласует {item['approver']}" if item.get("approver") else ""
        lines.append(f"- {item.get('title', 'Заявка')} — {status}{number}{approver}")
    return "\n".join(lines)


def _handoff_preview_text(preview: dict[str, Any]) -> str:
    question = (preview.get("draft") or {}).get("question", "")
    return (
        f"Передам наставнику вопрос:\n\n> {question}\n\n"
        "Отправить? Напиши «да» — или «нет», если не нужно."
    )


def _handoff_created_text(result: dict[str, Any]) -> str:
    lines = ["Вопрос передан наставнику ✅"]
    if result.get("mentor"):
        lines.append(f"- **Наставник:** {result['mentor']}")
    if result.get("external_id"):
        lines.append(f"- **Номер:** {result['external_id']}")
    lines.append(f"\n{result.get('next_step') or HANDOFF_NEXT_TEXT}")
    return "\n".join(lines)


def _error_text(error: dict[str, Any], name: str = "") -> str:
    code = error.get("code")
    if name == "create_handoff" and code == "NOT_FOUND":
        return HANDOFF_UNAVAILABLE_TEXT
    if code == "FORBIDDEN":
        return FORBIDDEN_TEXT
    if code == "CONFIRMATION_REQUIRED":
        return CONFIRMATION_TEXT
    if code == "INJECTION_SUSPECTED":
        return TAINTED_TEXT
    if code == "VALIDATION_ERROR":
        return f"Не получилось: {error.get('message') or 'проверь данные'}."
    return TOOL_ERROR_TEXT


def _answer_from_tool(name: str, payload: dict[str, Any]) -> str:
    if not payload.get("ok"):
        return _error_text(payload.get("error") or {}, name)
    result = payload.get("result") or {}
    builders = {
        "get_onboarding_plan": _plan_text,
        "get_my_profile": _profile_text,
        "search_knowledge": _search_text,
        "preview_absence_request": _preview_text,
        "create_absence_request": _created_text,
        "get_my_requests": _requests_text,
        "preview_access_bundle": _access_preview_text,
        "create_access_requests": _access_created_text,
        "preview_handoff": _handoff_preview_text,
        "create_handoff": _handoff_created_text,
    }
    builder = builders.get(name)
    return builder(result) if builder else FALLBACK_TEXT


def _flagged(messages: list[dict[str, Any]]) -> bool:
    return any(
        message.get("role") == "system" and INJECTION_NOTE in str(message.get("content") or "")
        for message in messages
    )


def _pending_tool(messages: list[dict[str, Any]]) -> str:
    for message in messages:
        if message.get("role") == "system":
            match = PENDING_PATTERN.search(str(message.get("content") or ""))
            if match:
                return match.group(1)
    return ""


class MockProvider:
    name = "mock"

    @staticmethod
    def _absence(question: str, messages: list[dict[str, Any]], call: Any) -> LLMResponse | None:
        dates = find_dates(question, today_from_system(messages))
        if dates:
            return call(
                "preview_absence_request",
                {
                    "date_from": dates[0].isoformat(),
                    "date_to": dates[-1].isoformat(),
                    "absence_type": absence_type(question),
                    "comment": "",
                },
            )
        if _contains(question.lower() + " ", HOW_WORDS):
            return call("search_knowledge", {"query": question})
        return LLMResponse(text=ASK_DATE_TEXT, model="mock")

    async def complete(self, messages: list[dict[str, Any]], tools: list[ToolSpec]) -> LLMResponse:
        user_index = _last_user_index(messages)
        if user_index < 0:
            return LLMResponse(text=FALLBACK_TEXT, model="mock")

        question = str(messages[user_index].get("content") or "").strip()
        lowered = question.lower()
        available = {tool.name for tool in tools}

        def call(name: str, arguments: dict[str, Any] | None = None) -> LLMResponse | None:
            if name not in available:
                return None
            return LLMResponse(tool_calls=[ToolCall(id="", name=name, arguments=arguments or {})])

        results = _tool_results(messages, user_index)
        if results:
            name, payload = results[-1]
            found_nothing = (
                name == "search_knowledge"
                and payload.get("ok")
                and not (payload.get("result") or {}).get("items")
            )
            handoff = call("preview_handoff", {"question": question}) if found_nothing else None
            if handoff:
                return handoff
            text = _answer_from_tool(name, payload)
            if name == "preview_handoff" and any(n == "search_knowledge" for n, _ in results):
                text = f"{NOT_FOUND_SHORT_TEXT}\n\n{text}"
            return LLMResponse(text=text, model="mock")

        if _flagged(messages):
            return LLMResponse(text=INJECTION_TEXT, model="mock")

        words = set(re.findall(r"[a-zа-яё]+", lowered))
        pending_tool = _pending_tool(messages)
        asked_before, answered_before = _previous_exchange(messages, user_index)
        continues_absence = answered_before == ASK_DATE_TEXT and bool(
            find_dates(question, today_from_system(messages))
        )
        if pending_tool and words & CONFIRM_WORDS and not _contains(lowered, DECLINE_WORDS):
            response = call(pending_tool)
        elif pending_tool and _contains(lowered, DECLINE_WORDS):
            return LLMResponse(text=DECLINED_TEXT, model="mock")
        elif continues_absence:
            response = self._absence(f"{asked_before} {question}", messages, call)
        elif _contains(lowered, HANDOFF_WORDS):
            to_send = asked_before if asked_before and len(words) <= 5 else question
            response = call("preview_handoff", {"question": to_send})
        elif _contains(lowered, PROFILE_WORDS):
            response = call("get_my_profile")
        elif _contains(lowered, REQUEST_WORDS):
            response = call("get_my_requests")
        elif _contains(lowered, ACCESS_WORDS):
            response = call("preview_access_bundle", {"bundle": "intern"})
        elif _contains(lowered, ABSENCE_WORDS):
            response = self._absence(question, messages, call)
        elif _contains(lowered, PLAN_WORDS):
            response = call("get_onboarding_plan")
        elif _contains(lowered, GREETING_WORDS) and len(lowered.split()) <= 4:
            return LLMResponse(text=GREETING_TEXT, model="mock")
        elif question:
            response = call("search_knowledge", {"query": question})
        else:
            response = None
        return response or LLMResponse(text=FALLBACK_TEXT, model="mock")
