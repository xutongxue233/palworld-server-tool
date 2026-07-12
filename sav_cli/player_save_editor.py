from __future__ import annotations

import copy
import hashlib
import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from uuid import UUID


MAX_TECHNOLOGY_POINTS = 999_999
TECHNOLOGY_FIELD = "TechnologyPoint"
ANCIENT_TECHNOLOGY_FIELD = "bossTechnologyPoint"
PLAYER_MAP_METADATA_SCHEMA = 1
PLAYER_MAP_GAME_VERSION = "1.0.0"
PLAYER_MAP_SOURCE_COMMIT = "2cb6fd963120b002f0732dad153786e624f64b38"
FAST_TRAVEL_FIELD = "FastTravelPointUnlockFlag"
AREA_FIELD = "FindAreaFlagMap"
WORLD_MAP_FIELD = "UnlockedWorldMapFlags"
EXPECTED_FAST_TRAVEL_COUNT = 174
EXPECTED_AREA_COUNT = 123
WORLD_MAP_FLAGS = ("MainMap", "Tree")


class PlayerSaveEditError(ValueError):
    pass


class PlayerSaveConflictError(PlayerSaveEditError):
    pass


@dataclass(frozen=True)
class PlayerMapMetadata:
    source_commit: str
    game_version: str
    fast_travel_guids: tuple[str, ...]
    areas: tuple[str, ...]
    world_flags: tuple[str, ...]


@dataclass(frozen=True)
class PlayerMapProgress:
    fast_travel_unlocked: int
    fast_travel_total: int
    areas_found: int
    areas_total: int
    world_maps_unlocked: int
    world_maps_total: int
    progress_digest: str
    game_version: str

    def to_dict(self) -> dict[str, Any]:
        return {
            "fast_travel_unlocked": self.fast_travel_unlocked,
            "fast_travel_total": self.fast_travel_total,
            "areas_found": self.areas_found,
            "areas_total": self.areas_total,
            "world_maps_unlocked": self.world_maps_unlocked,
            "world_maps_total": self.world_maps_total,
            "progress_digest": self.progress_digest,
            "game_version": self.game_version,
        }


@dataclass(frozen=True)
class PlayerMapProgressResult:
    player_uid: str
    fast_travel_before: int
    fast_travel_after: int
    fast_travel_total: int
    areas_before: int
    areas_after: int
    areas_total: int
    world_maps_before: int
    world_maps_after: int
    world_maps_total: int
    progress_digest_before: str
    progress_digest_after: str
    game_version: str
    created_fields: tuple[str, ...]
    expected_save_data_digest: str = field(repr=False, compare=False)
    expected_non_target_digest: str = field(repr=False, compare=False)

    def to_dict(self) -> dict[str, Any]:
        return {
            "player_uid": self.player_uid,
            "fast_travel_before": self.fast_travel_before,
            "fast_travel_after": self.fast_travel_after,
            "fast_travel_total": self.fast_travel_total,
            "areas_before": self.areas_before,
            "areas_after": self.areas_after,
            "areas_total": self.areas_total,
            "world_maps_before": self.world_maps_before,
            "world_maps_after": self.world_maps_after,
            "world_maps_total": self.world_maps_total,
            "progress_digest_before": self.progress_digest_before,
            "progress_digest_after": self.progress_digest_after,
            "game_version": self.game_version,
            "created_fields": list(self.created_fields),
        }


@dataclass(frozen=True)
class PlayerTechnologyPointsResult:
    player_uid: str
    technology_before: int
    technology_after: int
    ancient_before: int
    ancient_after: int
    created_fields: tuple[str, ...]

    def to_dict(self) -> dict[str, Any]:
        return {
            "player_uid": self.player_uid,
            "technology_before": self.technology_before,
            "technology_after": self.technology_after,
            "ancient_before": self.ancient_before,
            "ancient_after": self.ancient_after,
            "created_fields": list(self.created_fields),
        }


def _properties(gvas_or_properties: Any) -> dict[str, Any]:
    if hasattr(gvas_or_properties, "properties"):
        return gvas_or_properties.properties
    if isinstance(gvas_or_properties, dict):
        return gvas_or_properties.get("properties", gvas_or_properties)
    raise PlayerSaveEditError("Unsupported player GVAS data object")


