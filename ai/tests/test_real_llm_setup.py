import json
from pathlib import Path

import httpx
import pytest

from app.agent.loop import Agent
from app.agent.tools import CoreClient, ToolRegistry
from app.auth import User
from app.config import Settings
from app.providers import MockProvider, OpenAICompatProvider, build_provider
from app.providers.base import LLMError, LLMResponse, ToolCall
from evals import ping
from evals.run import main as run_evals

SIGNATURE = {"extra_content": {"google": {"thought_signature": "sig-1"}}}
PLAN_CALL = {
    "id": "c1",
    "type": "function",
    "function": {"name": "get_onboarding_plan", "arguments": "{}"},
    **SIGNATURE,
}


def provider_with(handler, **kwargs) -> OpenAICompatProvider:
    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return OpenAICompatProvider("https://llm.test/v1", "k", "test-model", client=client, **kwargs)


def completion(message: dict) -> httpx.Response:
    return httpx.Response(
        200,
        json={
            "model": "test-model",
            "choices": [{"message": message}],
            "usage": {"prompt_tokens": 12, "completion_tokens": 3},
        },
    )


class GeminiLike:
    def __init__(self, final: str = "Получи доступ к Jira") -> None:
        self.final = final
        self.bodies: list[dict] = []

    def __call__(self, request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        self.bodies.append(body)
        if body["messages"][-1]["role"] == "user":
            return completion({"content": None, "tool_calls": [PLAN_CALL]})
        echoed = body["messages"][-2]["tool_calls"][0]
        if echoed.get("extra_content") != SIGNATURE["extra_content"]:
            error = {"message": "Function call is missing a thought_signature"}
            return httpx.Response(400, json=[{"error": error}])
        return completion({"content": self.final})


def test_tests_ignore_local_env_file(tmp_path: Path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    (tmp_path / ".env").write_text("LLM_MODE=real\nLLM_API_KEY=secret\n", encoding="utf-8")

    settings = Settings()

    assert settings.llm_mode == "mock"
    assert settings.llm_api_key == ""


async def test_provider_keeps_extra_fields_and_sends_options():
    seen = {}

    def handler(request: httpx.Request) -> httpx.Response:
        seen.update(json.loads(request.content))
        return completion({"content": None, "tool_calls": [PLAN_CALL]})

    provider = provider_with(handler, reasoning_effort="low", max_tokens=2000, temperature=1.0)
    response = await provider.complete([{"role": "user", "content": "?"}], [])

    assert response.tool_calls == [
        ToolCall(id="c1", name="get_onboarding_plan", arguments={}, extra=SIGNATURE)
    ]
    assert seen["reasoning_effort"] == "low"
    assert seen["max_tokens"] == 2000
    assert seen["temperature"] == 1.0


async def test_provider_omits_reasoning_effort_by_default():
    seen = {}

    def handler(request: httpx.Request) -> httpx.Response:
        seen.update(json.loads(request.content))
        return completion({"content": "ok"})

    await provider_with(handler).complete([{"role": "user", "content": "?"}], [])
    assert "reasoning_effort" not in seen
    assert seen["max_tokens"] == 800


@pytest.mark.parametrize(
    ("body", "expected"),
    [
        ([{"error": {"message": "User location is not supported"}}], ": User location is not"),
        ({"error": {"message": "bad\n  key"}}, "ошибку 400: bad key"),
        ({"error": "plain"}, "ошибку 400: plain"),
        ({"other": 1}, "ошибку 400"),
        ([], "ошибку 400"),
    ],
)
async def test_provider_error_includes_provider_message(body, expected: str):
    provider = provider_with(lambda request: httpx.Response(400, json=body))
    with pytest.raises(LLMError) as error:
        await provider.complete([{"role": "user", "content": "?"}], [])
    assert expected in str(error.value)
    assert not error.value.retryable


async def test_provider_error_with_text_body():
    provider = provider_with(lambda request: httpx.Response(503, text="down"))
    with pytest.raises(LLMError) as error:
        await provider.complete([{"role": "user", "content": "?"}], [])
    assert str(error.value) == "LLM вернула ошибку 503"
    assert error.value.retryable


def test_build_provider_passes_llm_options():
    settings = Settings(
        llm_mode="real", llm_max_tokens=1500, llm_temperature=1.0, llm_reasoning_effort="low"
    )
    provider = build_provider(settings)
    assert (provider.max_tokens, provider.temperature, provider.reasoning_effort) == (
        1500,
        1.0,
        "low",
    )


class TwoStepProvider:
    name = "two-step"

    def __init__(self) -> None:
        self.calls: list[list[dict]] = []

    async def complete(self, messages, tools):
        self.calls.append([dict(message) for message in messages])
        if len(self.calls) == 1:
            call = ToolCall(id="c1", name="get_onboarding_plan", arguments={}, extra=SIGNATURE)
            return LLMResponse(tool_calls=[call])
        return LLMResponse(text="Дальше — Jira")


async def test_agent_echoes_provider_extra_fields(intern: User):
    def core(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"next_step": {"title": "Jira"}})

    client = httpx.AsyncClient(transport=httpx.MockTransport(core))
    provider = TwoStepProvider()
    agent = Agent(provider, ToolRegistry(CoreClient("http://core.test", client=client)))

    events = [event async for event in agent.run(intern, "Что дальше?")]

    assert events[-1]["type"] == "done"
    assistant = next(item for item in provider.calls[1] if item.get("tool_calls"))
    assert assistant["tool_calls"][0]["extra_content"] == SIGNATURE["extra_content"]


def test_ping_full_round_trip_with_signature(capsys):
    core = GeminiLike()
    settings = Settings(llm_api_key="sk-secret-123", llm_base_url="https://llm.test/v1")

    assert ping.main([], settings, provider_with(core)) == 0

    out = capsys.readouterr().out
    assert "llm.test, модель gpt-4o-mini, ключ задан" in out
    assert "OK: модель test-model вызвала get_onboarding_plan и ответила по результату" in out
    assert "токены 24+6" in out
    assert "sk-secret-123" not in out
    assert [tool["function"]["name"] for tool in core.bodies[0]["tools"]] == ["get_onboarding_plan"]
    tool_message = core.bodies[1]["messages"][-1]
    assert tool_message["tool_call_id"] == "c1"
    assert "Получить доступ к Jira" in tool_message["content"]


def without_extra(text, calls):
    return {
        "role": "assistant",
        "content": None,
        "tool_calls": [{"id": call_id, "type": "function"} for call_id, _ in calls],
    }


def test_ping_explains_lost_signature(capsys, monkeypatch):
    monkeypatch.setattr(ping, "assistant_tool_message", without_extra)

    assert ping.main([], Settings(llm_api_key="k"), provider_with(GeminiLike())) == 1

    out = capsys.readouterr().out
    assert "FAIL: ответ по результату инструмента: LLM вернула ошибку 400" in out
    assert "проверь extra в ToolCall" in out


def test_ping_fails_on_empty_final_answer(capsys):
    assert ping.main([], Settings(llm_api_key="k"), provider_with(GeminiLike(final=""))) == 1
    assert "увеличь LLM_MAX_TOKENS" in capsys.readouterr().out


def test_ping_fails_when_model_ignores_tools(capsys):
    def handler(request: httpx.Request) -> httpx.Response:
        return completion({"content": "Сначала получи доступы"})

    assert ping.main([], Settings(llm_api_key="k"), provider_with(handler)) == 1
    assert "без вызова инструмента: 'Сначала получи доступы'" in capsys.readouterr().out


@pytest.mark.parametrize(
    ("status", "body", "hint"),
    [
        (401, {}, "проверь LLM_API_KEY"),
        (404, {}, "проверь LLM_BASE_URL"),
        (400, [{"error": {"message": "User location is not supported"}}], "нужен VPN"),
        (400, {"error": {"message": "API key not valid."}}, "проверь LLM_API_KEY"),
        (500, {}, ""),
    ],
)
def test_ping_explains_http_errors(capsys, status: int, body, hint: str):
    provider = provider_with(lambda request: httpx.Response(status, json=body))

    assert ping.main([], Settings(llm_api_key="k"), provider) == 1

    out = capsys.readouterr().out
    assert f"FAIL: первый запрос: LLM вернула ошибку {status}" in out
    assert hint in out


def test_ping_without_key_does_not_touch_network(capsys):
    assert ping.main([], Settings(llm_base_url="no-scheme")) == 1
    out = capsys.readouterr().out
    assert "no-scheme, модель gpt-4o-mini, ключ не задан" in out
    assert "FAIL: первый запрос: LLM_API_KEY не задан" in out


def models_client(handler) -> httpx.Client:
    return httpx.Client(transport=httpx.MockTransport(handler))


def test_ping_lists_models(capsys):
    seen = {}

    def handler(request: httpx.Request) -> httpx.Response:
        seen["url"] = str(request.url)
        seen["auth"] = request.headers["Authorization"]
        data = [{"id": "models/gemini-flash"}, {"id": "gemini-alpha"}]
        return httpx.Response(200, json={"data": data})

    settings = Settings(llm_api_key="k", llm_base_url="https://llm.test/v1/")
    assert ping.main(["--models"], settings, client=models_client(handler)) == 0

    assert capsys.readouterr().out.split() == ["gemini-alpha", "gemini-flash"]
    assert seen == {"url": "https://llm.test/v1/models", "auth": "Bearer k"}


@pytest.mark.parametrize(
    ("handler", "text"),
    [
        (lambda request: httpx.Response(403), "FAIL: список моделей вернул 403"),
        (lambda request: httpx.Response(200, json={"data": []}), ""),
    ],
)
def test_ping_models_failures(capsys, handler, text: str):
    assert ping.main(["--models"], Settings(llm_api_key="k"), client=models_client(handler)) == 1
    assert text in capsys.readouterr().out


def test_ping_models_network_error(capsys):
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("down", request=request)

    assert ping.main(["--models"], Settings(llm_api_key="k"), client=models_client(handler)) == 1
    assert "FAIL: список моделей недоступен: ConnectError" in capsys.readouterr().out


def test_evals_default_to_mock_and_real_report_is_separate(tmp_path: Path, monkeypatch):
    cases = tmp_path / "cases.jsonl"
    cases.write_text(
        json.dumps({"id": "hi", "kind": "chat", "question": "Привет", "tools": []}) + "\n",
        encoding="utf-8",
    )
    modes = []

    def factory(mode: str, settings: Settings) -> MockProvider:
        modes.append(mode)
        return MockProvider()

    monkeypatch.setattr("evals.run.SUITES", {"golden": (cases, tmp_path / "report.md")})

    assert run_evals([], provider_factory=factory) == 0
    assert run_evals(["--mode", "real"], provider_factory=factory) == 0

    assert modes == ["mock", "real"]
    assert (tmp_path / "report.md").is_file()
    assert "Режим LLM: `real`" in (tmp_path / "report_real.md").read_text(encoding="utf-8")


@pytest.mark.parametrize(
    ("raw", "expected"),
    [({"a": 1}, {"a": 1}), ("", {}), (None, {}), ('{"b": 2}', {"b": 2}), ("[1]", {})],
)
async def test_provider_parses_arguments_in_any_shape(raw, expected: dict):
    call = {"id": "c", "function": {"name": "get_my_profile", "arguments": raw}}
    provider = provider_with(lambda request: completion({"tool_calls": [call]}))
    response = await provider.complete([{"role": "user", "content": "?"}], [])
    assert response.tool_calls[0].arguments == expected


def test_ping_models_without_key(capsys):
    def handler(request: httpx.Request) -> httpx.Response:
        raise AssertionError("no request without key")

    assert ping.main(["--models"], Settings(), client=models_client(handler)) == 1
    assert "FAIL: LLM_API_KEY не задан" in capsys.readouterr().out


class SearchThenAnswer:
    name = "search-then-answer"

    def __init__(self, answer: str) -> None:
        self.answer = answer
        self.turn = 0

    async def complete(self, messages, tools):
        self.turn += 1
        if self.turn == 1:
            call = ToolCall(id="s1", name="search_knowledge", arguments={"query": "секреты"})
            return LLMResponse(tool_calls=[call])
        return LLMResponse(text=self.answer)


def stale_core(stale: bool):
    def handler(request: httpx.Request) -> httpx.Response:
        item = {"document_id": "d1", "title": "Секреты", "url": "u", "stale": stale, "score": 1}
        return httpx.Response(200, json={"items": [item]})

    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return ToolRegistry(CoreClient("http://core.test", client=client))


async def answer_with(intern: User, answer: str, stale: bool) -> tuple[str, list[dict]]:
    agent = Agent(SearchThenAnswer(answer), stale_core(stale))
    events = [event async for event in agent.run(intern, "Где хранить секреты?")]
    text = "".join(event["text"] for event in events if event["type"] == "delta")
    return text, [event["citation"] for event in events if event["type"] == "citation"]


async def test_stale_warning_added_by_agent(intern: User):
    text, citations = await answer_with(intern, "Храни в менеджере секретов.", stale=True)
    assert text == (
        "Храни в менеджере секретов.\n\n"
        "⚠️ Документ давно не обновлялся — информация может быть устаревшей."
    )
    assert [item["id"] for item in citations] == ["d1"]


async def test_stale_warning_not_duplicated_or_added_to_fresh_docs(intern: User):
    own = "Храни в менеджере секретов. Документ может быть устаревшим."
    assert (await answer_with(intern, own, stale=True))[0] == own
    fresh = "Храни в менеджере секретов."
    assert (await answer_with(intern, fresh, stale=False))[0] == fresh


def test_eval_check_allows_repeated_search():
    from evals.run import Case, Observed, check

    case = Case(id="c", question="?", tools=["search_knowledge", "preview_handoff"])
    repeated = Observed(
        tools=[("search_knowledge", {}), ("search_knowledge", {}), ("preview_handoff", {})]
    )
    assert check(case, repeated) == []
    skipped = Observed(tools=[("search_knowledge", {}), ("search_knowledge", {})])
    assert check(case, skipped) == [
        "инструменты ['search_knowledge', 'search_knowledge'], ожидались "
        "['search_knowledge', 'preview_handoff']"
    ]


async def test_paced_provider_spaces_llm_calls():
    from evals.run import PacedProvider

    now = [100.0]
    waits: list[float] = []

    async def sleep(seconds: float) -> None:
        waits.append(seconds)
        now[0] += seconds

    paced = PacedProvider(MockProvider(), rpm=30, clock=lambda: now[0], sleep=sleep)
    messages = [{"role": "user", "content": "Привет"}]

    await paced.complete(messages, [])
    now[0] += 0.5
    await paced.complete(messages, [])
    now[0] += 5
    await paced.complete(messages, [])

    assert paced.name == "mock"
    assert waits == [1.5]


def test_evals_only_runs_selected_cases_with_rpm(tmp_path: Path, monkeypatch):
    cases = tmp_path / "cases.jsonl"
    lines = [{"id": key, "kind": "chat", "question": "Привет", "tools": []} for key in "abc"]
    cases.write_text("".join(json.dumps(line) + "\n" for line in lines), encoding="utf-8")
    report = tmp_path / "report.md"

    args = ["--cases", str(cases), "--report", str(report), "--only", "a,c", "--rpm", "6000"]
    assert run_evals(args, provider_factory=lambda mode, settings: MockProvider()) == 0

    text = report.read_text(encoding="utf-8")
    assert "`a`" in text and "`c`" in text and "`b`" not in text
