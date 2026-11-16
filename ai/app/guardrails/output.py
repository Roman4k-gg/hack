import re
from dataclasses import replace
from typing import Any

from app.agent.tools import ToolResult
from app.guardrails.injection import HIDDEN_FRAGMENT, find_injection
from app.guardrails.pii import mask_data

SENTENCE_SPLIT = re.compile(r"(?<=[.!?;\n])\s+")


def strip_injections(text: str, found: list[str]) -> str:
    if not find_injection(text):
        return text
    parts: list[str] = []
    hidden = False
    for sentence in SENTENCE_SPLIT.split(text):
        hits = find_injection(sentence)
        if not hits:
            parts.append(sentence)
            continue
        hidden = True
        found.extend(hits)
        if not parts or parts[-1] != HIDDEN_FRAGMENT:
            parts.append(HIDDEN_FRAGMENT)
    if not hidden:
        found.extend(find_injection(text))
        return HIDDEN_FRAGMENT
    return " ".join(parts)


def _clean(value: Any, found: list[str]) -> Any:
    if isinstance(value, str):
        return strip_injections(value, found)
    if isinstance(value, list):
        return [_clean(item, found) for item in value]
    if isinstance(value, dict):
        return {key: _clean(item, found) for key, item in value.items()}
    return value


def sanitize_result(result: ToolResult) -> tuple[ToolResult, list[str]]:
    if not result.ok or result.data is None:
        return result, []
    found: list[str] = []
    data = mask_data(_clean(result.data, found))
    return replace(result, data=data), sorted(set(found))
