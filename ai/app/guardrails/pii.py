import re
from typing import Any

EMAIL_MASK = "[email скрыт]"
PHONE_MASK = "[телефон скрыт]"
SECRET_MASK = "[секрет скрыт]"

EMAIL = re.compile(r"[\w.+-]+@[\w-]+(\.[\w-]+)+")
PHONE = re.compile(r"(?<![\w+])(\+7|8)[\s(-]*\d{3}[\s)-]*\d{3}[\s-]*\d{2}[\s-]*\d{2}(?!\d)")
SECRETS = [
    re.compile(r"\bsk-[A-Za-z0-9_-]{16,}"),
    re.compile(r"\bghp_[A-Za-z0-9]{20,}"),
    re.compile(r"\bAKIA[0-9A-Z]{16}\b"),
    re.compile(r"\beyJ[\w-]{8,}\.[\w-]{8,}\.[\w-]{8,}"),
    re.compile(r"(?i)\b(парол\w*|password|passwd|token|токен\w*|api[_-]?key)(\s*[:=]\s*)(?!\[)\S+"),
]


def _mask_secret(match: re.Match[str]) -> str:
    if match.lastindex and match.lastindex >= 2:
        return f"{match.group(1)}{match.group(2)}{SECRET_MASK}"
    return SECRET_MASK


def mask_pii(text: str) -> str:
    for pattern in SECRETS:
        text = pattern.sub(_mask_secret, text)
    text = EMAIL.sub(EMAIL_MASK, text)
    return PHONE.sub(PHONE_MASK, text)


def mask_data(value: Any) -> Any:
    if isinstance(value, str):
        return mask_pii(value)
    if isinstance(value, list):
        return [mask_data(item) for item in value]
    if isinstance(value, dict):
        return {key: mask_data(item) for key, item in value.items()}
    return value
