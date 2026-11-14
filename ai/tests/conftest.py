import time

import jwt
import pytest

from app.auth import User

SECRET = "dev-secret-change-me"


def make_token(role: str = "intern", scopes: list[str] | None = None, **extra) -> str:
    claims = {
        "sub": "22222222-2222-2222-2222-222222222224",
        "iss": "ai-buddy-core",
        "exp": int(time.time()) + 300,
        "email": "dasha@demo.local",
        "role": role,
        "team_id": "11111111-1111-1111-1111-111111111111",
        "scopes": scopes
        if scopes is not None
        else ["onboarding:plan:read", "onboarding:plan:write", "chat:use", "kb:read"],
    }
    claims.update(extra)
    return jwt.encode(claims, SECRET, algorithm="HS256")


@pytest.fixture
def intern() -> User:
    return User(
        id="22222222-2222-2222-2222-222222222224",
        role="intern",
        token="intern-token",
        team_id="11111111-1111-1111-1111-111111111111",
        scopes=["onboarding:plan:read", "chat:use", "kb:read"],
    )
