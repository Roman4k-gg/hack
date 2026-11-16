import time
from collections import deque
from collections.abc import Callable
from dataclasses import dataclass, field

MAX_STORED_CHARS = 1000


@dataclass
class Turn:
    user: str
    assistant: str
    at: float


@dataclass
class ConversationStore:
    max_turns: int = 5
    ttl: float = 1800.0
    clock: Callable[[], float] = time.monotonic
    _turns: dict[str, deque[Turn]] = field(default_factory=dict)

    def messages(self, user_id: str) -> list[dict[str, str]]:
        turns = self._turns.get(user_id)
        if not turns:
            return []
        now = self.clock()
        while turns and turns[0].at <= now - self.ttl:
            turns.popleft()
        result: list[dict[str, str]] = []
        for turn in turns:
            result.append({"role": "user", "content": turn.user})
            result.append({"role": "assistant", "content": turn.assistant})
        return result

    def add(self, user_id: str, user: str, assistant: str) -> None:
        if self.max_turns <= 0 or not user or not assistant:
            return
        turns = self._turns.setdefault(user_id, deque(maxlen=self.max_turns))
        turns.append(Turn(user[:MAX_STORED_CHARS], assistant[:MAX_STORED_CHARS], self.clock()))

    def clear(self, user_id: str) -> None:
        for key in [key for key in self._turns if owned_by(key, user_id)]:
            del self._turns[key]


def owned_by(key: str, user_id: str) -> bool:
    return key == user_id or key.startswith(f"{user_id}:")