def _save_data(player_gvas: Any) -> dict[str, Any]:
    try:
        save_data = _properties(player_gvas)["SaveData"]
        if save_data.get("struct_type") != "PalWorldPlayerSaveData":
            raise PlayerSaveEditError(
                "Player save does not contain PalWorldPlayerSaveData"
            )
        value = save_data["value"]
    except (KeyError, TypeError) as exc:
        raise PlayerSaveEditError("Player save does not contain SaveData") from exc
    if not isinstance(value, dict):
        raise PlayerSaveEditError("Player SaveData is invalid")
    return value


def _normalize_uid(value: Any) -> str:
    return str(value).replace("-", "").lower()


def _canonical_player_uid(value: Any) -> str:
    raw = _normalize_uid(value).strip()
    if not raw:
        raise PlayerSaveEditError("Player UID cannot be empty")
    if raw.isdecimal() and len(raw) <= 10:
        numeric_uid = int(raw)
        if numeric_uid > 0xFFFFFFFF:
            raise PlayerSaveEditError(f"Invalid decimal player UID: {value}")
        return f"{numeric_uid:08x}" + ("0" * 24)
    if len(raw) == 8:
        raw += "0" * 24
    if len(raw) != 32 or any(char not in "0123456789abcdef" for char in raw):
        raise PlayerSaveEditError(f"Invalid player UID: {value}")
    return raw


def _read_player_uid(save_data: dict[str, Any]) -> str:
    try:
        value = save_data["PlayerUId"]
        if value.get("type") != "StructProperty" or value.get("struct_type") != "Guid":
            raise PlayerSaveEditError("PlayerUId must be a Guid StructProperty")
        return _normalize_uid(value["value"])
    except (KeyError, TypeError) as exc:
        raise PlayerSaveEditError("Player save does not contain PlayerUId") from exc


def _canonical_metadata_guid(value: Any, label: str) -> str:
    if not isinstance(value, str) or not value or value != value.strip():
        raise PlayerSaveEditError(f"{label} must be a canonical GUID string")
    try:
        canonical = UUID(value).hex.upper()
    except (AttributeError, TypeError, ValueError) as exc:
        raise PlayerSaveEditError(f"Invalid {label}: {value}") from exc
    if value != canonical or canonical == "0" * 32:
        raise PlayerSaveEditError(f"{label} must be a non-zero uppercase GUID")
    return canonical


def _validate_fast_travel_guids(values: Any) -> tuple[str, ...]:
    if not isinstance(values, (list, tuple)):
        raise PlayerSaveEditError("Player map metadata is missing fast travel GUIDs")
    guids: list[str] = []
    seen: set[str] = set()
    for value in values:
        guid = _canonical_metadata_guid(value, "fast travel GUID")
        if guid in seen:
            raise PlayerSaveEditError(
                f"Player map metadata duplicates fast travel GUID {guid}"
            )
        seen.add(guid)
        guids.append(guid)
    if len(guids) != EXPECTED_FAST_TRAVEL_COUNT:
        raise PlayerSaveEditError(
            "Player map metadata must contain exactly "
            f"{EXPECTED_FAST_TRAVEL_COUNT} fast travel GUIDs"
        )
    return tuple(guids)


def _validate_area_ids(values: Any) -> tuple[str, ...]:
    if not isinstance(values, (list, tuple)):
        raise PlayerSaveEditError("Player map metadata is missing area IDs")
    areas: list[str] = []
    seen: set[str] = set()
    for value in values:
        if not isinstance(value, str) or not value or value != value.strip():
            raise PlayerSaveEditError(
                "Player map metadata contains an invalid area ID"
            )
        if value in seen:
            raise PlayerSaveEditError(
                f"Player map metadata duplicates area ID {value}"
            )
        seen.add(value)
        areas.append(value)
    if len(areas) != EXPECTED_AREA_COUNT:
        raise PlayerSaveEditError(
            "Player map metadata must contain exactly "
            f"{EXPECTED_AREA_COUNT} area IDs"
        )
    return tuple(areas)


def _validate_world_flags(values: Any) -> tuple[str, ...]:
    if not isinstance(values, (list, tuple)):
        raise PlayerSaveEditError("Player map metadata is missing world map flags")
    flags = tuple(values)
    if flags != WORLD_MAP_FLAGS:
        raise PlayerSaveEditError(
            "Player map metadata world flags must be MainMap and Tree"
        )
    return flags


