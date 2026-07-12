import copy
import hashlib
import json
import math
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from uuid import UUID


class PalEditError(ValueError):
    pass


class PalConflictError(PalEditError):
    pass


MAX_PAL_LEVEL = 80
MAX_PAL_NICKNAME_LENGTH = 32
FRIENDSHIP_RANK_COUNT = 11
MAX_CONDENSER_RANK = 5


@dataclass(frozen=True)
class PalNicknameResult:
    player_uid: str
    instance_id: str
    pal_type: str
    nickname_before: str
    nickname_after: str
    level: int
    exp: int
    nickname_created: bool

    def to_dict(self) -> dict[str, Any]:
        return {
            "player_uid": self.player_uid,
            "instance_id": self.instance_id,
            "pal_type": self.pal_type,
            "nickname_before": self.nickname_before,
            "nickname_after": self.nickname_after,
            "level": self.level,
            "exp": self.exp,
            "nickname_created": self.nickname_created,
        }


@dataclass(frozen=True)
class PalLevelMetadata:
    hp_scaling: float
    friendship_hp: float


@dataclass(frozen=True)
class PalLevelResult:
    player_uid: str
    instance_id: str
    pal_type: str
    nickname: str
    level_before: int
    level_after: int
    exp_before: int
    exp_after: int
    hp_before: int
    hp_after: int
    max_hp_before: int
    max_hp_after: int
    health_field: str
    max_hp_created: bool
    level_property_type: str
    exp_property_type: str
    expected_record: dict[str, Any] = field(repr=False, compare=False)
    expected_character_entries_digest: str = field(repr=False, compare=False)

    def to_dict(self) -> dict[str, Any]:
        return {
            "player_uid": self.player_uid,
            "instance_id": self.instance_id,
            "pal_type": self.pal_type,
            "nickname": self.nickname,
            "level_before": self.level_before,
            "level_after": self.level_after,
            "exp_before": self.exp_before,
            "exp_after": self.exp_after,
            "hp_before": self.hp_before,
            "hp_after": self.hp_after,
            "max_hp_before": self.max_hp_before,
            "max_hp_after": self.max_hp_after,
            "health_field": self.health_field,
            "max_hp_created": self.max_hp_created,
        }


@dataclass(frozen=True)
class PalHealthResult:
    player_uid: str
    instance_id: str
    pal_type: str
    nickname: str
    level: int
    exp: int
    hp_before: int
    hp_after: int
    max_hp: int
    health_field: str
    expected_record: dict[str, Any] = field(repr=False, compare=False)
    expected_character_entries_digest: str = field(repr=False, compare=False)

    def to_dict(self) -> dict[str, Any]:
        return {
            "player_uid": self.player_uid,
            "instance_id": self.instance_id,
            "pal_type": self.pal_type,
            "nickname": self.nickname,
            "level": self.level,
            "exp": self.exp,
            "hp_before": self.hp_before,
            "hp_after": self.hp_after,
            "max_hp": self.max_hp,
            "health_field": self.health_field,
        }


