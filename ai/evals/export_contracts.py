import asyncio
import json
import sys
from pathlib import Path
from typing import Any

from mcp.client.client import Client

from app.agent.tools import DEFAULT_TOOLS, CoreClient, Tool, ToolRegistry
from app.mcp.server import build_server

SPEC_DIR = Path(__file__).resolve().parents[2] / "specs" / "001-ai-buddy"
MCP_DIR = SPEC_DIR / "contracts" / "mcp"


def tool_kind(tool: Tool) -> str:
    if tool.requires_confirmation:
        return "write+confirmation"
    if tool.confirmed_by:
        return "preview"
    return "read"


async def server_tools() -> dict[str, Any]:
    registry = ToolRegistry(CoreClient("http://core.invalid"))
    async with Client(build_server(registry)) as client:
        listed = await client.list_tools()
    return {tool.name: tool for tool in listed.tools}


def contract(tool: Tool, served: Any) -> dict[str, Any]:
    annotations = served.annotations
    return {
        "name": tool.name,
        "description": tool.spec.description,
        "kind": tool_kind(tool),
        "scope": tool.scope or None,
        "confirmed_by": tool.confirmed_by or None,
        "requires_confirmation": tool.requires_confirmation,
        "annotations": {
            "read_only": annotations.read_only_hint,
            "idempotent": annotations.idempotent_hint,
        },
        "llm_parameters": tool.spec.parameters,
        "mcp_input_schema": served.input_schema,
    }


def render(data: dict[str, Any]) -> str:
    return json.dumps(data, ensure_ascii=False, indent=2) + "\n"


async def build_contracts() -> dict[str, str]:
    served = await server_tools()
    return {
        f"{tool.name}.json": render(contract(tool, served[tool.name])) for tool in DEFAULT_TOOLS
    }


def main(target: Path = MCP_DIR) -> int:
    target.mkdir(parents=True, exist_ok=True)
    for name, text in asyncio.run(build_contracts()).items():
        (target / name).write_text(text, encoding="utf-8")
    print(f"contracts written to {target}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
