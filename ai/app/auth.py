from dataclasses import dataclass, field
from typing import Annotated

import jwt
from fastapi import Depends, HTTPException, Request, status

from app.config import Settings, get_settings


@dataclass(frozen=True)
class User:
    id: str
    role: str
    token: str
    email: str = ""
    team_id: str = ""
    scopes: list[str] = field(default_factory=list)

    def has_scope(self, scope: str) -> bool:
        return "*" in self.scopes or scope in self.scopes


def decode_token(token: str, settings: Settings) -> User:
    claims = jwt.decode(
        token,
        settings.jwt_secret,
        algorithms=["HS256"],
        issuer=settings.jwt_issuer,
        options={"require": ["sub", "exp"]},
    )
    return User(
        id=claims["sub"],
        role=claims.get("role", ""),
        token=token,
        email=claims.get("email", ""),
        team_id=claims.get("team_id", ""),
        scopes=list(claims.get("scopes") or []),
    )


def current_user(request: Request, settings: Annotated[Settings, Depends(get_settings)]) -> User:
    header = request.headers.get("Authorization", "")
    scheme, _, token = header.partition(" ")
    if scheme.lower() != "bearer" or not token.strip():
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "Нет токена")
    try:
        return decode_token(token.strip(), settings)
    except jwt.PyJWTError as exc:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "Токен недействителен") from exc
