import re
from pathlib import Path

from app.agent.prompts import PROMPTS_DIR
from app.agent.tools import DEFAULT_TOOLS
from app.config import Settings
from evals.export_contracts import MCP_DIR, SPEC_DIR, build_contracts, main

AI_DIR = Path(__file__).resolve().parents[1]
SPEC = SPEC_DIR / "spec.md"


def spec_text() -> str:
    return SPEC.read_text(encoding="utf-8")


def defined_tests() -> set[str]:
    found = set()
    for path in (AI_DIR / "tests").glob("test_*.py"):
        for name in re.findall(r"^(?:async )?def (test_\w+)", path.read_text("utf-8"), re.M):
            found.add(f"tests/{path.name}::{name}")
    return found


def test_spec_kit_files_exist():
    for name in (
        "spec.md",
        "plan.md",
        "tasks.md",
        "research.md",
        "data-model.md",
        "quickstart.md",
        "contracts/rest.md",
        "contracts/sse.md",
        "prompts/system.md",
        "prompts/tools.md",
        "prompts/policy.md",
    ):
        assert (SPEC_DIR / name).is_file(), name


def test_tests_referenced_in_spec_exist():
    referenced = set(re.findall(r"tests/test_\w+\.py::test_\w+", spec_text()))
    assert len(referenced) >= 30
    assert referenced - defined_tests() == set()


def test_every_requirement_is_traced_to_tests():
    text = spec_text()
    requirements = set(re.findall(r"^\| (FR-\d+) \| (?!`tests/)", text, re.M))
    traced = set(re.findall(r"^\| (FR-\d+) \| `tests/", text, re.M))
    assert len(requirements) >= 16
    assert requirements == traced


def test_every_tool_is_in_access_matrix():
    text = spec_text()
    for tool in DEFAULT_TOOLS:
        assert f"| `{tool.name}` |" in text
        if tool.scope:
            assert f"`{tool.scope}`" in text


def test_spec_prompt_matches_working_prompt():
    version = Settings().prompt_version
    working = (PROMPTS_DIR / version / "system.md").read_text(encoding="utf-8")
    assert (SPEC_DIR / "prompts" / "system.md").read_text(encoding="utf-8") == working


async def test_mcp_contracts_match_code():
    expected = await build_contracts()
    on_disk = {path.name: path.read_text(encoding="utf-8") for path in MCP_DIR.glob("*.json")}
    assert on_disk == expected


def test_export_contracts_writes_files(tmp_path: Path, capsys):
    assert main(tmp_path / "mcp") == 0
    assert len(list((tmp_path / "mcp").glob("*.json"))) == len(DEFAULT_TOOLS)
    assert "contracts written" in capsys.readouterr().out
