from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any


class PlayerEditError(ValueError):
    pass


class PlayerConflictError(PlayerEditError):
    pass


MAX_PLAYER_LEVEL = 80
MAX_UNUSED_STATUS_POINTS = 0xFFFF


@dataclass(frozen=True)
class PlayerProfileResult:
    player_uid: str
    nickname_before: str
    nickname_after: str
    level_before: int
    level_after: int
    exp_before: int
    exp_after: int
    character_records: int
    guild_records: int

    def to_dict(self) -> dict[str, Any]:
        return {
            "player_uid": self.player_uid,
            "nickname_before": self.nickname_before,
            "nickname_after": self.nickname_after,
            "level_before": self.level_before,
            "level_after": self.level_after,
            "exp_before": self.exp_before,
            "exp_after": self.exp_after,
            "character_records": self.character_records,
            "guild_records": self.guild_records,
        }


@dataclass(frozen=True)
class PlayerStatPointsResult:
    player_uid: str
    before: int
    after: int
    character_records: int

    def to_dict(self) -> dict[str, Any]:
        return {
            "player_uid": self.player_uid,
            "before": self.before,
            "after": self.after,
            "character_records": self.character_records,
        }


def load_exp_table(path: str) -> dict[int, int]:
    exp_path = Path(path)
    if not exp_path.is_file():
        raise PlayerEditError(f"Experience table does not exist: {exp_path}")
    payload = json.loads(exp_path.read_text(encoding="utf-8"))
    table: dict[int, int] = {}
    for raw_level, values in payload.items():
        if not isinstance(values, dict):
            continue
        try:
            level = int(raw_level)
            total_exp = int(values["TotalEXP"])
        except (KeyError, TypeError, ValueError):
            continue
        if level >= 1 and total_exp >= 0:
            table[level] = total_exp
    if not table or min(table) != 1:
        raise PlayerEditError("Experience table is empty or does not start at level 1")
    levels = sorted(table)
    if levels != list(range(1, max(levels) + 1)):
        raise PlayerEditError("Experience table levels are not contiguous")
    if any(table[level] > table[level + 1] for level in levels[:-1]):
        raise PlayerEditError("Experience table totals are not monotonic")
    return table


def _properties(gvas_or_properties: Any) -> dict[str, Any]:
    if hasattr(gvas_or_properties, "properties"):
        return gvas_or_properties.properties
    if isinstance(gvas_or_properties, dict):
        return gvas_or_properties.get("properties", gvas_or_properties)
    raise PlayerEditError("Unsupported GVAS data object")


def _world(level_gvas: Any) -> dict[str, Any]:
    try:
        return _properties(level_gvas)["worldSaveData"]["value"]
    except (KeyError, TypeError) as exc:
        raise PlayerEditError("Level.sav does not contain worldSaveData") from exc


def _normalize_uid(value: Any) -> str:
    return str(value).replace("-", "").lower()


def _canonical_player_uid(value: Any) -> str:
    raw = _normalize_uid(value).strip()
    if not raw:
        raise PlayerEditError("Player UID cannot be empty")
    if raw.isdecimal() and len(raw) <= 10:
        numeric_uid = int(raw)
        if numeric_uid > 0xFFFFFFFF:
            raise PlayerEditError(f"Invalid decimal player UID: {value}")
        return f"{numeric_uid:08x}" + ("0" * 24)
    if len(raw) == 8:
        raw += "0" * 24
    if len(raw) != 32 or any(char not in "0123456789abcdef" for char in raw):
        raise PlayerEditError(f"Invalid player UID: {value}")
    return raw


def _player_character_records(world: dict[str, Any], player_uid: str) -> list[dict[str, Any]]:
    target = _canonical_player_uid(player_uid)
    try:
        entries = world["CharacterSaveParameterMap"]["value"]
    except (KeyError, TypeError) as exc:
        raise PlayerEditError("Level.sav does not contain player character records") from exc

    records: list[dict[str, Any]] = []
    for entry in entries:
        key = entry.get("key", {})
        if _normalize_uid(key.get("PlayerUId", {}).get("value", "")) != target:
            continue
        save_parameter = (
            entry.get("value", {})
            .get("RawData", {})
            .get("value", {})
            .get("object", {})
            .get("SaveParameter", {})
        )
        if save_parameter.get("struct_type") != "PalIndividualCharacterSaveParameter":
            continue
        value = save_parameter.get("value", {})
        if value.get("IsPlayer", {}).get("value") is True:
            records.append(value)
    return records