def _validate_player_map_metadata(metadata: PlayerMapMetadata) -> None:
    if not isinstance(metadata, PlayerMapMetadata):
        raise PlayerSaveEditError("Invalid player map metadata object")
    if metadata.source_commit != PLAYER_MAP_SOURCE_COMMIT:
        raise PlayerSaveEditError("Unsupported player map metadata source commit")
    if metadata.game_version != PLAYER_MAP_GAME_VERSION:
        raise PlayerSaveEditError("Unsupported player map metadata game version")
    _validate_fast_travel_guids(metadata.fast_travel_guids)
    _validate_area_ids(metadata.areas)
    _validate_world_flags(metadata.world_flags)


def load_player_map_metadata(path: str) -> PlayerMapMetadata:
    metadata_path = Path(path)
    if not metadata_path.is_file():
        raise PlayerSaveEditError(
            f"Player map metadata does not exist: {metadata_path}"
        )
    try:
        payload = json.loads(metadata_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise PlayerSaveEditError(
            f"Unable to read player map metadata: {metadata_path}"
        ) from exc
    if not isinstance(payload, dict):
        raise PlayerSaveEditError("Player map metadata must contain a JSON object")
    if payload.get("schema") != PLAYER_MAP_METADATA_SCHEMA:
        raise PlayerSaveEditError("Unsupported player map metadata schema")
    source_commit = payload.get("source_commit")
    game_version = payload.get("game_version")
    if source_commit != PLAYER_MAP_SOURCE_COMMIT:
        raise PlayerSaveEditError("Unsupported player map metadata source commit")
    if game_version != PLAYER_MAP_GAME_VERSION:
        raise PlayerSaveEditError("Unsupported player map metadata game version")
    metadata = PlayerMapMetadata(
        source_commit=source_commit,
        game_version=game_version,
        fast_travel_guids=_validate_fast_travel_guids(
            payload.get("fast_travel_guids")
        ),
        areas=_validate_area_ids(payload.get("areas")),
        world_flags=_validate_world_flags(payload.get("world_flags")),
    )
    _validate_player_map_metadata(metadata)
    return metadata


def _record_data(save_data: dict[str, Any]) -> dict[str, Any]:
    record_property = save_data.get("RecordData")
    if not isinstance(record_property, dict):
        raise PlayerSaveEditError("Player save does not contain RecordData")
    if (
        record_property.get("type") != "StructProperty"
        or record_property.get("struct_type")
        != "PalLoggedinPlayerSaveDataRecordData"
    ):
        raise PlayerSaveEditError(
            "Player RecordData must be a PalLoggedinPlayerSaveDataRecordData StructProperty"
        )
    record_data = record_property.get("value")
    if not isinstance(record_data, dict):
        raise PlayerSaveEditError("Player RecordData is invalid")
    return record_data


def _map_key_identity(field: str, key: str) -> str:
    if field != FAST_TRAVEL_FIELD:
        return "name:" + key
    try:
        return "guid:" + UUID(key).hex.upper()
    except (AttributeError, TypeError, ValueError):
        return "name:" + key


def _read_map_entries(
    record_data: dict[str, Any], field: str
) -> tuple[dict[str, Any] | None, list[dict[str, Any]], dict[str, dict[str, Any]]]:
    map_property = record_data.get(field)
    if map_property is None:
        return None, [], {}
    if not isinstance(map_property, dict) or map_property.get("type") != "MapProperty":
        raise PlayerSaveEditError(f"Player {field} must be a MapProperty")
    if (
        map_property.get("key_type") != "NameProperty"
        or map_property.get("value_type") != "BoolProperty"
        or "key_struct_type" not in map_property
        or map_property.get("key_struct_type") is not None
        or "value_struct_type" not in map_property
        or map_property.get("value_struct_type") is not None
    ):
        raise PlayerSaveEditError(
            f"Player {field} must map NameProperty keys to BoolProperty values"
        )
    entries = map_property.get("value")
    if not isinstance(entries, list):
        raise PlayerSaveEditError(f"Player {field} entries must be an array")
    by_identity: dict[str, dict[str, Any]] = {}
    for entry in entries:
        if not isinstance(entry, dict):
            raise PlayerSaveEditError(f"Player {field} contains an invalid entry")
        key = entry.get("key")
        value = entry.get("value")
        if not isinstance(key, str) or not key:
            raise PlayerSaveEditError(f"Player {field} contains an invalid key")
        if not isinstance(value, bool):
            raise PlayerSaveEditError(
                f"Player {field} contains a non-boolean value"
            )
        identity = _map_key_identity(field, key)
        if identity in by_identity:
            raise PlayerSaveEditError(
                f"Player {field} contains duplicate key {key}"
            )
        by_identity[identity] = entry
    return map_property, entries, by_identity


def _normalized_map_values(
    record_data: dict[str, Any], field: str
) -> list[list[Any]]:
    _, _, entries = _read_map_entries(record_data, field)
    return [
        [identity, entries[identity]["value"]]
        for identity in sorted(entries)
    ]


def _progress_digest(
    record_data: dict[str, Any], metadata: PlayerMapMetadata
) -> str:
    payload = {
        "schema": PLAYER_MAP_METADATA_SCHEMA,
        "game_version": metadata.game_version,
        "targets": {
            "fast_travel": list(metadata.fast_travel_guids),
            "areas": list(metadata.areas),
            "world_maps": list(metadata.world_flags),
        },
        "progress": {
            FAST_TRAVEL_FIELD: _normalized_map_values(
                record_data, FAST_TRAVEL_FIELD
            ),
            AREA_FIELD: _normalized_map_values(record_data, AREA_FIELD),
            WORLD_MAP_FIELD: _normalized_map_values(
                record_data, WORLD_MAP_FIELD
            ),
        },
    }
    normalized = json.dumps(
        payload,
        ensure_ascii=True,
        separators=(",", ":"),
        sort_keys=True,
    ).encode("utf-8")
    return hashlib.sha256(normalized).hexdigest()


def _read_player_map_progress_from_save_data(
    save_data: dict[str, Any], metadata: PlayerMapMetadata
) -> PlayerMapProgress:
    record_data = _record_data(save_data)
    _, _, fast_travel_entries = _read_map_entries(
        record_data, FAST_TRAVEL_FIELD
    )
    _, _, area_entries = _read_map_entries(record_data, AREA_FIELD)
    _, _, world_map_entries = _read_map_entries(record_data, WORLD_MAP_FIELD)
    fast_travel_unlocked = sum(
        fast_travel_entries.get("guid:" + guid, {}).get("value") is True
        for guid in metadata.fast_travel_guids
    )
    areas_found = sum(
        area_entries.get("name:" + area, {}).get("value") is True
        for area in metadata.areas
    )
    world_maps_unlocked = sum(
        world_map_entries.get("name:" + flag, {}).get("value") is True
        for flag in metadata.world_flags
    )
    return PlayerMapProgress(
        fast_travel_unlocked=fast_travel_unlocked,
        fast_travel_total=len(metadata.fast_travel_guids),
        areas_found=areas_found,
        areas_total=len(metadata.areas),
        world_maps_unlocked=world_maps_unlocked,
        world_maps_total=len(metadata.world_flags),
        progress_digest=_progress_digest(record_data, metadata),
        game_version=metadata.game_version,
    )


def read_player_map_progress(
    player_gvas: Any, metadata: PlayerMapMetadata
) -> PlayerMapProgress:
    _validate_player_map_metadata(metadata)
    save_data = _save_data(player_gvas)
    _read_player_uid(save_data)
    return _read_player_map_progress_from_save_data(save_data, metadata)


def _validate_progress_digest(value: Any) -> str:
    if not isinstance(value, str) or len(value) != 64:
        raise PlayerSaveEditError("Expected player map progress digest is invalid")
    normalized = value.lower()
    if any(character not in "0123456789abcdef" for character in normalized):
        raise PlayerSaveEditError("Expected player map progress digest is invalid")
    return normalized


def _standard_map_property() -> dict[str, Any]:
    return {
        "key_type": "NameProperty",
        "value_type": "BoolProperty",
        "key_struct_type": None,
        "value_struct_type": None,
        "id": None,
        "value": [],
        "type": "MapProperty",
    }


def _unlock_map_entries(
    record_data: dict[str, Any],
    field: str,
    target_keys: tuple[str, ...],
    created_fields: list[str],
) -> None:
    if field not in record_data:
        record_data[field] = _standard_map_property()
        created_fields.append(field)
    _, entries, by_identity = _read_map_entries(record_data, field)
    for key in target_keys:
        identity = _map_key_identity(field, key)
        existing = by_identity.get(identity)
        if existing is None:
            entry = {"key": key, "value": True}
            entries.append(entry)
            by_identity[identity] = entry
        else:
            existing["value"] = True


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


def _non_target_digest(save_data: dict[str, Any]) -> str:
    non_target = copy.deepcopy(save_data)
    record_data = _record_data(non_target)
    for field_name in (FAST_TRAVEL_FIELD, AREA_FIELD, WORLD_MAP_FIELD):
        record_data.pop(field_name, None)
    return _structure_digest(non_target)


def _is_fully_unlocked(progress: PlayerMapProgress) -> bool:
    return (
        progress.fast_travel_unlocked == progress.fast_travel_total
        and progress.areas_found == progress.areas_total
        and progress.world_maps_unlocked == progress.world_maps_total
    )


def unlock_player_map(
    player_gvas: Any,
    player_uid: str,
    metadata: PlayerMapMetadata,
    expected_progress_digest: str,
) -> PlayerMapProgressResult:
    _validate_player_map_metadata(metadata)
    expected_digest = _validate_progress_digest(expected_progress_digest)
    save_data = _save_data(player_gvas)
    target_uid = _canonical_player_uid(player_uid)
    stored_uid = _read_player_uid(save_data)
    if stored_uid != target_uid:
        raise PlayerSaveEditError(
            f"Player save UID {stored_uid} does not match target {target_uid}"
        )

    before = _read_player_map_progress_from_save_data(save_data, metadata)
    if before.progress_digest != expected_digest:
        raise PlayerSaveConflictError(
            "Player map progress changed from digest "
            f"{expected_digest} to {before.progress_digest}"
        )
    if _is_fully_unlocked(before):
        raise PlayerSaveEditError("Player map is already fully unlocked")

    expected_non_target_digest = _non_target_digest(save_data)
    updated_save_data = copy.deepcopy(save_data)
    updated_record_data = _record_data(updated_save_data)
    created_fields: list[str] = []
    _unlock_map_entries(
        updated_record_data,
        FAST_TRAVEL_FIELD,
        metadata.fast_travel_guids,
        created_fields,
    )
    _unlock_map_entries(
        updated_record_data,
        AREA_FIELD,
        metadata.areas,
        created_fields,
    )
    _unlock_map_entries(
        updated_record_data,
        WORLD_MAP_FIELD,
        metadata.world_flags,
        created_fields,
    )
    after = _read_player_map_progress_from_save_data(updated_save_data, metadata)
    if not _is_fully_unlocked(after):
        raise PlayerSaveEditError("Player map unlock did not reach full progress")
    if _non_target_digest(updated_save_data) != expected_non_target_digest:
        raise PlayerSaveEditError("Player map unlock changed non-target save data")
    expected_save_data_digest = _structure_digest(updated_save_data)

    save_data.clear()
    save_data.update(updated_save_data)
    return PlayerMapProgressResult(
        player_uid=str(player_uid),
        fast_travel_before=before.fast_travel_unlocked,
        fast_travel_after=after.fast_travel_unlocked,
        fast_travel_total=after.fast_travel_total,
        areas_before=before.areas_found,
        areas_after=after.areas_found,
        areas_total=after.areas_total,
        world_maps_before=before.world_maps_unlocked,
        world_maps_after=after.world_maps_unlocked,
        world_maps_total=after.world_maps_total,
        progress_digest_before=before.progress_digest,
        progress_digest_after=after.progress_digest,
        game_version=metadata.game_version,
        created_fields=tuple(created_fields),
        expected_save_data_digest=expected_save_data_digest,
        expected_non_target_digest=expected_non_target_digest,
    )


def verify_player_map_unlock(
    player_gvas: Any,
    metadata: PlayerMapMetadata,
    result: PlayerMapProgressResult,
) -> None:
    _validate_player_map_metadata(metadata)
    if not isinstance(result, PlayerMapProgressResult):
        raise PlayerSaveEditError("Invalid player map unlock result")
    if (
        result.game_version != metadata.game_version
        or result.fast_travel_total != len(metadata.fast_travel_guids)
        or result.areas_total != len(metadata.areas)
        or result.world_maps_total != len(metadata.world_flags)
    ):
        raise PlayerSaveEditError("Player map unlock result metadata changed")

    save_data = _save_data(player_gvas)
    if _read_player_uid(save_data) != _canonical_player_uid(result.player_uid):
        raise PlayerSaveEditError("Rebuilt player save changed PlayerUId")
    observed = _read_player_map_progress_from_save_data(save_data, metadata)
    if (
        observed.fast_travel_unlocked != result.fast_travel_after
        or observed.areas_found != result.areas_after
        or observed.world_maps_unlocked != result.world_maps_after
        or observed.progress_digest != result.progress_digest_after
    ):
        raise PlayerSaveEditError(
            "Rebuilt player save did not persist full map progress"
        )
    if _non_target_digest(save_data) != result.expected_non_target_digest:
        raise PlayerSaveEditError(
            "Rebuilt player save changed data outside map progress"
        )
    if _structure_digest(save_data) != result.expected_save_data_digest:
        raise PlayerSaveEditError(
            "Rebuilt player save changed the expected map progress structure"
        )


def _validate_points(value: Any, label: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise PlayerSaveEditError(f"{label} must be an integer")
    if value < 0 or value > MAX_TECHNOLOGY_POINTS:
        raise PlayerSaveEditError(
            f"{label} must be between 0 and {MAX_TECHNOLOGY_POINTS}"
        )
    return value


def _read_points(save_data: dict[str, Any], field: str, label: str) -> int:
    property_value = save_data.get(field)
    if property_value is None:
        return 0
    if not isinstance(property_value, dict) or property_value.get("type") != "IntProperty":
        raise PlayerSaveEditError(f"{field} must be an IntProperty")
    if "value" not in property_value:
        raise PlayerSaveEditError(f"{field} is missing its value")
    return _validate_points(property_value["value"], f"Stored {label}")


def _write_points(
    save_data: dict[str, Any], field: str, value: int, created_fields: list[str]
) -> None:
    property_value = save_data.get(field)
    if property_value is None:
        if value == 0:
            return
        save_data[field] = {"id": None, "value": value, "type": "IntProperty"}
        created_fields.append(field)
        return
    property_value["value"] = value


def set_player_technology_points(
    player_gvas: Any,
    player_uid: str,
    expected_technology_points: int,
    expected_ancient_technology_points: int,
    technology_points: int,
    ancient_technology_points: int,
) -> PlayerTechnologyPointsResult:
    expected_technology_points = _validate_points(
        expected_technology_points, "Expected technology points"
    )
    expected_ancient_technology_points = _validate_points(
        expected_ancient_technology_points, "Expected ancient technology points"
    )
    technology_points = _validate_points(technology_points, "Technology points")
    ancient_technology_points = _validate_points(
        ancient_technology_points, "Ancient technology points"
    )

    save_data = _save_data(player_gvas)
    target_uid = _canonical_player_uid(player_uid)
    stored_uid = _read_player_uid(save_data)
    if stored_uid != target_uid:
        raise PlayerSaveEditError(
            f"Player save UID {stored_uid} does not match target {target_uid}"
        )

    technology_before = _read_points(
        save_data, TECHNOLOGY_FIELD, "technology points"
    )
    ancient_before = _read_points(
        save_data, ANCIENT_TECHNOLOGY_FIELD, "ancient technology points"
    )
    if technology_before != expected_technology_points:
        raise PlayerSaveConflictError(
            "Player technology points changed from "
            f"{expected_technology_points} to {technology_before}"
        )
    if ancient_before != expected_ancient_technology_points:
        raise PlayerSaveConflictError(
            "Player ancient technology points changed from "
            f"{expected_ancient_technology_points} to {ancient_before}"
        )
    if (
        technology_before == technology_points
        and ancient_before == ancient_technology_points
    ):
        raise PlayerSaveEditError("Player technology points are unchanged")

    created_fields: list[str] = []
    _write_points(save_data, TECHNOLOGY_FIELD, technology_points, created_fields)
    _write_points(
        save_data,
        ANCIENT_TECHNOLOGY_FIELD,
        ancient_technology_points,
        created_fields,
    )
    return PlayerTechnologyPointsResult(
        player_uid=player_uid,
        technology_before=technology_before,
        technology_after=technology_points,
        ancient_before=ancient_before,
        ancient_after=ancient_technology_points,
        created_fields=tuple(created_fields),
    )


def verify_player_technology_points(
    player_gvas: Any, result: PlayerTechnologyPointsResult
) -> None:
    save_data = _save_data(player_gvas)
    if _read_player_uid(save_data) != _canonical_player_uid(result.player_uid):
        raise PlayerSaveEditError("Rebuilt player save changed PlayerUId")
    observed_technology = _read_points(
        save_data, TECHNOLOGY_FIELD, "technology points"
    )
    observed_ancient = _read_points(
        save_data, ANCIENT_TECHNOLOGY_FIELD, "ancient technology points"
    )
    if observed_technology != result.technology_after:
        raise PlayerSaveEditError(
            "Rebuilt player save did not persist technology points"
        )
    if observed_ancient != result.ancient_after:
        raise PlayerSaveEditError(
            "Rebuilt player save did not persist ancient technology points"
        )
    for field in result.created_fields:
        if save_data.get(field, {}).get("type") != "IntProperty":
            raise PlayerSaveEditError(
                f"Rebuilt player save did not preserve {field} as IntProperty"
            )
