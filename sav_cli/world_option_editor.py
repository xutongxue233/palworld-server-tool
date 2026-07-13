from __future__ import annotations

import base64
import hashlib
import json
import math
import re
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


GAME_VERSION = "1.0.0"
PAL_CONF_COMMIT = "a0f75513a99684922b0ad58692304f8ddfcc06d3"
WORLD_OPTION_CLASS = "/Script/Pal.PalWorldOptionSaveGame"
WORLD_OPTION_METADATA_ENTRIES = 109
WORLD_OPTION_METADATA_SHA256 = (
    "81bb88b68cc6427ed94ea9e29fafa51b443202db514e627eb7163d0674222f34"
)
WORLD_OPTION_TEMPLATE_SHA256 = (
    "61306e3043cba19cbda1be0a11c37c7a8a0270a69762ae223c6bfa50ab18068e"
)
WORLD_OPTION_TEMPLATE_BASE64 = (
    "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"
    "HLTG9A"
)
INTEGER_RE = re.compile(r"^[+-]?\d+$")
FLOAT_RE = re.compile(
    r"^[+-]?(?:\d+\.\d*|\d*\.\d+|\d+)(?:[eE][+-]?\d+)?$"
)
ENUM_VALUE_RE = re.compile(r"^[A-Za-z0-9_]+$")
ENUM_ALLOWED_VALUES = {
    "AllowConnectPlatform": frozenset({"Steam", "Xbox", "PS5", "Mac"}),
    "CrossplayPlatforms": frozenset({"Steam", "Xbox", "PS5", "Mac"}),
    "DeathPenalty": frozenset({"None", "Item", "ItemAndEquipment", "All"}),
    "Difficulty": frozenset({"None"}),
    "LogFormatType": frozenset({"Text", "Json"}),
    "RandomizerType": frozenset({"None", "Region", "All"}),
}


class WorldOptionEditError(ValueError):
    pass


@dataclass(frozen=True)
class WorldOptionSyncResult:
    updated_keys: tuple[str, ...]
    skipped_keys: tuple[str, ...]
    settings_digest: str
    expected_properties: dict[str, dict[str, Any]] = field(
        repr=False, compare=False
    )
    preserved_digest: str = field(repr=False, compare=False)

    def to_dict(self) -> dict[str, Any]:
        return {
            "game_version": GAME_VERSION,
            "updated_keys": list(self.updated_keys),
            "skipped_keys": list(self.skipped_keys),
            "settings_digest": self.settings_digest,
        }


def default_world_option_template() -> bytes:
    try:
        content = base64.b64decode(WORLD_OPTION_TEMPLATE_BASE64, validate=True)
    except (ValueError, TypeError) as exc:
        raise WorldOptionEditError("Bundled WorldOption.sav template is invalid") from exc
    digest = hashlib.sha256(content).hexdigest()
    if digest != WORLD_OPTION_TEMPLATE_SHA256:
        raise WorldOptionEditError("Bundled WorldOption.sav template checksum mismatch")
    return content


