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


CORE_CALLS = {
    "get_my_profile": ("GET", "/api/v1/me"),
    "get_onboarding_plan": ("GET", "/api/v1/onboarding/plan"),
    "search_knowledge": ("GET", "/api/v1/kb/search"),
    "preview_absence_request": ("POST", "/api/v1/requests/absence/preview"),
    "create_absence_request": ("POST", "/api/v1/requests/absence"),
    "get_my_requests": ("GET", "/api/v1/requests"),
    "preview_access_bundle": ("POST", "/api/v1/requests/access/preview"),
    "create_access_requests": ("POST", "/api/v1/requests/access"),
    "preview_handoff": None,
    "create_handoff": ("POST", "/api/v1/handoffs"),
}
ERROR_CODES = [
    "UNAUTHORIZED",
    "FORBIDDEN",
    "NOT_FOUND",
    "VALIDATION_ERROR",
    "CONFLICT",
    "TIMEOUT",
    "UPSTREAM_ERROR",
    "CONFIRMATION_REQUIRED",
    "INJECTION_SUSPECTED",
]


def tools_index(served: dict[str, Any]) -> dict[str, Any]:
    tools = []
    for tool in DEFAULT_TOOLS:
        annotations = served[tool.name].annotations
        call = CORE_CALLS[tool.name]
        tools.append(
            {
                "name": tool.name,
                "description": tool.spec.description,
                "scope": tool.scope or None,
                "readOnly": annotations.read_only_hint,
                "destructive": annotations.destructive_hint,
                "requiresConfirmation": tool.requires_confirmation,
                "inputSchema": served[tool.name].input_schema,
                "calls": {"method": call[0], "path": call[1]} if call else None,
            }
        )
    return {"version": "1.0", "generated_from": "ai/app", "tools": tools, "errors": ERROR_CODES}


async def build_contracts() -> dict[str, str]:
    served = await server_tools()
    contracts = {
        f"{tool.name}.json": render(contract(tool, served[tool.name])) for tool in DEFAULT_TOOLS
    }
    contracts["tools.json"] = render(tools_index(served))
    return contracts


def main(target: Path = MCP_DIR) -> int:
    target.mkdir(parents=True, exist_ok=True)
    for name, text in asyncio.run(build_contracts()).items():
        (target / name).write_text(text, encoding="utf-8")
    print(f"contracts written to {target}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
