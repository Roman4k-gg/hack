import json
import logging
from datetime import UTC, datetime
from typing import Any

SERVICE = "ai"
FIELDS_ATTR = "fields"


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        payload: dict[str, Any] = {
            "ts": datetime.fromtimestamp(record.created, UTC).isoformat(timespec="milliseconds"),
            "level": record.levelname.lower(),
            "service": SERVICE,
            "logger": record.name,
            "message": record.getMessage(),
        }
        payload.update(getattr(record, FIELDS_ATTR, None) or {})
        if record.exc_info:
            payload["error"] = self.formatException(record.exc_info)
        return json.dumps(payload, ensure_ascii=False, default=str)


def setup_logging(level: str = "info", json_logs: bool = True) -> None:
    handler = logging.StreamHandler()
    handler.setFormatter(
        JsonFormatter()
        if json_logs
        else logging.Formatter("%(asctime)s %(levelname)s ai %(name)s: %(message)s")
    )
    root = logging.getLogger()
    root.handlers = [handler]
    root.setLevel(level.upper())


def log_event(logger: logging.Logger, event: str, level: int = logging.INFO, **fields: Any) -> None:
    logger.log(level, event, extra={FIELDS_ATTR: {"event": event, **fields}})