def _guild_player_records(world: dict[str, Any], player_uid: str) -> list[dict[str, Any]]:
    target = _canonical_player_uid(player_uid)
    records: list[dict[str, Any]] = []
    for group in world.get("GroupSaveDataMap", {}).get("value", []):
        group_value = group.get("value", {})
        group_type = group_value.get("GroupType", {}).get("value", {}).get("value")
        if group_type != "EPalGroupType::Guild":
            continue
        raw = group_value.get("RawData", {}).get("value", {})
        for player in raw.get("players", []):
            if _normalize_uid(player.get("player_uid", "")) == target:
                records.append(player)
    return records


def _read_character_profile(record: dict[str, Any]) -> tuple[str, str | None, int, int]:
    try:
        nickname = str(record["NickName"]["value"])
        filtered_property = record.get("FilteredNickName")
        filtered_nickname = (
            str(filtered_property["value"])
            if filtered_property is not None
            else None
        )
        level = int(record["Level"]["value"]["value"])
        exp = int(record["Exp"]["value"])
    except (KeyError, TypeError, ValueError) as exc:
        raise PlayerEditError("Player character record is missing profile fields") from exc
    return nickname, filtered_nickname, level, exp


def _validate_unused_stat_points(value: Any, label: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise PlayerEditError(f"{label} must be an integer")
    if value < 0 or value > MAX_UNUSED_STATUS_POINTS:
        raise PlayerEditError(
            f"{label} must be between 0 and {MAX_UNUSED_STATUS_POINTS}"
        )
    return value


def _read_unused_stat_points(record: dict[str, Any]) -> int:
    property_value = record.get("UnusedStatusPoint")
    if not isinstance(property_value, dict):
        raise PlayerEditError("Player character record is missing UnusedStatusPoint")
    if property_value.get("type") != "UInt16Property":
        raise PlayerEditError("UnusedStatusPoint must be a UInt16Property")
    if "value" not in property_value:
        raise PlayerEditError("UnusedStatusPoint is missing its value")
    return _validate_unused_stat_points(
        property_value["value"],
        "Stored unused stat points",
    )


def set_player_stat_points(
    level_gvas: Any,
    player_uid: str,
    expected_unused_stat_points: int,
    unused_stat_points: int,
) -> PlayerStatPointsResult:
    expected_unused_stat_points = _validate_unused_stat_points(
        expected_unused_stat_points,
        "Expected unused stat points",
    )
    unused_stat_points = _validate_unused_stat_points(
        unused_stat_points,
        "Unused stat points",
    )

    world = _world(level_gvas)
    character_records = _player_character_records(world, player_uid)
    if len(character_records) != 1:
        raise PlayerEditError(
            f"Expected one player character record, found {len(character_records)}"
        )

    character = character_records[0]
    current_unused_stat_points = _read_unused_stat_points(character)
    if current_unused_stat_points != expected_unused_stat_points:
        raise PlayerConflictError(
            "Player unused stat points changed from "
            f"{expected_unused_stat_points} to {current_unused_stat_points}"
        )
    if unused_stat_points == current_unused_stat_points:
        raise PlayerEditError("Player unused stat points are unchanged")

    character["UnusedStatusPoint"]["value"] = unused_stat_points
    return PlayerStatPointsResult(
        player_uid=player_uid,
        before=current_unused_stat_points,
        after=unused_stat_points,
        character_records=len(character_records),
    )


def verify_player_stat_points(
    level_gvas: Any,
    result: PlayerStatPointsResult,
) -> None:
    world = _world(level_gvas)
    character_records = _player_character_records(world, result.player_uid)
    if len(character_records) != result.character_records:
        raise PlayerEditError("Rebuilt save changed the player character record count")
    unused_stat_points = _read_unused_stat_points(character_records[0])
    if unused_stat_points != result.after:
        raise PlayerEditError("Rebuilt save did not persist the unused stat points")


def set_player_profile(
    level_gvas: Any,
    player_uid: str,
    expected_nickname: str,
    expected_level: int,
    nickname: str,
    level: int,
    exp_table: dict[int, int],
) -> PlayerProfileResult:
    nickname = nickname.strip()
    if not nickname:
        raise PlayerEditError("Nickname cannot be empty")
    if len(nickname) > 32:
        raise PlayerEditError("Nickname cannot exceed 32 characters")
    if level < 1 or level > MAX_PLAYER_LEVEL or level not in exp_table:
        raise PlayerEditError(f"Level must be between 1 and {MAX_PLAYER_LEVEL}")
    if expected_level < 1 or expected_level > MAX_PLAYER_LEVEL:
        raise PlayerEditError(
            f"Expected level must be between 1 and {MAX_PLAYER_LEVEL}"
        )

    world = _world(level_gvas)
    character_records = _player_character_records(world, player_uid)
    if len(character_records) != 1:
        raise PlayerEditError(
            f"Expected one player character record, found {len(character_records)}"
        )
    current_nickname, filtered_nickname, current_level, current_exp = _read_character_profile(
        character_records[0]
    )
    if current_nickname != expected_nickname:
        raise PlayerConflictError(
            f"Player nickname changed from {expected_nickname!r} to {current_nickname!r}"
        )
    if current_level != expected_level:
        raise PlayerConflictError(
            f"Player level changed from {expected_level} to {current_level}"
        )
    if filtered_nickname is not None and filtered_nickname != current_nickname:
        raise PlayerEditError(
            f"Filtered nickname {filtered_nickname!r} does not match "
            f"character nickname {current_nickname!r}"
        )
    if nickname == current_nickname and level == current_level:
        raise PlayerEditError("Player profile is unchanged")

    guild_records = _guild_player_records(world, player_uid)
    for guild_player in guild_records:
        guild_name = str(guild_player.get("player_info", {}).get("player_name", ""))
        if guild_name != current_nickname:
            raise PlayerEditError(
                f"Guild player nickname {guild_name!r} does not match "
                f"character nickname {current_nickname!r}"
            )

    character = character_records[0]
    character["NickName"]["value"] = nickname
    if "FilteredNickName" in character:
        character["FilteredNickName"]["value"] = nickname
    character["Level"]["value"]["value"] = level
    exp_after = exp_table[level] if level != current_level else current_exp
    character["Exp"]["value"] = exp_after
    for guild_player in guild_records:
        guild_player.setdefault("player_info", {})["player_name"] = nickname

    return PlayerProfileResult(
        player_uid=player_uid,
        nickname_before=current_nickname,
        nickname_after=nickname,
        level_before=current_level,
        level_after=level,
        exp_before=current_exp,
        exp_after=exp_after,
        character_records=len(character_records),
        guild_records=len(guild_records),
    )


def verify_player_profile(level_gvas: Any, result: PlayerProfileResult) -> None:
    world = _world(level_gvas)
    character_records = _player_character_records(world, result.player_uid)
    if len(character_records) != result.character_records:
        raise PlayerEditError("Rebuilt save changed the player character record count")
    nickname, filtered_nickname, level, exp = _read_character_profile(
        character_records[0]
    )
    if (nickname, level, exp) != (
        result.nickname_after,
        result.level_after,
        result.exp_after,
    ):
        raise PlayerEditError("Rebuilt save did not persist the player profile")
    if filtered_nickname is not None and filtered_nickname != result.nickname_after:
        raise PlayerEditError("Rebuilt save did not persist the filtered nickname")

    guild_records = _guild_player_records(world, result.player_uid)
    if len(guild_records) != result.guild_records:
        raise PlayerEditError("Rebuilt save changed the guild player record count")
    for guild_player in guild_records:
        if guild_player.get("player_info", {}).get("player_name") != result.nickname_after:
            raise PlayerEditError("Rebuilt save did not persist the guild player nickname")
