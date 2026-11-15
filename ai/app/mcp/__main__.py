import uvicorn

from app.config import get_settings
from app.mcp.server import build_app


def main() -> None:
    settings = get_settings()
    uvicorn.run(
        build_app(),
        host=settings.mcp_host,
        port=settings.mcp_port,
        log_level=settings.log_level.lower(),
    )


if __name__ == "__main__":
    main()
