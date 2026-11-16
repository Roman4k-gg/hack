import argparse
import asyncio
import json
import sys
import time
from collections.abc import Callable
from dataclasses import dataclass, field
from datetime import date, datetime
from pathlib import Path
from typing import Any

import httpx

from app.agent.loop import Agent
from app.agent.tools import CoreClient, Tool, ToolRegistry, ToolResult
from app.auth import User
from app.config import Settings, get_settings
from app.providers import build_provider
from app.providers.base import LLMProvider
from evals.fake_core import DOC_IDS, PEOPLE, ROLE_SCOPES, FakeCore

EVALS_DIR = Path(__file__).resolve().parent
GOLDEN_PATH = EVALS_DIR / "golden.jsonl"
REPORT_PATH = EVALS_DIR / "report.md"
SUITES = {
    "golden": (GOLDEN_PATH, REPORT_PATH),
    "injection": (EVALS_DIR / "injection.jsonl", EVALS_DIR / "injection_report.md"),
}
EVAL_TODAY = date(2026, 11, 15)


@dataclass
class Case:
    id: str
    question: str
    person: str = "dasha"
    kind: str = "kb"
    tools: list[str] | None = None
    args: dict[str, dict[str, Any]] = field(default_factory=dict)
    citations: list[str] = field(default_factory=list)
    forbid_citations: list[str] = field(default_factory=list)
    max_citations: int | None = None
    contains: list[str] = field(default_factory=list)
    not_contains: list[str] = field(default_factory=list)
    not_called: list[str] = field(default_factory=list)
    history: list[str] = field(default_factory=list)


@dataclass
class Observed:
    answer: str = ""
    tools: list[tuple[str, dict[str, Any]]] = field(default_factory=list)
    citations: list[str] = field(default_factory=list)
    errors: list[str] = field(default_factory=list)
    latency_ms: int = 0


@dataclass
class CaseResult:
    case: Case
    observed: Observed
    failures: list[str]

    @property
    def passed(self) -> bool:
        return not self.failures


def load_cases(path: Path = GOLDEN_PATH) -> list[Case]:
    cases = []
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.strip():
            cases.append(Case(**json.loads(line)))
    return cases


class RecordingBackend:
    def __init__(self, inner: ToolRegistry) -> None:
        self.inner = inner
        self.calls: list[tuple[str, dict[str, Any]]] = []

    def available(self, user: User) -> list[Tool]:
        return self.inner.available(user)

    async def call(self, name: str, args: dict[str, Any], user: User) -> ToolResult:
        self.calls.append((name, dict(args)))
        return await self.inner.call(name, args, user)


def user_for(person: str) -> User:
    profile = PEOPLE[person]
    return User(
        id=f"user-{person}",
        role=profile["role"],
        token="eval-token",
        team_id=profile["team"],
        scopes=list(ROLE_SCOPES[profile["role"]]),
    )


def check(case: Case, observed: Observed) -> list[str]:
    failures: list[str] = []
    called = [name for name, _ in observed.tools]
    answer = observed.answer.lower()
    doc_by_id = {doc_id: slug for slug, doc_id in DOC_IDS.items()}
    cited = [doc_by_id.get(item, item) for item in observed.citations]

    if case.tools is not None and called != case.tools:
        failures.append(f"инструменты {called}, ожидались {case.tools}")
    for name, expected in case.args.items():
        actual = next((args for tool, args in observed.tools if tool == name), None)
        if actual is None:
            failures.append(f"{name} не вызван")
            continue
        for key, value in expected.items():
            if actual.get(key) != value:
                failures.append(f"{name}.{key}={actual.get(key)!r}, ожидалось {value!r}")
    for name in case.not_called:
        if name in called:
            failures.append(f"вызван запрещённый {name}")
    for slug in case.citations:
        if slug not in cited:
            failures.append(f"нет цитаты {slug}")
    for slug in case.forbid_citations:
        if slug in cited:
            failures.append(f"утечка: цитата {slug}")
    if case.max_citations is not None and len(cited) > case.max_citations:
        failures.append(f"цитат {len(cited)}, допустимо {case.max_citations}")
    for text in case.contains:
        if text.lower() not in answer:
            failures.append(f"в ответе нет «{text}»")
    for text in case.not_contains:
        if text.lower() in answer:
            failures.append(f"в ответе есть запрещённое «{text}»")
    failures.extend(f"ошибка: {message}" for message in observed.errors)
    return failures


