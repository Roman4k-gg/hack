import logging

from fastapi import FastAPI

from app.api import chat, health
from app.config import get_settings

settings = get_settings()
logging.basicConfig(
    level=settings.log_level.upper(),
    format="%(asctime)s %(levelname)s ai %(name)s: %(message)s",
)

app = FastAPI(title="AI-buddy · ai", version="0.1.0")
app.include_router(health.router)
app.include_router(chat.router)