def load_world_option_metadata(path: str | Path) -> dict[str, dict[str, str]]:
    try:
        content = Path(path).read_bytes()
        if hashlib.sha256(content).hexdigest() != WORLD_OPTION_METADATA_SHA256:
            raise WorldOptionEditError("WorldOption metadata checksum mismatch")
        payload = json.loads(content.decode("utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise WorldOptionEditError(f"Unable to load WorldOption metadata: {exc}") from exc
    if (
        not isinstance(payload, dict)
        or payload.get("schema") != 1
        or payload.get("game_version") != GAME_VERSION
        or payload.get("source_commit") != PAL_CONF_COMMIT
        or not isinstance(payload.get("settings"), dict)
        or len(payload["settings"]) != WORLD_OPTION_METADATA_ENTRIES
    ):
        raise WorldOptionEditError("WorldOption metadata is not for Palworld 1.0.0")
    settings: dict[str, dict[str, str]] = {}
    for key, value in payload["settings"].items():
        if not isinstance(key, str) or not isinstance(value, dict):
            raise WorldOptionEditError("WorldOption metadata contains an invalid entry")
        kind = value.get("kind")
        if kind not in {"Bool", "Int", "Float", "Str", "Enum", "Array"}:
            raise WorldOptionEditError(f"WorldOption metadata has invalid type for {key}")
        entry = {"kind": kind}
        if kind in {"Enum", "Array"}:
            enum_type = value.get("enum_type")
            if not isinstance(enum_type, str) or not ENUM_VALUE_RE.fullmatch(enum_type):
                raise WorldOptionEditError(
                    f"WorldOption metadata has invalid enum type for {key}"
                )
            entry["enum_type"] = enum_type
        settings[key] = entry
    return settings


def parse_palworld_settings(content: str) -> dict[str, Any]:
    text = content.lstrip("\ufeff")
    section = "[/Script/Pal.PalGameWorldSettings]"
    marker = "OptionSettings=("
    section_index = text.find(section)
    marker_index = text.find(marker)
    if section_index < 0 or marker_index < section_index:
        raise WorldOptionEditError("PalWorldSettings.ini is missing OptionSettings")
    body_start = marker_index + len(marker)
    depth = 1
    quoted = False
    escaped = False
    body_end = -1
    for index, character in enumerate(text[body_start:], start=body_start):
        if escaped:
            escaped = False
            continue
        if quoted and character == "\\":
            escaped = True
            continue
        if character == '"':
            quoted = not quoted
            continue
        if quoted:
            continue
        if character == "(":
            depth += 1
        elif character == ")":
            depth -= 1
            if depth == 0:
                body_end = index
                break
    if body_end < 0 or quoted:
        raise WorldOptionEditError("PalWorldSettings.ini has incomplete OptionSettings")

    settings: dict[str, Any] = {}
    for token in _split_top_level(text[body_start:body_end]):
        if not token.strip():
            continue
        if "=" not in token:
            raise WorldOptionEditError(f"Invalid OptionSettings entry: {token.strip()}")
        key, raw_value = token.split("=", 1)
        key = key.strip()
        if not key or not ENUM_VALUE_RE.fullmatch(key):
            raise WorldOptionEditError(f"Invalid OptionSettings key: {key!r}")
        if key in settings:
            raise WorldOptionEditError(f"Duplicate OptionSettings key: {key}")
        settings[key] = _parse_ini_value(raw_value.strip())
    if not settings:
        raise WorldOptionEditError("PalWorldSettings.ini OptionSettings is empty")
    return settings


def _split_top_level(value: str) -> list[str]:
    entries: list[str] = []
    start = 0
    depth = 0
    quoted = False
    escaped = False
    for index, character in enumerate(value):
        if escaped:
            escaped = False
            continue
        if quoted and character == "\\":
            escaped = True
            continue
        if character == '"':
            quoted = not quoted
            continue
        if quoted:
            continue
        if character == "(":
            depth += 1
        elif character == ")":
            depth -= 1
            if depth < 0:
                raise WorldOptionEditError("OptionSettings has an unexpected closing parenthesis")
        elif character == "," and depth == 0:
            entries.append(value[start:index])
            start = index + 1
    if quoted or depth != 0:
        raise WorldOptionEditError("OptionSettings contains an incomplete value")
    entries.append(value[start:])
    return entries


def _parse_ini_value(raw: str) -> Any:
    if not raw:
        return ""
    if raw.startswith('"'):
        if not raw.endswith('"') or len(raw) < 2:
            raise WorldOptionEditError("OptionSettings contains an incomplete string")
        try:
            value = json.loads(raw)
        except json.JSONDecodeError as exc:
            raise WorldOptionEditError(f"OptionSettings contains an invalid string: {exc}") from exc
        if not isinstance(value, str):
            raise WorldOptionEditError("OptionSettings string value is invalid")
        return value
    if raw.startswith("("):
        if not raw.endswith(")"):
            raise WorldOptionEditError("OptionSettings contains an incomplete array")
        inner = raw[1:-1]
        if not inner.strip():
            return []
        values = [_parse_ini_value(item.strip()) for item in _split_top_level(inner)]
        if any(not isinstance(item, str) for item in values):
            raise WorldOptionEditError("OptionSettings arrays must contain names or strings")
        return values
    lowered = raw.lower()
    if lowered == "true":
        return True
    if lowered == "false":
        return False
    if INTEGER_RE.fullmatch(raw):
        return int(raw)
    if FLOAT_RE.fullmatch(raw):
        value = float(raw)
        if not math.isfinite(value):
            raise WorldOptionEditError("OptionSettings number must be finite")
        return value
    return raw


def _properties(gvas_or_properties: Any) -> dict[str, Any]:
    if hasattr(gvas_or_properties, "properties"):
        return gvas_or_properties.properties
    if isinstance(gvas_or_properties, dict):
        return gvas_or_properties.get("properties", gvas_or_properties)
    raise WorldOptionEditError("Unsupported WorldOption GVAS object")


def _world_option_settings(gvas: Any) -> dict[str, Any]:
    header = getattr(gvas, "header", None)
    if header is not None and getattr(header, "save_game_class_name", "") != WORLD_OPTION_CLASS:
        raise WorldOptionEditError("Save is not a Palworld WorldOption.sav")
    try:
        properties = _properties(gvas)
        option_world = properties["OptionWorldData"]
        if option_world.get("struct_type") != "PalOptionWorldSaveData":
            raise KeyError("OptionWorldData struct type")
        settings_property = option_world["value"]["Settings"]
        if settings_property.get("struct_type") != "PalOptionWorldSettings":
            raise KeyError("Settings struct type")
        settings = settings_property["value"]
    except (KeyError, TypeError) as exc:
        raise WorldOptionEditError("WorldOption.sav has an unsupported settings structure") from exc
    if not isinstance(settings, dict):
        raise WorldOptionEditError("WorldOption.sav settings are invalid")
    return settings


def _make_property(key: str, value: Any, metadata: dict[str, str]) -> dict[str, Any]:
    kind = metadata["kind"]
    if kind == "Bool":
        if not isinstance(value, bool):
            raise WorldOptionEditError(f"{key} must be True or False")
        return {"value": value, "id": None, "type": "BoolProperty"}
    if kind == "Int":
        if isinstance(value, bool) or not isinstance(value, int):
            raise WorldOptionEditError(f"{key} must be an integer")
        if value < -(2**31) or value > 2**31 - 1:
            raise WorldOptionEditError(f"{key} exceeds IntProperty range")
        return {"id": None, "value": value, "type": "IntProperty"}
    if kind == "Float":
        if isinstance(value, bool) or not isinstance(value, (int, float)):
            raise WorldOptionEditError(f"{key} must be a number")
        number = float(value)
        if not math.isfinite(number):
            raise WorldOptionEditError(f"{key} must be finite")
        return {"id": None, "value": number, "type": "FloatProperty"}
    if kind == "Str":
        if isinstance(value, list):
            if any(not isinstance(item, str) for item in value):
                raise WorldOptionEditError(f"{key} contains an invalid string")
            value = ",".join(value)
        if not isinstance(value, str):
            raise WorldOptionEditError(f"{key} must be a string")
        if len(value) > 4096:
            raise WorldOptionEditError(f"{key} exceeds the safe string limit")
        return {"id": None, "value": value, "type": "StrProperty"}
    if kind == "Enum":
        if not isinstance(value, str):
            raise WorldOptionEditError(f"{key} must be an enum name")
        enum_type = metadata["enum_type"]
        enum_value = value.split("::", 1)[-1]
        if not ENUM_VALUE_RE.fullmatch(enum_value):
            raise WorldOptionEditError(f"{key} has an invalid enum value")
        allowed_values = ENUM_ALLOWED_VALUES.get(key)
        if allowed_values is None or enum_value not in allowed_values:
            raise WorldOptionEditError(
                f"{key} is not a supported Palworld {GAME_VERSION} enum value"
            )
        return {
            "id": None,
            "value": {"type": enum_type, "value": f"{enum_type}::{enum_value}"},
            "type": "EnumProperty",
        }
    if kind == "Array":
        if not isinstance(value, list) or len(value) > 64:
            raise WorldOptionEditError(f"{key} must be a short array")
        enum_type = metadata["enum_type"]
        values: list[str] = []
        for item in value:
            if not isinstance(item, str):
                raise WorldOptionEditError(f"{key} contains an invalid enum value")
            enum_value = item.split("::", 1)[-1]
            if not ENUM_VALUE_RE.fullmatch(enum_value):
                raise WorldOptionEditError(f"{key} contains an invalid enum value")
            allowed_values = ENUM_ALLOWED_VALUES.get(key)
            if allowed_values is None or enum_value not in allowed_values:
                raise WorldOptionEditError(
                    f"{key} contains an unsupported Palworld {GAME_VERSION} enum value"
                )
            values.append(f"{enum_type}::{enum_value}")
        return {
            "array_type": "EnumProperty",
            "id": None,
            "value": {"values": values},
            "type": "ArrayProperty",
        }
    raise WorldOptionEditError(f"Unsupported WorldOption type for {key}")


def _mapping_digest(value: dict[str, Any]) -> str:
    payload = json.dumps(
        value,
        ensure_ascii=True,
        sort_keys=True,
        separators=(",", ":"),
    ).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def sync_world_option(
    gvas: Any,
    ini_content: str,
    metadata: dict[str, dict[str, str]],
) -> WorldOptionSyncResult:
    parsed = parse_palworld_settings(ini_content)
    settings = _world_option_settings(gvas)
    expected: dict[str, dict[str, Any]] = {}
    skipped: list[str] = []
    for key, value in parsed.items():
        entry = metadata.get(key)
        if entry is None:
            skipped.append(key)
            continue
        expected[key] = _make_property(key, value, entry)
    if not expected:
        raise WorldOptionEditError("No Palworld 1.0.0 WorldOption settings were found")

    preserved = {key: value for key, value in settings.items() if key not in expected}
    for key, value in expected.items():
        settings[key] = value

    properties = _properties(gvas)
    timestamp = properties.get("Timestamp")
    if isinstance(timestamp, dict) and timestamp.get("type") == "StructProperty":
        timestamp["value"] = 621355968000000000 + time.time_ns() // 100

    return WorldOptionSyncResult(
        updated_keys=tuple(sorted(expected)),
        skipped_keys=tuple(sorted(skipped)),
        settings_digest=_mapping_digest(expected),
        expected_properties=expected,
        preserved_digest=_mapping_digest(preserved),
    )


def verify_world_option_sync(gvas: Any, result: WorldOptionSyncResult) -> None:
    settings = _world_option_settings(gvas)
    for key, expected in result.expected_properties.items():
        if settings.get(key) != expected:
            raise WorldOptionEditError(f"Rebuilt WorldOption.sav changed {key}")
    preserved = {
        key: value
        for key, value in settings.items()
        if key not in result.expected_properties
    }
    if _mapping_digest(preserved) != result.preserved_digest:
        raise WorldOptionEditError(
            "Rebuilt WorldOption.sav changed settings outside the requested sync"
        )
    actual = {key: settings[key] for key in result.expected_properties}
    if _mapping_digest(actual) != result.settings_digest:
        raise WorldOptionEditError("Rebuilt WorldOption.sav settings digest mismatch")
