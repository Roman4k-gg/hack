import time
from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any

from app.agent.history import owned_by

HIDDEN_FIELDS = ("preview_id", "confirmation_token", "payload")


@dataclass
class PendingAction:
    tool: str
    preview_id: str
    confirmation_token: str
    draft: dict[str, Any]
    turn_id: str
    expires_at: float
    payload: dict[str, Any] = field(default_factory=dict)

    def confirm_args(self) -> dict[str, Any]:
        return {
            **self.payload,
            "preview_id": self.preview_id,
            "confirmation_token": self.confirmation_token,
            "idempotency_key": self.preview_id,
        }


@dataclass
class PendingStore:
    ttl: float = 600.0
    clock: Callable[[], float] = time.monotonic
    _items: dict[str, PendingAction] = field(default_factory=dict)

    def put(self, user_id: str, tool: str, preview: dict[str, Any], turn_id: str) -> None:
        payload = preview.get("payload")
        self._items[user_id] = PendingAction(
            payload=dict(payload) if isinstance(payload, dict) else {},
            tool=tool,
            preview_id=str(preview.get("preview_id") or ""),
            confirmation_token=str(preview.get("confirmation_token") or ""),
            draft={key: value for key, value in preview.items() if key not in HIDDEN_FIELDS},
            turn_id=turn_id,
            expires_at=self.clock() + self.ttl,
        )

    def get(self, user_id: str) -> PendingAction | None:
        action = self._items.get(user_id)
        if action is None:
            return None
        if action.expires_at <= self.clock():
            self._items.pop(user_id, None)
            return None
        return action

    def pop(self, user_id: str) -> None:
        self._items.pop(user_id, None)

    def clear(self, user_id: str) -> None:
        for key in [key for key in self._items if owned_by(key, user_id)]:
            del self._items[key]