def load_pal_exp_table(path: str) -> dict[int, int]:
    exp_path = Path(path)
    if not exp_path.is_file():
        raise PalEditError(f"Experience table does not exist: {exp_path}")
    try:
        payload = json.loads(exp_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise PalEditError(f"Unable to read Pal experience table: {exp_path}") from exc
    if not isinstance(payload, dict):
        raise PalEditError("Pal experience table must contain a JSON object")
    table: dict[int, int] = {}
    for raw_level, values in payload.items():
        if not isinstance(values, dict):
            continue
        try:
            level = int(raw_level)
            total_exp = int(values["PalTotalEXP"])
        except (KeyError, TypeError, ValueError):
            continue
        if 1 <= level <= MAX_PAL_LEVEL and total_exp >= 0:
            table[level] = total_exp
    levels = list(range(1, MAX_PAL_LEVEL + 1))
    if sorted(table) != levels:
        raise PalEditError(
            f"Pal experience table must contain levels 1 through {MAX_PAL_LEVEL}"
        )
    if table[1] != 0:
        raise PalEditError("Pal experience table level 1 total must be zero")
    if any(table[level] >= table[level + 1] for level in levels[:-1]):
        raise PalEditError("Pal experience table totals must increase by level")
    return table


def load_pal_level_metadata(
    path: str,
) -> tuple[dict[str, PalLevelMetadata], tuple[int, ...]]:
    metadata_path = Path(path)
    if not metadata_path.is_file():
        raise PalEditError(f"Pal level metadata does not exist: {metadata_path}")
    try:
        payload = json.loads(metadata_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise PalEditError(f"Unable to read Pal level metadata: {metadata_path}") from exc
    if not isinstance(payload, dict):
        raise PalEditError("Pal level metadata must contain a JSON object")
    if payload.get("schema") != 1 or payload.get("game_version") != "1.0.0":
        raise PalEditError("Unsupported Pal level metadata schema or game version")
    if payload.get("max_level") != MAX_PAL_LEVEL:
        raise PalEditError(f"Pal level metadata must target level {MAX_PAL_LEVEL}")

    raw_thresholds = payload.get("friendship_thresholds")
    if (
        not isinstance(raw_thresholds, list)
        or len(raw_thresholds) != FRIENDSHIP_RANK_COUNT
    ):
        raise PalEditError("Pal level metadata is missing friendship thresholds")
    thresholds: list[int] = []
    for value in raw_thresholds:
        if isinstance(value, bool) or not isinstance(value, int) or value < 0:
            raise PalEditError("Friendship thresholds must be non-negative integers")
        thresholds.append(value)
    if thresholds[0] != 0 or thresholds != sorted(set(thresholds)):
        raise PalEditError("Friendship thresholds must start at zero and increase")

    raw_catalog = payload.get("pals")
    if not isinstance(raw_catalog, dict) or not raw_catalog:
        raise PalEditError("Pal level metadata catalog is empty")
    catalog: dict[str, PalLevelMetadata] = {}
    for raw_id, values in raw_catalog.items():
        pal_id = str(raw_id).strip().lower()
        if not pal_id or not isinstance(values, dict):
            raise PalEditError("Pal level metadata contains an invalid entry")
        if pal_id in catalog:
            raise PalEditError(f"Pal level metadata contains duplicate ID {raw_id}")
        hp_scaling = values.get("hp_scaling")
        friendship_hp = values.get("friendship_hp", 0)
        if (
            isinstance(hp_scaling, bool)
            or not isinstance(hp_scaling, (int, float))
            or not math.isfinite(float(hp_scaling))
            or float(hp_scaling) <= 0
        ):
            raise PalEditError(f"Pal {raw_id} has invalid HP scaling")
        if (
            isinstance(friendship_hp, bool)
            or not isinstance(friendship_hp, (int, float))
            or not math.isfinite(float(friendship_hp))
            or float(friendship_hp) < 0
        ):
            raise PalEditError(f"Pal {raw_id} has invalid friendship HP scaling")
        catalog[pal_id] = PalLevelMetadata(
            hp_scaling=float(hp_scaling),
            friendship_hp=float(friendship_hp),
        )
    return catalog, tuple(thresholds)


def _properties(gvas_or_properties: Any) -> dict[str, Any]:
    if hasattr(gvas_or_properties, "properties"):
        return gvas_or_properties.properties
    if isinstance(gvas_or_properties, dict):
        return gvas_or_properties.get("properties", gvas_or_properties)
    raise PalEditError("Unsupported GVAS data object")


def _world(level_gvas: Any) -> dict[str, Any]:
    try:
        return _properties(level_gvas)["worldSaveData"]["value"]
    except (KeyError, TypeError) as exc:
        raise PalEditError("Level.sav does not contain worldSaveData") from exc


def _character_entries(world: dict[str, Any]) -> list[dict[str, Any]]:
    try:
        entries = world["CharacterSaveParameterMap"]["value"]
    except (KeyError, TypeError) as exc:
        raise PalEditError("Level.sav does not contain character records") from exc
    if not isinstance(entries, list):
        raise PalEditError("Level.sav character records must be an array")
    return entries


def _update_structure_digest(digest: Any, value: Any) -> None:
    if value is None:
        digest.update(b"N;")
    elif isinstance(value, bool):
        digest.update(b"B1;" if value else b"B0;")
    elif isinstance(value, int):
        digest.update(f"I{value};".encode("ascii"))
    elif isinstance(value, float):
        digest.update(f"F{value.hex()};".encode("ascii"))
    elif isinstance(value, str):
        encoded = value.encode("utf-8")
        digest.update(f"S{len(encoded)}:".encode("ascii"))
        digest.update(encoded)
    elif isinstance(value, bytes):
        digest.update(f"Y{len(value)}:".encode("ascii"))
        digest.update(value)
    elif isinstance(value, dict):
        digest.update(f"D{len(value)}:".encode("ascii"))
        for key in sorted(value, key=lambda item: (type(item).__name__, repr(item))):
            _update_structure_digest(digest, key)
            _update_structure_digest(digest, value[key])
    elif isinstance(value, (list, tuple)):
        digest.update(f"L{len(value)}:".encode("ascii"))
        for item in value:
            _update_structure_digest(digest, item)
    else:
        type_name = f"{type(value).__module__}.{type(value).__qualname__}"
        _update_structure_digest(digest, type_name)
        _update_structure_digest(digest, str(value))


def _structure_digest(value: Any) -> str:
    digest = hashlib.sha256()
    _update_structure_digest(digest, value)
    return digest.hexdigest()


def _normalize_guid(value: Any, label: str) -> str:
    try:
        return UUID(str(value)).hex
    except (AttributeError, TypeError, ValueError) as exc:
        raise PalEditError(f"Invalid {label}: {value}") from exc


def _canonical_instance_id(value: Any) -> str:
    normalized = _normalize_guid(value, "Pal instance ID")
    if normalized == "0" * 32:
        raise PalEditError("Pal instance ID cannot be zero")
    return str(UUID(hex=normalized))


def _canonical_player_uid(value: Any) -> str:
    raw = str(value).replace("-", "").strip().lower()
    if not raw:
        raise PalEditError("Player UID cannot be empty")
    if raw.isdecimal() and len(raw) <= 10:
        numeric_uid = int(raw)
        if numeric_uid > 0xFFFFFFFF:
            raise PalEditError(f"Invalid decimal player UID: {value}")
        return f"{numeric_uid:08x}" + ("0" * 24)
    if len(raw) == 8:
        raw += "0" * 24
    if len(raw) != 32 or any(char not in "0123456789abcdef" for char in raw):
        raise PalEditError(f"Invalid player UID: {value}")
    return raw


def _pal_records(world: dict[str, Any], instance_id: str) -> list[dict[str, Any]]:
    target = _normalize_guid(instance_id, "Pal instance ID")

    records: list[dict[str, Any]] = []
    for entry in _character_entries(world):
        raw_instance_id = entry.get("key", {}).get("InstanceId", {}).get("value")
        if raw_instance_id is None:
            continue
        try:
            normalized = _normalize_guid(raw_instance_id, "stored Pal instance ID")
        except PalEditError:
            continue
        if normalized != target:
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
        value = save_parameter.get("value")
        if not isinstance(value, dict) or value.get("IsPlayer", {}).get("value") is True:
            continue
        records.append(value)
    return records


def _read_scalar_property(
    record: dict[str, Any],
    name: str,
    property_type: str,
    default: Any = None,
) -> Any:
    prop = record.get(name)
    if prop is None:
        return default
    if not isinstance(prop, dict) or prop.get("type") != property_type:
        raise PalEditError(f"Pal {name} must be a {property_type}")
    if "value" not in prop:
        raise PalEditError(f"Pal {name} is missing its value")
    value = prop["value"]
    if isinstance(value, dict) and "value" in value:
        value = value["value"]
    return value


def _require_int(value: Any, label: str, minimum: int = 0) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value < minimum:
        raise PalEditError(f"{label} must be an integer of at least {minimum}")
    return value


def _read_profile(record: dict[str, Any]) -> tuple[str, str, int, int]:
    nickname = _read_scalar_property(record, "NickName", "StrProperty", "")
    pal_type = _read_scalar_property(record, "CharacterID", "NameProperty")
    level_property = record.get("Level")
    exp_property = record.get("Exp")
    if not isinstance(level_property, dict):
        raise PalEditError("Pal record is missing Level")
    if level_property.get("type") == "ByteProperty":
        level = _read_scalar_property(record, "Level", "ByteProperty")
    elif level_property.get("type") == "IntProperty":
        level = _read_scalar_property(record, "Level", "IntProperty")
    else:
        raise PalEditError("Pal Level must be a ByteProperty or IntProperty")
    if not isinstance(exp_property, dict):
        raise PalEditError("Pal record is missing Exp")
    if exp_property.get("type") == "Int64Property":
        exp = _read_scalar_property(record, "Exp", "Int64Property")
    elif exp_property.get("type") == "IntProperty":
        exp = _read_scalar_property(record, "Exp", "IntProperty")
    else:
        raise PalEditError("Pal Exp must be an Int64Property or IntProperty")
    if not isinstance(nickname, str):
        raise PalEditError("Pal NickName must contain text")
    if not isinstance(pal_type, str) or not pal_type:
        raise PalEditError("Pal CharacterID must contain a Pal type")
    level = _require_int(level, "Stored Pal level", 1)
    if level > MAX_PAL_LEVEL:
        raise PalEditError(f"Stored Pal level cannot exceed {MAX_PAL_LEVEL}")
    exp = _require_int(exp, "Stored Pal EXP")
    return nickname, pal_type, level, exp


def _read_owner(record: dict[str, Any]) -> str:
    owner = record.get("OwnerPlayerUId")
    if not isinstance(owner, dict) or owner.get("type") != "StructProperty":
        raise PalEditError("Pal OwnerPlayerUId must be a StructProperty")
    if owner.get("struct_type") != "Guid" or "value" not in owner:
        raise PalEditError("Pal OwnerPlayerUId must contain a GUID")
    return _normalize_guid(owner["value"], "stored Pal owner UID")


def _read_fixed_point(record: dict[str, Any], name: str) -> int | None:
    prop = record.get(name)
    if prop is None:
        return None
    if (
        not isinstance(prop, dict)
        or prop.get("type") != "StructProperty"
        or prop.get("struct_type") != "FixedPoint64"
    ):
        raise PalEditError(f"Pal {name} must be a FixedPoint64 StructProperty")
    try:
        value_property = prop["value"]["Value"]
        if value_property.get("type") != "Int64Property":
            raise PalEditError(f"Pal {name}.Value must be an Int64Property")
        value = value_property["value"]
    except (KeyError, TypeError) as exc:
        raise PalEditError(f"Pal {name} is missing its fixed-point value") from exc
    return _require_int(value, f"Stored Pal {name}")


def _read_health(record: dict[str, Any]) -> tuple[str, int, int, bool]:
    health_fields = [name for name in ("Hp", "HP") if name in record]
    if not health_fields:
        raise PalEditError("Pal record is missing Hp or HP")
    if len(health_fields) != 1:
        raise PalEditError("Pal record contains both Hp and HP")
    health_field = health_fields[0]
    hp = _read_fixed_point(record, health_field)
    assert hp is not None
    max_hp = _read_fixed_point(record, "MaxHP")
    return health_field, hp, 0 if max_hp is None else max_hp, max_hp is not None


def _read_optional_int_property(
    record: dict[str, Any],
    name: str,
    allowed_types: tuple[str, ...],
    default: int,
) -> int:
    prop = record.get(name)
    if prop is None:
        return default
    if not isinstance(prop, dict) or prop.get("type") not in allowed_types:
        expected = " or ".join(allowed_types)
        raise PalEditError(f"Pal {name} must be a {expected}")
    property_type = prop["type"]
    value = prop.get("value")
    if property_type == "ByteProperty":
        if not isinstance(value, dict) or value.get("type") != "None":
            raise PalEditError(f"Pal {name} ByteProperty has an invalid value shape")
        value = value.get("value")
    elif isinstance(value, dict):
        raise PalEditError(f"Pal {name} {property_type} has an invalid value shape")
    if isinstance(value, bool) or not isinstance(value, int):
        raise PalEditError(f"Pal {name} must contain an integer")
    return value


def _read_optional_bool_property(
    record: dict[str, Any],
    name: str,
    default: bool,
) -> bool:
    prop = record.get(name)
    if prop is None:
        return default
    if not isinstance(prop, dict) or prop.get("type") != "BoolProperty":
        raise PalEditError(f"Pal {name} must be a BoolProperty")
    value = prop.get("value")
    if not isinstance(value, bool):
        raise PalEditError(f"Pal {name} must contain a boolean")
    return value


def _resolve_level_metadata(
    pal_type: str,
    catalog: dict[str, PalLevelMetadata],
) -> PalLevelMetadata:
    key = pal_type.strip().lower()
    if key in catalog:
        return catalog[key]
    if key.startswith("b_o_s_s_"):
        canonical_boss = "boss_" + key.removeprefix("b_o_s_s_")
        if canonical_boss in catalog:
            return catalog[canonical_boss]
    raise PalEditError(f"No level metadata is available for Pal type {pal_type}")


def _friendship_rank(points: int, thresholds: tuple[int, ...]) -> int:
    for rank in range(len(thresholds) - 1, 0, -1):
        if points >= thresholds[rank]:
            return rank
    return 0


def _calculate_max_hp(
    record: dict[str, Any],
    level: int,
    metadata: PalLevelMetadata,
    friendship_thresholds: tuple[int, ...],
) -> int:
    talent_hp = _read_optional_int_property(
        record, "Talent_HP", ("ByteProperty", "IntProperty"), 0
    )
    rank_hp = _read_optional_int_property(
        record, "Rank_HP", ("ByteProperty", "IntProperty"), 0
    )
    condenser_rank = _read_optional_int_property(
        record, "Rank", ("ByteProperty", "IntProperty"), 0
    )
    friendship_points = _read_optional_int_property(
        record, "FriendshipPoint", ("IntProperty", "Int64Property"), 0
    )
    is_awake = _read_optional_bool_property(record, "bIsAwakening", False)
    if talent_hp < 0 or talent_hp > 100:
        raise PalEditError("Pal Talent_HP must be between 0 and 100")
    if rank_hp < 0 or rank_hp > 20:
        raise PalEditError("Pal Rank_HP must be between 0 and 20")
    if condenser_rank < 0 or condenser_rank > MAX_CONDENSER_RANK:
        raise PalEditError(
            f"Pal Rank must be between 0 and {MAX_CONDENSER_RANK}"
        )

    friendship_rank = _friendship_rank(friendship_points, friendship_thresholds)
    hp_iv = talent_hp * 0.3 / 100
    soul_bonus = rank_hp * 0.03
    condenser_bonus = max(0, condenser_rank - 1) * 0.05
    base = math.floor(
        500 + 5 * level + metadata.hp_scaling * 0.5 * level * (1 + hp_iv)
    )
    base_with_condenser = math.floor(base * (1 + condenser_bonus))
    trust_bonus = int(
        level
        * friendship_rank
        * metadata.friendship_hp
        * 0.65
        * (1 + condenser_bonus)
        + 0.5
    )
    awake_bonus = (
        math.floor(metadata.hp_scaling * level * 0.065 * (1 + condenser_bonus))
        if is_awake
        else 0
    )
    return math.floor(
        (base_with_condenser + trust_bonus + awake_bonus) * (1 + soul_bonus)
    ) * 1000


def _set_level_value(record: dict[str, Any], level: int) -> str:
    prop = record["Level"]
    property_type = prop["type"]
    if property_type == "ByteProperty":
        value = prop.get("value")
        if (
            not isinstance(value, dict)
            or value.get("type") != "None"
            or isinstance(value.get("value"), bool)
            or not isinstance(value.get("value"), int)
        ):
            raise PalEditError("Pal Level ByteProperty has an invalid value shape")
        value["value"] = level
    elif property_type == "IntProperty":
        if isinstance(prop.get("value"), bool) or not isinstance(
            prop.get("value"), int
        ):
            raise PalEditError("Pal Level IntProperty has an invalid value shape")
        prop["value"] = level
    else:
        raise PalEditError("Pal Level must be a ByteProperty or IntProperty")
    return property_type


def _set_exp_value(record: dict[str, Any], exp: int) -> str:
    prop = record["Exp"]
    property_type = prop["type"]
    if property_type not in ("Int64Property", "IntProperty"):
        raise PalEditError("Pal Exp must be an Int64Property or IntProperty")
    if isinstance(prop.get("value"), bool) or not isinstance(prop.get("value"), int):
        raise PalEditError("Pal Exp has an invalid value shape")
    prop["value"] = exp
    return property_type


def _set_fixed_point_value(prop: dict[str, Any], value: int) -> None:
    try:
        value_property = prop["value"]["Value"]
    except (KeyError, TypeError) as exc:
        raise PalEditError("Pal health has an invalid fixed-point value shape") from exc
    if (
        value_property.get("type") != "Int64Property"
        or isinstance(value_property.get("value"), bool)
        or not isinstance(value_property.get("value"), int)
    ):
        raise PalEditError("Pal health has an invalid fixed-point value shape")
    value_property["value"] = value


def _validate_nickname(value: Any, label: str) -> str:
    if not isinstance(value, str):
        raise PalEditError(f"{label} must be text")
    if len(value) > MAX_PAL_NICKNAME_LENGTH:
        raise PalEditError(
            f"{label} cannot exceed {MAX_PAL_NICKNAME_LENGTH} characters"
        )
    if any(ord(char) < 32 or ord(char) == 127 for char in value):
        raise PalEditError(f"{label} cannot contain control characters")
    return value


def _validate_level(value: Any, label: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise PalEditError(f"{label} must be an integer")
    if value < 1 or value > MAX_PAL_LEVEL:
        raise PalEditError(f"{label} must be between 1 and {MAX_PAL_LEVEL}")
    return value


def rename_pal(
    level_gvas: Any,
    player_uid: str,
    instance_id: str,
    expected_nickname: str,
    expected_level: int,
    expected_exp: int,
    nickname: str,
) -> PalNicknameResult:
    expected_nickname = _validate_nickname(expected_nickname, "Expected Pal nickname")
    nickname = _validate_nickname(nickname, "Pal nickname")
    expected_level = _validate_level(expected_level, "Expected Pal level")
    expected_exp = _require_int(expected_exp, "Expected Pal EXP")
    if expected_nickname == nickname:
        raise PalEditError("Pal nickname is unchanged")

    canonical_instance_id = _canonical_instance_id(instance_id)
    canonical_player_uid = _canonical_player_uid(player_uid)
    records = _pal_records(_world(level_gvas), canonical_instance_id)
    if len(records) != 1:
        raise PalEditError(
            f"Expected exactly one Pal record for instance {canonical_instance_id}, "
            f"found {len(records)}"
        )
    record = records[0]
    if _read_owner(record) != canonical_player_uid:
        raise PalConflictError("Pal owner changed before the edit")

    current_nickname, pal_type, current_level, current_exp = _read_profile(record)
    if current_nickname != expected_nickname:
        raise PalConflictError(
            f"Pal nickname changed from {expected_nickname!r} to {current_nickname!r}"
        )
    if current_level != expected_level:
        raise PalConflictError(
            f"Pal level changed from {expected_level} to {current_level}"
        )
    if current_exp != expected_exp:
        raise PalConflictError(f"Pal EXP changed from {expected_exp} to {current_exp}")

    nickname_created = "NickName" not in record
    if nickname_created:
        record["NickName"] = {
            "id": None,
            "type": "StrProperty",
            "value": nickname,
        }
    else:
        record["NickName"]["value"] = nickname

    return PalNicknameResult(
        player_uid=player_uid,
        instance_id=canonical_instance_id,
        pal_type=pal_type,
        nickname_before=current_nickname,
        nickname_after=nickname,
        level=current_level,
        exp=current_exp,
        nickname_created=nickname_created,
    )


def verify_pal_nickname(level_gvas: Any, result: PalNicknameResult) -> None:
    records = _pal_records(_world(level_gvas), result.instance_id)
    if len(records) != 1:
        raise PalEditError("Rebuilt save changed the Pal record count")
    record = records[0]
    if _read_owner(record) != _canonical_player_uid(result.player_uid):
        raise PalEditError("Rebuilt save changed the Pal owner")
    nickname, pal_type, level, exp = _read_profile(record)
    if pal_type != result.pal_type:
        raise PalEditError("Rebuilt save changed the Pal type")
    if (nickname, level, exp) != (
        result.nickname_after,
        result.level,
        result.exp,
    ):
        raise PalEditError("Rebuilt save did not persist only the Pal nickname")


def set_pal_level(
    level_gvas: Any,
    player_uid: str,
    instance_id: str,
    expected_nickname: str,
    expected_level: int,
    expected_exp: int,
    expected_hp: int,
    expected_max_hp: int,
    level: int,
    exp_table: dict[int, int],
    level_metadata: dict[str, PalLevelMetadata],
    friendship_thresholds: tuple[int, ...],
) -> PalLevelResult:
    expected_nickname = _validate_nickname(expected_nickname, "Expected Pal nickname")
    expected_level = _validate_level(expected_level, "Expected Pal level")
    level = _validate_level(level, "Pal level")
    expected_exp = _require_int(expected_exp, "Expected Pal EXP")
    expected_hp = _require_int(expected_hp, "Expected Pal HP")
    expected_max_hp = _require_int(expected_max_hp, "Expected Pal MaxHP")
    if level == expected_level:
        raise PalEditError("Pal level is unchanged")
    if level not in exp_table:
        raise PalEditError(f"Pal experience table does not contain level {level}")
    if (
        len(friendship_thresholds) != FRIENDSHIP_RANK_COUNT
        or friendship_thresholds[0] != 0
        or tuple(sorted(set(friendship_thresholds))) != friendship_thresholds
    ):
        raise PalEditError("Friendship thresholds must start at zero and increase")

    canonical_instance_id = _canonical_instance_id(instance_id)
    canonical_player_uid = _canonical_player_uid(player_uid)
    records = _pal_records(_world(level_gvas), canonical_instance_id)
    if len(records) != 1:
        raise PalEditError(
            f"Expected exactly one Pal record for instance {canonical_instance_id}, "
            f"found {len(records)}"
        )
    record = records[0]
    if _read_owner(record) != canonical_player_uid:
        raise PalConflictError("Pal owner changed before the edit")

    current_nickname, pal_type, current_level, current_exp = _read_profile(record)
    if current_nickname != expected_nickname:
        raise PalConflictError(
            f"Pal nickname changed from {expected_nickname!r} to {current_nickname!r}"
        )
    if current_level != expected_level:
        raise PalConflictError(
            f"Pal level changed from {expected_level} to {current_level}"
        )
    if current_exp != expected_exp:
        raise PalConflictError(f"Pal EXP changed from {expected_exp} to {current_exp}")

    health_field, hp_before, max_hp_before, has_max_hp = _read_health(record)
    if hp_before != expected_hp:
        raise PalConflictError(f"Pal HP changed from {expected_hp} to {hp_before}")
    if max_hp_before != expected_max_hp:
        raise PalConflictError(
            f"Pal MaxHP changed from {expected_max_hp} to {max_hp_before}"
        )

    metadata = _resolve_level_metadata(pal_type, level_metadata)
    hp_after = _calculate_max_hp(
        record,
        level,
        metadata,
        friendship_thresholds,
    )
    if hp_after <= 0:
        raise PalEditError("Calculated Pal MaxHP must be positive")
    exp_after = exp_table[level]
    updated_record = copy.deepcopy(record)
    level_property_type = _set_level_value(updated_record, level)
    exp_property_type = _set_exp_value(updated_record, exp_after)
    _set_fixed_point_value(updated_record[health_field], hp_after)
    if has_max_hp:
        _set_fixed_point_value(updated_record["MaxHP"], hp_after)
    else:
        updated_record["MaxHP"] = copy.deepcopy(updated_record[health_field])

    expected_record = copy.deepcopy(updated_record)
    record.clear()
    record.update(updated_record)
    expected_character_entries_digest = _structure_digest(
        _character_entries(_world(level_gvas))
    )

    return PalLevelResult(
        player_uid=player_uid,
        instance_id=canonical_instance_id,
        pal_type=pal_type,
        nickname=current_nickname,
        level_before=current_level,
        level_after=level,
        exp_before=current_exp,
        exp_after=exp_after,
        hp_before=hp_before,
        hp_after=hp_after,
        max_hp_before=max_hp_before,
        max_hp_after=hp_after,
        health_field=health_field,
        max_hp_created=not has_max_hp,
        level_property_type=level_property_type,
        exp_property_type=exp_property_type,
        expected_record=expected_record,
        expected_character_entries_digest=expected_character_entries_digest,
    )


def verify_pal_level(level_gvas: Any, result: PalLevelResult) -> None:
    records = _pal_records(_world(level_gvas), result.instance_id)
    if len(records) != 1:
        raise PalEditError("Rebuilt save changed the Pal record count")
    record = records[0]
    if _read_owner(record) != _canonical_player_uid(result.player_uid):
        raise PalEditError("Rebuilt save changed the Pal owner")
    nickname, pal_type, level, exp = _read_profile(record)
    if pal_type != result.pal_type or nickname != result.nickname:
        raise PalEditError("Rebuilt save changed the Pal identity")
    if record["Level"].get("type") != result.level_property_type:
        raise PalEditError("Rebuilt save changed the Pal Level property type")
    if record["Exp"].get("type") != result.exp_property_type:
        raise PalEditError("Rebuilt save changed the Pal Exp property type")
    if (level, exp) != (result.level_after, result.exp_after):
        raise PalEditError("Rebuilt save did not persist the Pal level and EXP")
    health_field, hp, max_hp, has_max_hp = _read_health(record)
    if health_field != result.health_field:
        raise PalEditError("Rebuilt save changed the Pal health field casing")
    if not has_max_hp or (hp, max_hp) != (
        result.hp_after,
        result.max_hp_after,
    ):
        raise PalEditError("Rebuilt save did not persist the Pal health values")
    if record != result.expected_record:
        raise PalEditError("Rebuilt save changed unexpected Pal fields")
    if (
        _structure_digest(_character_entries(_world(level_gvas)))
        != result.expected_character_entries_digest
    ):
        raise PalEditError("Rebuilt save changed other character records")


def restore_pal_health(
    level_gvas: Any,
    player_uid: str,
    instance_id: str,
    expected_nickname: str,
    expected_level: int,
    expected_exp: int,
    expected_hp: int,
    expected_max_hp: int,
) -> PalHealthResult:
    expected_nickname = _validate_nickname(expected_nickname, "Expected Pal nickname")
    expected_level = _validate_level(expected_level, "Expected Pal level")
    expected_exp = _require_int(expected_exp, "Expected Pal EXP")
    expected_hp = _require_int(expected_hp, "Expected Pal HP")
    expected_max_hp = _require_int(expected_max_hp, "Expected Pal MaxHP")
    if expected_max_hp <= 0:
        raise PalEditError("Expected Pal MaxHP must be positive")

    canonical_instance_id = _canonical_instance_id(instance_id)
    canonical_player_uid = _canonical_player_uid(player_uid)
    records = _pal_records(_world(level_gvas), canonical_instance_id)
    if len(records) != 1:
        raise PalConflictError(
            f"Expected exactly one Pal record for instance {canonical_instance_id}, "
            f"found {len(records)}"
        )
    record = records[0]
    if _read_owner(record) != canonical_player_uid:
        raise PalConflictError("Pal owner changed before the edit")

    nickname, pal_type, level, exp = _read_profile(record)
    if nickname != expected_nickname:
        raise PalConflictError(
            f"Pal nickname changed from {expected_nickname!r} to {nickname!r}"
        )
    if level != expected_level:
        raise PalConflictError(f"Pal level changed from {expected_level} to {level}")
    if exp != expected_exp:
        raise PalConflictError(f"Pal EXP changed from {expected_exp} to {exp}")

    health_field, hp_before, max_hp, has_max_hp = _read_health(record)
    if not has_max_hp:
        raise PalConflictError("Pal record is missing MaxHP")
    if hp_before != expected_hp:
        raise PalConflictError(f"Pal HP changed from {expected_hp} to {hp_before}")
    if max_hp != expected_max_hp:
        raise PalConflictError(
            f"Pal MaxHP changed from {expected_max_hp} to {max_hp}"
        )
    if max_hp <= 0:
        raise PalEditError("Stored Pal MaxHP must be positive")
    if hp_before == max_hp:
        raise PalEditError("Pal health is already full")
    if hp_before > max_hp:
        raise PalEditError("Stored Pal HP cannot exceed MaxHP")

    updated_record = copy.deepcopy(record)
    _set_fixed_point_value(updated_record[health_field], max_hp)
    expected_record = copy.deepcopy(updated_record)
    record.clear()
    record.update(updated_record)
    expected_character_entries_digest = _structure_digest(
        _character_entries(_world(level_gvas))
    )

    return PalHealthResult(
        player_uid=player_uid,
        instance_id=canonical_instance_id,
        pal_type=pal_type,
        nickname=nickname,
        level=level,
        exp=exp,
        hp_before=hp_before,
        hp_after=max_hp,
        max_hp=max_hp,
        health_field=health_field,
        expected_record=expected_record,
        expected_character_entries_digest=expected_character_entries_digest,
    )


def verify_pal_health(level_gvas: Any, result: PalHealthResult) -> None:
    records = _pal_records(_world(level_gvas), result.instance_id)
    if len(records) != 1:
        raise PalEditError("Rebuilt save changed the Pal record count")
    record = records[0]
    if _read_owner(record) != _canonical_player_uid(result.player_uid):
        raise PalEditError("Rebuilt save changed the Pal owner")
    nickname, pal_type, level, exp = _read_profile(record)
    if (nickname, pal_type, level, exp) != (
        result.nickname,
        result.pal_type,
        result.level,
        result.exp,
    ):
        raise PalEditError("Rebuilt save changed the Pal identity")
    health_field, hp, max_hp, has_max_hp = _read_health(record)
    if health_field != result.health_field:
        raise PalEditError("Rebuilt save changed the Pal health field casing")
    if not has_max_hp or (hp, max_hp) != (result.hp_after, result.max_hp):
        raise PalEditError("Rebuilt save did not persist the restored Pal health")
    if record != result.expected_record:
        raise PalEditError("Rebuilt save changed unexpected Pal fields")
    if (
        _structure_digest(_character_entries(_world(level_gvas)))
        != result.expected_character_entries_digest
    ):
        raise PalEditError("Rebuilt save changed other character records")
