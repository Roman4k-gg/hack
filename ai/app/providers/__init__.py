from app.config import Settings
from app.providers.base import LLMError, LLMProvider, LLMResponse, ToolCall, ToolSpec
from app.providers.mock import MockProvider
from app.providers.openai_compat import OpenAICompatProvider


def build_provider(settings: Settings) -> LLMProvider:
    if settings.llm_mode == "real":
        return OpenAICompatProvider(
            base_url=settings.llm_base_url,
            api_key=settings.llm_api_key,
            model=settings.llm_model,
            timeout=settings.llm_timeout,
        )
    return MockProvider()


__all__ = [
    "LLMError",
    "LLMProvider",
    "LLMResponse",
    "MockProvider",
    "OpenAICompatProvider",
    "ToolCall",
    "ToolSpec",
    "build_provider",
]
