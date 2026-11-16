import logging
import time
from dataclasses import dataclass, field
from typing import Any

from app.observability import log_event
from app.providers.base import LLMResponse

logger = logging.getLogger("app.agent.run")


@dataclass
class AgentRun:
    trace_id: str
    user_id: str
    role: str
    prompt_version: str
    message_length: int
    clock: Any = time.perf_counter
    started: float = 0.0
    model: str = ""
    iterations: int = 0
    llm_retries: int = 0
    input_tokens: int = 0
    output_tokens: int = 0
    tools: list[dict[str, Any]] = field(default_factory=list)
    input_flags: list[str] = field(default_factory=list)
    tool_flags: list[str] = field(default_factory=list)
    history_messages: int = 0
    status: str = "cancelled"
    error: str = ""

    def __post_init__(self) -> None:
        self.started = self.clock()

    def elapsed_ms(self, since: float | None = None) -> int:
        return int((self.clock() - (self.started if since is None else since)) * 1000)

    def add_response(self, response: LLMResponse) -> None:
        self.iterations += 1
        self.model = response.model or self.model
        self.input_tokens += int(response.usage.get("input_tokens", 0))
        self.output_tokens += int(response.usage.get("output_tokens", 0))

    def add_tool(self, name: str, ok: bool, error_code: str, started: float) -> None:
        self.tools.append(
            {
                "name": name,
                "status": "success" if ok else "error",
                "error_code": error_code or None,
                "latency_ms": self.elapsed_ms(started),
            }
        )

    def finish(self, status: str, error: str = "") -> None:
        self.status = status
        self.error = error

    def as_dict(self) -> dict[str, Any]:
        return {
            "trace_id": self.trace_id,
            "user_id": self.user_id,
            "role": self.role,
            "prompt_version": self.prompt_version,
            "model": self.model,
            "status": self.status,
            "error": self.error or None,
            "latency_ms": self.elapsed_ms(),
            "tool_iterations": self.iterations,
            "llm_retries": self.llm_retries,
            "input_tokens": self.input_tokens,
            "output_tokens": self.output_tokens,
            "message_length": self.message_length,
            "history_messages": self.history_messages,
            "tool_calls": self.tools,
            "injection_input": self.input_flags,
            "injection_tools": self.tool_flags,
        }

    def log(self) -> dict[str, Any]:
        record = self.as_dict()
        level = logging.INFO if self.status in ("ok", "limit") else logging.WARNING
        log_event(logger, "agent_run", level=level, **record)
        return record
