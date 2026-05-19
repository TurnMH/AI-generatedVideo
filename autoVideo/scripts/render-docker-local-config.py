#!/usr/bin/env python3
"""Render a production override config from config.local.yaml.

The docker compose stack mounts config.docker.local.yaml into every Go service as
AUTOVIDEO_CONFIG_OVERRIDE_FILE. This helper extracts only the runtime LLM-related
sections that must not fall back to empty defaults in docker deployments.
"""

from __future__ import annotations

import argparse
import copy
import sys
from pathlib import Path

import yaml


REQUIRED_PATHS = [
    ("image-service", "models", "openai_base"),
    ("image-service", "models", "openai_keys"),
    ("project-service", "llm", "base_url"),
    ("project-service", "llm", "api_key"),
    ("project-service", "llm", "model"),
    ("character-service", "llm", "base_url"),
    ("character-service", "llm", "api_key"),
    ("character-service", "llm", "model"),
    ("script-service", "llm", "openai", "base_url"),
    ("script-service", "llm", "openai", "api_key"),
    ("script-service", "llm", "openai", "model"),
]


def read_yaml(path: Path) -> dict:
    try:
        with path.open("r", encoding="utf-8") as handle:
            data = yaml.safe_load(handle)
    except FileNotFoundError:
        raise SystemExit(f"source config not found: {path}")
    except yaml.YAMLError as exc:
        raise SystemExit(f"failed to parse yaml from {path}: {exc}")
    if data is None:
        return {}
    if not isinstance(data, dict):
        raise SystemExit(f"unexpected yaml root in {path}: expected mapping")
    return data


def get_nested(data: dict, path: tuple[str, ...]):
    node = data
    for part in path:
        if not isinstance(node, dict) or part not in node:
            return None
        node = node[part]
    return node


def pick_sections(source: dict) -> dict:
    override: dict[str, dict] = {}

    selection_map: dict[str, tuple[str, ...]] = {
        "project-service": ("llm", "concurrency"),
        "script-service": ("llm",),
        "character-service": ("llm", "gemini", "claude", "qwen", "zhipu", "concurrency"),
        "image-service": ("models",),
    }

    for service_name, keys in selection_map.items():
        service_source = source.get(service_name)
        if not isinstance(service_source, dict):
            continue

        service_override: dict[str, object] = {}
        for key in keys:
            value = service_source.get(key)
            if value is not None:
                service_override[key] = copy.deepcopy(value)

        if service_override:
            override[service_name] = service_override

    root_llm: dict[str, object] = {}
    project_llm = source.get("project-service", {}).get("llm") if isinstance(source.get("project-service"), dict) else None
    script_llm = source.get("script-service", {}).get("llm") if isinstance(source.get("script-service"), dict) else None
    character_llm = source.get("character-service", {}).get("llm") if isinstance(source.get("character-service"), dict) else None

    if isinstance(script_llm, dict):
        for key in ("provider",):
            value = script_llm.get(key)
            if value is not None:
                root_llm[key] = copy.deepcopy(value)
        for key in ("openai", "claude", "qwen", "zhipu"):
            value = script_llm.get(key)
            if value is not None:
                root_llm[key] = copy.deepcopy(value)

    if isinstance(project_llm, dict):
        for key in ("base_url", "api_key", "model", "timeout", "fallback_base_url", "fallback_api_key", "fallback_model"):
            value = project_llm.get(key)
            if value is not None:
                root_llm[key] = copy.deepcopy(value)

    if isinstance(character_llm, dict):
        for key in ("base_url", "api_key", "model", "vision_model", "timeout"):
            value = character_llm.get(key)
            if value is not None:
                root_llm[key] = copy.deepcopy(value)

    if root_llm:
        override["llm"] = root_llm

    character_concurrency = source.get("character-service", {}).get("concurrency") if isinstance(source.get("character-service"), dict) else None
    if isinstance(character_concurrency, dict) and character_concurrency:
        override["concurrency"] = copy.deepcopy(character_concurrency)

    character_image = source.get("character-service", {}).get("image") if isinstance(source.get("character-service"), dict) else None
    default_model = character_image.get("default_model") if isinstance(character_image, dict) else None
    if isinstance(default_model, str) and default_model.strip():
        service_override = override.setdefault("character-service", {})
        image_override = service_override.setdefault("image", {})
        image_override["default_model"] = default_model.strip()

    return override


def validate_required_values(source: dict) -> None:
    missing: list[str] = []
    for path in REQUIRED_PATHS:
        value = get_nested(source, path)
        if value is None:
            missing.append(".".join(path))
            continue
        if isinstance(value, str) and not value.strip():
            missing.append(".".join(path))
            continue
        if isinstance(value, (list, tuple, dict)) and len(value) == 0:
            missing.append(".".join(path))
    if missing:
        joined = ", ".join(missing)
        raise SystemExit(f"missing required runtime LLM values in config.local.yaml: {joined}")


def main() -> int:
    parser = argparse.ArgumentParser(description="Render config.docker.local.yaml from config.local.yaml")
    parser.add_argument("--source", type=Path, default=Path(__file__).resolve().parents[1] / "config.local.yaml")
    parser.add_argument("--output", type=Path, default=Path(__file__).resolve().parents[1] / "config.docker.local.yaml")
    args = parser.parse_args()

    source = read_yaml(args.source)
    validate_required_values(source)
    override = pick_sections(source)

    if not override:
        raise SystemExit("no override sections were selected from config.local.yaml")

    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", encoding="utf-8") as handle:
        yaml.safe_dump(override, handle, sort_keys=False, allow_unicode=True, default_flow_style=False)

    print(f"rendered {args.output} from {args.source}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())