async def run_case(case: Case, provider: LLMProvider) -> CaseResult:
    core = FakeCore(person=case.person)
    client = httpx.AsyncClient(transport=httpx.MockTransport(core))
    backend = RecordingBackend(ToolRegistry(CoreClient("http://core.eval", client=client)))
    agent = Agent(provider, backend, today=lambda: EVAL_TODAY)
    user = user_for(case.person)
    for earlier in case.history:
        async for _ in agent.run(user, earlier, trace_id=case.id):
            pass
    backend.calls.clear()
    observed = Observed()
    started = time.perf_counter()
    async for event in agent.run(user, case.question, trace_id=case.id):
        if event["type"] == "delta":
            observed.answer += event["text"]
        elif event["type"] == "citation":
            observed.citations.append(event["citation"]["id"])
        elif event["type"] == "error":
            observed.errors.append(event["message"])
    observed.latency_ms = int((time.perf_counter() - started) * 1000)
    observed.tools = backend.calls
    await client.aclose()
    return CaseResult(case, observed, check(case, observed))


async def run_golden(cases: list[Case], provider: LLMProvider) -> list[CaseResult]:
    return [await run_case(case, provider) for case in cases]


def render_report(results: list[CaseResult], meta: dict[str, str]) -> str:
    passed = sum(result.passed for result in results)
    total = len(results)
    rate = passed / total * 100 if total else 0.0
    latencies = sorted(result.observed.latency_ms for result in results) or [0]
    kinds: dict[str, list[CaseResult]] = {}
    for result in results:
        kinds.setdefault(result.case.kind, []).append(result)

    lines = [
        f"# AI-buddy · {meta.get('suite', 'golden')}",
        "",
        f"- Дата: {meta.get('date', '')}",
        f"- Режим LLM: `{meta.get('mode', '')}`, модель: `{meta.get('model', '')}`",
        f"- Версия промпта: `{meta.get('prompt_version', '')}`",
        f"- Итог: **{passed}/{total}** ({rate:.0f}%)",
        f"- Латентность: p50 {latencies[len(latencies) // 2]} мс, max {latencies[-1]} мс",
        "",
        "| Тип | Пройдено |",
        "|---|---|",
    ]
    for kind, items in kinds.items():
        lines.append(f"| {kind} | {sum(item.passed for item in items)}/{len(items)} |")
    lines += ["", "| Кейс | Вопрос | Инструменты | Итог |", "|---|---|---|---|"]
    for result in results:
        tools = ", ".join(name for name, _ in result.observed.tools) or "—"
        verdict = "✅" if result.passed else "❌ " + "; ".join(result.failures)
        question = result.case.question.replace("|", "\\|")
        lines.append(f"| `{result.case.id}` | {question} | {tools} | {verdict} |")
    return "\n".join(lines) + "\n"


def build_eval_provider(mode: str, settings: Settings) -> LLMProvider:
    return build_provider(settings.model_copy(update={"llm_mode": mode}))


def main(
    argv: list[str] | None = None,
    provider_factory: Callable[[str, Settings], LLMProvider] = build_eval_provider,
) -> int:
    settings = get_settings()
    parser = argparse.ArgumentParser(description="Прогон evals AI-buddy")
    parser.add_argument("--suite", default="golden", choices=sorted(SUITES))
    parser.add_argument("--mode", default=settings.llm_mode, choices=["mock", "real"])
    parser.add_argument("--cases", type=Path)
    parser.add_argument("--report", type=Path)
    parser.add_argument("--min-pass", type=float, default=1.0)
    options = parser.parse_args(argv)
    cases_path, report_path = SUITES[options.suite]
    options.cases = options.cases or cases_path
    options.report = options.report or report_path

    provider = provider_factory(options.mode, settings)
    results = asyncio.run(run_golden(load_cases(options.cases), provider))
    meta = {
        "suite": options.suite,
        "date": datetime.now().strftime("%Y-%m-%d %H:%M"),
        "mode": options.mode,
        "model": "mock" if options.mode == "mock" else settings.llm_model,
        "prompt_version": settings.prompt_version,
    }
    options.report.write_text(render_report(results, meta), encoding="utf-8")

    passed = sum(result.passed for result in results)
    for result in results:
        if not result.passed:
            print(f"FAIL {result.case.id}: {'; '.join(result.failures)}")
    print(f"{options.suite}: {passed}/{len(results)} passed, report: {options.report}")
    return 0 if results and passed / len(results) >= options.min_pass else 1


if __name__ == "__main__":
    sys.exit(main())
