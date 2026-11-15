import re
from datetime import date, timedelta

WEEKDAY_STEMS = ("понедельник", "вторник", "сред", "четверг", "пятниц", "суббот", "воскресень")
MONTHS = (
    "январ",
    "феврал",
    "март",
    "апрел",
    "ма",
    "июн",
    "июл",
    "август",
    "сентябр",
    "октябр",
    "ноябр",
    "декабр",
)
MONTH_PATTERN = (
    r"(\d{1,2})(?:\s*-\s*(\d{1,2}))?\s+"
    r"(января|февраля|марта|апреля|мая|июня|июля|августа|сентября|октября|ноября|декабря)"
)
TODAY_PATTERN = re.compile(r"Сегодня: (\d{4}-\d{2}-\d{2})")


def today_from_system(messages: list[dict]) -> date:
    for message in messages:
        if message.get("role") == "system":
            match = TODAY_PATTERN.search(str(message.get("content") or ""))
            if match:
                return date.fromisoformat(match.group(1))
    return date.today()


def _future(day: int, month: int, year: int | None, today: date) -> date | None:
    try:
        value = date(year or today.year, month, day)
    except ValueError:
        return None
    if year is None and value < today:
        value = date(today.year + 1, month, day)
    return value


def _month_number(name: str) -> int:
    for index, stem in enumerate(MONTHS):
        if name.startswith(stem):
            return index + 1
    return 0


def find_dates(text: str, today: date) -> list[date]:
    lowered = text.lower()
    found: list[tuple[int, date]] = []

    for match in re.finditer(r"(\d{4})-(\d{2})-(\d{2})", lowered):
        value = _future(int(match.group(3)), int(match.group(2)), int(match.group(1)), today)
        if value:
            found.append((match.start(), value))

    for match in re.finditer(r"\b(\d{1,2})\.(\d{1,2})(?:\.(\d{4}))?\b", lowered):
        year = int(match.group(3)) if match.group(3) else None
        value = _future(int(match.group(1)), int(match.group(2)), year, today)
        if value:
            found.append((match.start(), value))

    for match in re.finditer(r"\bс\s+(\d{1,2})\s+по\s+" + MONTH_PATTERN, lowered):
        month = _month_number(match.group(4))
        start = _future(int(match.group(1)), month, None, today)
        end = _future(int(match.group(2)), month, None, today)
        if start and end:
            found.extend([(match.start(), start), (match.start() + 1, end)])
    if not found:
        for match in re.finditer(MONTH_PATTERN, lowered):
            month = _month_number(match.group(3))
            start = _future(int(match.group(1)), month, None, today)
            if start:
                found.append((match.start(), start))
            if match.group(2):
                end = _future(int(match.group(2)), month, None, today)
                if end:
                    found.append((match.start() + 1, end))

    position = lowered.find("послезавтра")
    if position >= 0:
        found.append((position, today + timedelta(days=2)))
    else:
        position = lowered.find("завтра")
        if position >= 0:
            found.append((position, today + timedelta(days=1)))
    position = lowered.find("сегодня")
    if position >= 0:
        found.append((position, today))

    for weekday, stem in enumerate(WEEKDAY_STEMS):
        position = lowered.find(stem)
        if position >= 0:
            ahead = (weekday - today.weekday()) % 7 or 7
            found.append((position, today + timedelta(days=ahead)))

    return [value for _, value in sorted(found, key=lambda item: item[0])]


def absence_type(text: str) -> str:
    lowered = text.lower()
    if "больничн" in lowered or "заболел" in lowered or "болею" in lowered:
        return "sick_leave"
    if "отпуск" in lowered:
        return "vacation"
    return "day_off"
