import httpx
import pytest

from app.agent.tools import CoreClient, ToolRegistry, keywords, stem
from app.auth import User

JIRA = {
    "document_id": "55555555-5555-5555-5555-555555555501",
    "title": "Как получить доступ к Jira",
    "summary": "Инструкция",
    "url": "https://demo.local/docs/jira-access",
    "chunk": "1. Напишите наставнику в чат.",
    "updated_at": "2026-11-14T09:00:00Z",
    "stale": False,
    "score": 1,
}
VPN = {**JIRA, "document_id": "vpn", "title": "Настройка VPN", "score": 0.5}


def registry_for(responses: dict[str, list[dict]], seen: list[str] | None = None) -> ToolRegistry:
    def handler(request: httpx.Request) -> httpx.Response:
        query = request.url.params["q"]
        if seen is not None:
            seen.append(query)
        assert request.url.path == "/api/v1/kb/search"
        assert request.url.params["limit"] == "5"
        return httpx.Response(200, json={"items": responses.get(query, [])})

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return ToolRegistry(CoreClient("http://core.test", client=client))


def test_keywords_drop_stop_words_and_short_words():
    assert keywords("Как получить доступ к Jira?") == ["доступ", "jira"]
    assert keywords("что такое экватор и как к нему готовиться") == [
        "экватор",
        "нему",
        "готовитьс",
    ]
    assert keywords("a b c d e f g h", limit=2) == []
    assert keywords("Как устроены релизы и релиз?") == ["устроен", "релиз"]


def test_stem_cuts_russian_endings_only():
    assert stem("отпуска") == "отпуск"
    assert stem("правила") == "правил"
    assert stem("стажёра") == "стажёр"
    assert stem("дейлик") == "дейлик"
    assert stem("дней") == "дней"
    assert stem("jira") == "jira"
    assert stem("первый") == "перв"


async def test_search_ranks_documents_matching_more_keywords(intern: User):
    many = {**VPN, "document_id": "many", "score": 0.7}
    registry = registry_for({"отпуск": [many, JIRA], "дней": [many]})

    result = await registry.call("search_knowledge", {"query": "отпуска дней"}, intern)

    assert [item["document_id"] for item in result.data["items"]] == ["many", JIRA["document_id"]]


async def test_search_uses_full_query_first(intern: User):
    seen: list[str] = []
    registry = registry_for({"Jira": [JIRA]}, seen)

    result = await registry.call("search_knowledge", {"query": "Jira"}, intern)

    assert seen == ["Jira"]
    assert result.ok
    assert result.data["items"][0]["title"] == "Как получить доступ к Jira"
    assert result.citations == [
        {
            "id": JIRA["document_id"],
            "title": "Как получить доступ к Jira",
            "url": "https://demo.local/docs/jira-access",
        }
    ]


async def test_search_falls_back_to_keywords_and_merges(intern: User):
    seen: list[str] = []
    registry = registry_for({"доступ": [VPN, JIRA], "jira": [{**JIRA, "score": 0.7}]}, seen)

    result = await registry.call(
        "search_knowledge", {"query": "Как получить доступ к Jira?"}, intern
    )

    assert seen == ["Как получить доступ к Jira?", "доступ", "jira"]
    assert [item["document_id"] for item in result.data["items"]] == [JIRA["document_id"], "vpn"]
    assert result.data["items"][0]["score"] == 1.0


async def test_search_nothing_found(intern: User):
    result = await registry_for({}).call("search_knowledge", {"query": "экватор"}, intern)
    assert result.ok
    assert result.data == {"query": "экватор", "items": []}
    assert result.citations == []


@pytest.mark.parametrize("args", [{}, {"query": "  "}])
async def test_search_requires_query(intern: User, args: dict):
    result = await registry_for({}).call("search_knowledge", args, intern)
    assert result.error_code == "VALIDATION_ERROR"


async def test_search_requires_kb_scope():
    user = User(id="u", role="x", token="t", scopes=["chat:use"])
    registry = registry_for({})
    assert "search_knowledge" not in {tool.name for tool in registry.available(user)}


async def test_core_validation_error_is_mapped(intern: User):
    client = httpx.AsyncClient(transport=httpx.MockTransport(lambda r: httpx.Response(400)))
    registry = ToolRegistry(CoreClient("http://core.test", client=client))
    result = await registry.call("search_knowledge", {"query": "jira"}, intern)
    assert result.error_code == "VALIDATION_ERROR"
