from pathlib import Path

import pytest

from app.config import Settings
from app.providers import MockProvider
from app.providers.base import LLMError
from evals.fake_core import DOC_IDS, DOCS, FakeCore, can_read
from evals.run import (
    Case,
    CaseResult,
    Observed,
    build_eval_provider,
    check,
    load_cases,
    main,
    render_report,
    run_case,
    run_golden,
)


async def test_golden_set_passes_in_mock_mode():
    cases = load_cases()
    results = await run_golden(cases, MockProvider())

    assert len(cases) >= 15
    assert {case.kind for case in cases} >= {"kb", "acl", "plan", "action", "unknown"}
    failed = {result.case.id: result.failures for result in results if not result.passed}
    assert failed == {}


def test_case_ids_are_unique():
    ids = [case.id for case in load_cases()]
    assert len(ids) == len(set(ids))


def test_check_reports_every_kind_of_failure():
    case = Case(
        id="x",
        question="q",
        tools=["search_knowledge"],
        args={"preview_absence_request": {"date_from": "2026-11-20"}},
        citations=["jira-access"],
        forbid_citations=["secrets-policy"],
        max_citations=0,
        contains=["SSO"],
        not_contains=["пароль"],
        not_called=["create_absence_request"],
    )
    observed = Observed(
        answer="Твой пароль",
        tools=[("create_absence_request", {})],
        citations=[DOC_IDS["secrets-policy"]],
        errors=["boom"],
    )

    failures = check(case, observed)

    assert len(failures) == 9
    assert "утечка: цитата secrets-policy" in failures
    assert "preview_absence_request не вызван" in failures


def test_check_wrong_argument():
    case = Case(id="x", question="q", args={"get_my_profile": {"a": 1}})
    observed = Observed(tools=[("get_my_profile", {"a": 2})])
    assert check(case, observed) == ["get_my_profile.a=2, ожидалось 1"]


def test_acl_rules_match_core():
    docs = {doc.slug: doc for doc in DOCS}
    intern = ["kb:read", "kb:read:team"]

    assert can_read(docs["jira-access"], intern, "search")
    assert not can_read(docs["vpn-setup"], intern, "search")
    assert can_read(docs["vacation-policy"], intern, "ads")
    assert not can_read(docs["secrets-policy"], intern, "search")
    assert can_read(docs["secrets-policy"], ["kb:read", "kb:read:hr"], "")
    assert not can_read(docs["vacation-policy"], [], "search")
    assert can_read(docs["benefits-2026"], ["*"], "")


def test_fake_core_search_is_ranked_and_marks_stale():
    hr = FakeCore(person="marina")
    [item] = hr.search("пароли", 5)
    assert item["stale"] is True
    assert FakeCore(person="sonya").search("деплой", 5)[0]["score"] == 0.85
    assert FakeCore().me()["team_name"] == "Search"


def test_fake_core_returns_chunks_like_core():
    [jira] = [item for item in FakeCore().search("доступ", 5) if item["title"].endswith("Jira")]
    assert jira["chunk"] == "3. Доступ приходит на рабочую почту в течение дня."
    assert FakeCore().search("пошаговая", 5)[0]["chunk"].startswith("1. Напишите")


def test_fake_core_document_endpoint_checks_acl():
    core = FakeCore()
    ok = core.route("GET", f"/api/v1/kb/documents/{DOC_IDS['jira-access']}", {}, {})
    assert ok.status_code == 200
    assert "SSO" in ok.json()["content"]
    assert ok.json()["owner"] == "petr@demo.local"
    hidden = core.route("GET", f"/api/v1/kb/documents/{DOC_IDS['vpn-setup']}", {}, {})
    assert hidden.status_code == 403
    assert core.route("GET", "/api/v1/kb/documents/doc-missing", {}, {}).status_code == 404


async def test_run_case_collects_errors_and_unknown_routes():
    class Broken:
        name = "broken"

        async def complete(self, messages, tools):
            raise LLMError("down", retryable=False)

    result = await run_case(Case(id="e", question="q"), Broken())
    assert not result.passed
    assert result.observed.errors

    core = FakeCore()
    assert core.route("GET", "/api/v1/unknown", {}, {}) is None


def test_render_report_summarises_results():
    ok = CaseResult(Case(id="a", question="Как | дела", kind="kb"), Observed(latency_ms=5), [])
    bad = CaseResult(
        Case(id="b", question="q", kind="acl"),
        Observed(tools=[("search_knowledge", {})], latency_ms=9),
        ["нет цитаты x"],
    )

    report = render_report([ok, bad], {"mode": "mock", "model": "mock", "prompt_version": "v1"})

    assert "**1/2** (50%)" in report
    assert "| kb | 1/1 |" in report
    assert "Как \\| дела" in report
    assert "❌ нет цитаты x" in report
    assert "p50 9 мс, max 9 мс" in report
    assert "**0/0** (0%)" in render_report([], {})


def test_main_writes_report_and_sets_exit_code(tmp_path: Path, capsys):
    cases = tmp_path / "cases.jsonl"
    cases.write_text(
        '{"id": "hello", "question": "Привет!", "tools": [], "contains": ["AI-buddy"]}\n\n'
        '{"id": "fails", "question": "Привет!", "contains": ["никогда"]}\n',
        encoding="utf-8",
    )
    report = tmp_path / "report.md"

    def factory(mode: str, settings: Settings) -> MockProvider:
        assert mode == "mock"
        return MockProvider()

    args = ["--mode", "mock", "--cases", str(cases), "--report", str(report)]
    assert main(args, provider_factory=factory) == 1
    assert main([*args, "--min-pass", "0.5"], provider_factory=factory) == 0
    assert "**1/2**" in report.read_text(encoding="utf-8")
    assert "FAIL fails" in capsys.readouterr().out


@pytest.mark.parametrize("mode", ["mock", "real"])
def test_build_eval_provider(mode: str):
    provider = build_eval_provider(mode, Settings(llm_api_key="k"))
    assert provider.name in {"mock", "openai"}
