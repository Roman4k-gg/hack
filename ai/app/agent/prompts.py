from datetime import date
from functools import lru_cache
from pathlib import Path

from app.auth import User

PROMPTS_DIR = Path(__file__).resolve().parent.parent / "prompts"


@lru_cache
def load_system_prompt(version: str) -> str:
    return (PROMPTS_DIR / version / "system.md").read_text(encoding="utf-8").strip()


def build_system_message(user: User, version: str, today: date | None = None) -> dict[str, str]:
    today = today or date.today()
    context = "\n".join(
        [
            "Контекст:",
            f"- Сегодня: {today.isoformat()}",
            f"- Роль пользователя: {user.role or 'неизвестна'}",
            f"- Команда (id): {user.team_id or 'не указана'}",
        ]
    )
    return {"role": "system", "content": f"{load_system_prompt(version)}\n\n{context}"}
