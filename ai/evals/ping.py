import argparse
import asyncio
import json
import sys
import time
from typing import Any
from urllib.parse import urlparse

import httpx

from app.agent.loop import assistant_tool_message
from app.agent.tools import DEFAULT_TOOLS
from app.config import Settings, get_settings
from app.providers import build_provider
from app.providers.base import LLMError, LLMProvider, LLMResponse

SYSTEM = "Ты помощник стажёра. На вопросы о плане адаптации отвечай через get_onboarding_plan."
QUESTION = "Что мне делать дальше по плану адаптации?"
PLAN_RESULT = {"next_step": {"title": "Получить доступ к Jira"}}
PLAN_TOOL = next(tool.spec for tool in DEFAULT_TOOLS if tool.name == "get_onboarding_plan")
HINTS = {
    "location is not supported": "провайдер недоступен из твоего региона — нужен VPN",
    "thought_signature": "провайдер требует вернуть подпись вызова — проверь extra в ToolCall",
    "API key not valid": "проверь LLM_API_KEY",
    "401": "проверь LLM_API_KEY",
    "403": "у ключа нет доступа к этой модели",
    "404": "проверь LLM_BASE_URL и LLM_MODEL",
    "400": "провайдер не принял запрос — возможно, модель не умеет tools",
    "429": "лимит или закончились деньги на ключе",
}


def hint_for(message: str) -> str:
    return next((hint for code, hint in HINTS.items() if code in message), "")


def failure(step: str, exc: LLMError) -> tuple[bool, str]:
    hint = hint_for(str(exc))
    return False, f"{step}: {exc} — {hint}" if hint else f"{step}: {exc}"


def tokens(*responses: LLMResponse) -> str:
    used_in = sum(response.usage.get("input_tokens", 0) for response in responses)
    used_out = sum(response.usage.get("output_tokens", 0) for response in responses)
    return f"{used_in}+{used_out}"


async def ping(provider: LLMProvider) -> tuple[bool, str]:
    messages: list[dict[str, Any]] = [
        {"role": "system", "content": SYSTEM},
        {"role": "user", "content": QUESTION},
    ]
    try:
        first = await provider.complete(messages, [PLAN_TOOL])
    except LLMError as exc:
        return failure("первый запрос", exc)
    if not any(call.name == PLAN_TOOL.name for call in first.tool_calls):
        return False, f"модель ответила без вызова инструмента: {first.text[:200]!r}"

    calls = [(call.id or f"call-{index}", call) for index, call in enumerate(first.tool_calls)]
    messages.append(assistant_tool_message(first.text, calls))
    for call_id, _ in calls:
        content = json.dumps({"ok": True, "result": PLAN_RESULT}, ensure_ascii=False)
        messages.append({"role": "tool", "tool_call_id": call_id, "content": content})
    try:
        final = await provider.complete(messages, [PLAN_TOOL])
    except LLMError as exc:
        return failure("ответ по результату инструмента", exc)
    if not final.text.strip():
        return False, "пустой ответ после инструмента — увеличь LLM_MAX_TOKENS"
    return True, (
        f"модель {first.model} вызвала {PLAN_TOOL.name} и ответила по результату, "
        f"токены {tokens(first, final)}"
    )


def list_models(settings: Settings, client: httpx.Client | None = None) -> int:
    if not settings.llm_api_key:
        print("FAIL: LLM_API_KEY не задан")
        return 1
    client = client or httpx.Client(timeout=settings.llm_timeout)
    try:
        response = client.get(
            f"{settings.llm_base_url.rstrip('/')}/models",
            headers={"Authorization": f"Bearer {settings.llm_api_key}"},
        )
    except httpx.HTTPError as exc:
        print(f"FAIL: список моделей недоступен: {type(exc).__name__}")
        return 1
    if response.status_code >= 400:
        print(f"FAIL: список моделей вернул {response.status_code}")
        return 1
    items = response.json().get("data") or []
    names = sorted(str(item.get("id", "")).removeprefix("models/") for item in items)
    for name in names:
        print(name)
    return 0 if names else 1


def main(
    argv: list[str] | None = None,
    settings: Settings | None = None,
    provider: LLMProvider | None = None,
    client: httpx.Client | None = None,
) -> int:
    parser = argparse.ArgumentParser(description="Проверка ключа и модели LLM")
    parser.add_argument("--models", action="store_true", help="показать доступные модели")
    options = parser.parse_args(argv)
    settings = settings or get_settings()
    if options.models:
        return list_models(settings, client)
    provider = provider or build_provider(settings.model_copy(update={"llm_mode": "real"}))
    key = "задан" if settings.llm_api_key else "не задан"
    host = urlparse(settings.llm_base_url).netloc or settings.llm_base_url
    print(f"LLM: {host}, модель {settings.llm_model}, ключ {key}")
    started = time.perf_counter()
    ok, message = asyncio.run(ping(provider))
    elapsed = int((time.perf_counter() - started) * 1000)
    print(f"{'OK' if ok else 'FAIL'}: {message} ({elapsed} мс)")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
