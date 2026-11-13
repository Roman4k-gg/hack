from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    postgres_dsn: str = "postgres://buddy:buddy@localhost:5432/buddy?sslmode=disable"
    core_url: str = "http://localhost:8081"
    jwt_secret: str = "dev-secret-change-me"
    jwt_issuer: str = "ai-buddy-core"

    llm_mode: str = "mock"
    llm_model: str = "gpt-4o-mini"
    llm_api_key: str = ""
    llm_base_url: str = "https://api.openai.com/v1"
    llm_timeout: float = 30.0

    max_tool_iterations: int = 5
    tool_timeout: float = 5.0
    mock_stream_delay: float = 0.03
    prompt_version: str = "v1"
    log_level: str = "info"


@lru_cache
def get_settings() -> Settings:
    return Settings()
