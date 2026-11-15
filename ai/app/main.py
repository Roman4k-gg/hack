from fastapi import FastAPI

from app.api import chat, health
from app.config import get_settings
from app.observability import setup_logging

settings = get_settings()
setup_logging(settings.log_level, settings.log_json)

app = FastAPI(title="AI-buddy · ai", version="0.1.0")
app.include_router(health.router)
app.include_router(chat.router)
