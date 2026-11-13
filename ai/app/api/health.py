from fastapi import APIRouter

from app.config import get_settings

router = APIRouter()


@router.get("/healthz")
async def healthz() -> dict:
    return {"status": "ok"}


@router.get("/readyz")
async def readyz() -> dict:
    settings = get_settings()
    return {
        "status": "ok",
        "llm_mode": settings.llm_mode,
        "prompt_version": settings.prompt_version,
    }
