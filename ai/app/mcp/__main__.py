import uvicorn

from app.config import get_settings
from app.mcp.server import build_app
from app.observability import setup_logging


def main() -> None:
    settings = get_settings()
    setup_logging(settings.log_level, settings.log_json)
    uvicorn.run(
        build_app(),
        host=settings.mcp_host,
        port=settings.mcp_port,
        log_level=settings.log_level.lower(),
    )


if __name__ == "__main__":
    main()
