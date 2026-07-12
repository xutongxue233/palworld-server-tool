import argparse
import json
import os
import shutil
import sys
import tempfile
import time
import traceback
from pathlib import Path
from urllib.parse import urljoin

import requests
from palsav.commands.convert import convert_json_to_sav, convert_sav_to_json
from palsav.core import compress_gvas_to_sav, decompress_sav_to_gvas
from palsav.gvas import GvasFile
from palsav.paltypes import PALWORLD_CUSTOM_PROPERTIES, PALWORLD_TYPE_HINTS

from logger import log
from inventory_editor import (
    InventoryConflictError,
    InventoryEditError,
    deliver_item,
    get_item_definition,
    load_item_catalog,
    resolve_player_save,
    set_item_quantity,
    verify_delivery,
    verify_inventory_mutation,
)
from player_editor import (
    PlayerConflictError,
    PlayerEditError,
    load_exp_table,
    set_player_profile,
    set_player_stat_points,
    verify_player_profile,
    verify_player_stat_points,
)
from player_save_editor import (
    PlayerSaveConflictError,
    PlayerSaveEditError,
    load_player_map_metadata,
    set_player_technology_points,
    unlock_player_map,
    verify_player_map_unlock,
    verify_player_technology_points,
)
from pal_editor import (
    PalConflictError,
    PalEditError,
    load_pal_exp_table,
    load_pal_level_metadata,
    rename_pal,
    restore_pal_health,
    set_pal_level,
    verify_pal_health,
    verify_pal_level,
    verify_pal_nickname,
)
from structurer import (
    convert_sav,
    skip_decode,
    skip_encode,
    structure_guild,
    structure_player,
)


PAL_CHARACTER_RAW_PATH = ".worldSaveData.CharacterSaveParameterMap.Value.RawData"


def _encode_skipped_property(writer, property_type, properties):
    return skip_encode(writer, property_type, dict(properties))


PAL_EDIT_CUSTOM_PROPERTIES = dict(PALWORLD_CUSTOM_PROPERTIES)
for _property_path in PAL_EDIT_CUSTOM_PROPERTIES:
    if _property_path != PAL_CHARACTER_RAW_PATH:
        PAL_EDIT_CUSTOM_PROPERTIES[_property_path] = (
            skip_decode,
            _encode_skipped_property,
        )


def _write_temporary_save(output_path: Path, content: bytes) -> Path:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    file_descriptor, temporary_name = tempfile.mkstemp(
        prefix=f".{output_path.name}.",
        suffix=".tmp",
        dir=output_path.parent,
    )
    temporary_path = Path(temporary_name)
    try:
        with os.fdopen(file_descriptor, "wb") as temporary_file:
            temporary_file.write(content)
            temporary_file.flush()
            os.fsync(temporary_file.fileno())
    except Exception:
        temporary_path.unlink(missing_ok=True)
        raise
    return temporary_path


def structure_save(file_path: Path, output: str, request: str, token: str) -> None:
    convert_sav(str(file_path))
    filetime = file_path.stat().st_mtime
    player_dir = file_path.parent / "Players"

    players = structure_player(str(player_dir), filetime=filetime)
    guilds = structure_guild(filetime)

    for player in players:
        for guild in guilds:
            for guild_player in guild["players"]:
                if player["player_uid"] == guild_player["player_uid"]:
                    player["save_last_online"] = guild_player["last_online"]
                    break

    if not request:
        output_path = Path(output or "structure.json")
        if output_path.suffix.lower() != ".json":
            output_path = output_path.with_suffix(output_path.suffix + ".json")
        output_path.write_text(
            json.dumps(
                {"players": players, "guilds": guilds},
                indent=2,
                ensure_ascii=False,
            ),
            encoding="utf-8",
        )
        log(f"Players: {len(players)}")
        log(f"Guilds: {len(guilds)}")
        log(f"Structured data written to {output_path}")
        return

    headers = {"Authorization": f"Bearer {token}"}
    player_url = urljoin(request, "player")
    guild_url = urljoin(request, "guild")

    log(f"Put players to {player_url} with Players: {len(players)}")
    player_response = requests.put(
        player_url,
        headers=headers,
        json=players,
        timeout=30,
    )
    player_response.raise_for_status()

    log(f"Put guilds to {guild_url} with Guilds: {len(guilds)}")
    guild_response = requests.put(
        guild_url,
        headers=headers,
        json=guilds,
        timeout=30,
    )
    guild_response.raise_for_status()


def load_gvas(
    file_path: Path,
    custom_properties=PALWORLD_CUSTOM_PROPERTIES,
) -> tuple[bytes, int, GvasFile]:
    raw_gvas, save_type = decompress_sav_to_gvas(file_path.read_bytes())
    gvas = GvasFile.read(
        raw_gvas,
        type_hints=PALWORLD_TYPE_HINTS,
        custom_properties=custom_properties,
    )
    return raw_gvas, save_type, gvas


def require_lossless_gvas_roundtrip(
    raw_gvas: bytes,
    gvas: GvasFile,
    mismatch_message: str,
    custom_properties=PALWORLD_CUSTOM_PROPERTIES,
) -> bytes:
    written = gvas.write(custom_properties=custom_properties)
    if written != raw_gvas:
        raise ValueError(mismatch_message)
    return written


def load_level_for_edit(file_path: Path) -> tuple[int, GvasFile]:
    raw_gvas, save_type, gvas = load_gvas(file_path)
    require_lossless_gvas_roundtrip(
        raw_gvas,
        gvas,
        "GVAS changed during the edit preflight validation pass",
    )
    return save_type, gvas


def load_level_for_pal_edit(file_path: Path) -> tuple[int, GvasFile]:
    raw_gvas, save_type, gvas = load_gvas(file_path, PAL_EDIT_CUSTOM_PROPERTIES)
    require_lossless_gvas_roundtrip(
        raw_gvas,
        gvas,
        "GVAS changed during the Pal edit preflight validation pass",
        PAL_EDIT_CUSTOM_PROPERTIES,
    )
    return save_type, gvas


def validate_save(file_path: Path) -> None:
    raw_gvas, save_type, gvas = load_gvas(file_path)
    require_lossless_gvas_roundtrip(
        raw_gvas,
        gvas,
        "GVAS changed during a read/write validation pass",
    )
    log(
        f"Valid save: class={gvas.header.save_game_class_name}, "
        f"type=0x{save_type:02X}, raw_bytes={len(raw_gvas)}"
    )


def roundtrip_save(file_path: Path, output: str) -> None:
    raw_gvas, save_type, gvas = load_gvas(file_path)
    written = require_lossless_gvas_roundtrip(
        raw_gvas,
        gvas,
        "GVAS changed before recompression",
    )

    output_path = Path(output or f"{file_path}.roundtrip.sav")
    if output_path.resolve() == file_path.resolve():
        raise ValueError("Roundtrip output must not overwrite the input save")

    rebuilt = compress_gvas_to_sav(written, save_type)
    output_path.write_bytes(rebuilt)
    rebuilt_raw, rebuilt_type, _ = load_gvas(output_path)
    if rebuilt_type != save_type or rebuilt_raw != raw_gvas:
        output_path.unlink(missing_ok=True)
        raise ValueError("Roundtrip validation failed after recompression")

    log(f"Roundtrip save written to {output_path}")


def export_json(file_path: Path, output: str) -> None:
    output_path = Path(output or f"{file_path}.json")
    convert_sav_to_json(
        str(file_path),
        str(output_path),
        force=True,
        minify=False,
        custom_properties_keys=["all"],
    )
    log(f"Editable JSON written to {output_path}")


def rebuild_json(file_path: Path, output: str) -> None:
    output_path = Path(output or file_path.with_suffix(".sav"))
    if output_path.resolve() == file_path.resolve():
        raise ValueError("Rebuild output must not overwrite the input JSON")

    convert_json_to_sav(str(file_path), str(output_path), force=True)
    validate_save(output_path)
    log(f"Rebuilt save written to {output_path}")


def give_item(
    file_path: Path,
    output: str,
    player_uid: str,
    item_id: str,
    quantity: int,
    container: str,
    metadata: str,
) -> None:
    if not player_uid:
        raise ValueError("--player-uid is required for give-item mode")
    if not item_id:
        raise ValueError("--item-id is required for give-item mode")

    output_path = Path(output or f"{file_path}.give-item.sav")
    if output_path.resolve() == file_path.resolve():
        raise ValueError("Give-item output must not overwrite the input save")

    save_type, level_gvas = load_level_for_edit(file_path)
    player_path = resolve_player_save(file_path, player_uid)
    _, _, player_gvas = load_gvas(player_path)
    catalog = load_item_catalog(metadata)
    definition = get_item_definition(catalog, item_id)
    result = deliver_item(
        level_gvas,
        player_gvas,
        player_uid,
        definition,
        quantity,
        container,
    )

    written = level_gvas.write(custom_properties=PALWORLD_CUSTOM_PROPERTIES)
    rebuilt = compress_gvas_to_sav(written, save_type)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_bytes(rebuilt)
    try:
        validate_save(output_path)
        _, rebuilt_type, rebuilt_gvas = load_gvas(output_path)
        if rebuilt_type != save_type:
            raise ValueError(
                f"Save type changed from 0x{save_type:02X} to 0x{rebuilt_type:02X}"
            )
        verify_delivery(rebuilt_gvas, player_gvas, result)
    except Exception:
        output_path.unlink(missing_ok=True)
        raise

    log("DELIVERY_RESULT " + json.dumps(result.to_dict(), ensure_ascii=True))
    log(f"Edited save written to {output_path}")


def set_inventory_quantity(
    file_path: Path,
    output: str,
    player_uid: str,
    container: str,
    slot_index: int,
    quantity: int,
    expected_item_id: str,
    expected_quantity: int,
    metadata: str,
    expected_dynamic_id: str,
) -> None:
    if not player_uid:
        raise ValueError("--player-uid is required for set-item-quantity mode")
    if not expected_item_id:
        raise ValueError(
            "--expected-item-id is required for set-item-quantity mode"
        )
    if slot_index < 0:
        raise ValueError("--slot-index must be zero or greater")

    output_path = Path(output or f"{file_path}.set-item-quantity.sav")
    if output_path.resolve() == file_path.resolve():
        raise ValueError("Set-item-quantity output must not overwrite the input save")

    save_type, level_gvas = load_level_for_edit(file_path)
    player_path = resolve_player_save(file_path, player_uid)
    _, _, player_gvas = load_gvas(player_path)
    catalog = load_item_catalog(metadata)
    result = set_item_quantity(
        level_gvas,
        player_gvas,
        player_uid,
        catalog,
        container,
        slot_index,
        quantity,
        expected_item_id,
        expected_quantity,
        expected_dynamic_id,
    )

    written = level_gvas.write(custom_properties=PALWORLD_CUSTOM_PROPERTIES)
    rebuilt = compress_gvas_to_sav(written, save_type)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_bytes(rebuilt)
    try:
        validate_save(output_path)
        _, rebuilt_type, rebuilt_gvas = load_gvas(output_path)
        if rebuilt_type != save_type:
            raise ValueError(
                f"Save type changed from 0x{save_type:02X} to 0x{rebuilt_type:02X}"
            )
        verify_inventory_mutation(rebuilt_gvas, player_gvas, result)
    except Exception:
        output_path.unlink(missing_ok=True)
        raise

    log("INVENTORY_RESULT " + json.dumps(result.to_dict(), ensure_ascii=True))
    log(f"Edited save written to {output_path}")


def edit_player_profile(
    file_path: Path,
    output: str,
    player_uid: str,
    expected_nickname: str,
    expected_level: int,
    nickname: str,
    level: int,
    exp_table_path: str,
) -> None:
    if not player_uid:
        raise ValueError("--player-uid is required for edit-player-profile mode")
    output_path = Path(output or f"{file_path}.edit-player-profile.sav")
    if output_path.resolve() == file_path.resolve():
        raise ValueError("Edit-player-profile output must not overwrite the input save")

    save_type, level_gvas = load_level_for_edit(file_path)
    bundle_dir = Path(getattr(sys, "_MEIPASS", Path(__file__).resolve().parent))
    table_path = exp_table_path or str(bundle_dir / "pal_exp_table.json")
    result = set_player_profile(
        level_gvas,
        player_uid,
        expected_nickname,
        expected_level,
        nickname,
        level,
        load_exp_table(table_path),
    )

    written = level_gvas.write(custom_properties=PALWORLD_CUSTOM_PROPERTIES)
    rebuilt = compress_gvas_to_sav(written, save_type)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_bytes(rebuilt)
    try:
        validate_save(output_path)
        _, rebuilt_type, rebuilt_gvas = load_gvas(output_path)
        if rebuilt_type != save_type:
            raise ValueError(
                f"Save type changed from 0x{save_type:02X} to 0x{rebuilt_type:02X}"
            )
        verify_player_profile(rebuilt_gvas, result)
    except Exception:
        output_path.unlink(missing_ok=True)
        raise

    log("PLAYER_PROFILE_RESULT " + json.dumps(result.to_dict(), ensure_ascii=True))
    log(f"Edited save written to {output_path}")


def edit_player_stat_points(
    file_path: Path,
    output: str,
    player_uid: str,
    expected_unused_stat_points: int,
    unused_stat_points: int,
) -> None:
    if not player_uid:
        raise ValueError("--player-uid is required for edit-player-stat-points mode")
    output_path = Path(output or f"{file_path}.edit-player-stat-points.sav")
    if output_path.resolve() == file_path.resolve():
        raise ValueError(
            "Edit-player-stat-points output must not overwrite the input save"
        )

    save_type, level_gvas = load_level_for_edit(file_path)
    result = set_player_stat_points(
        level_gvas,
        player_uid,
        expected_unused_stat_points,
        unused_stat_points,
    )

    written = level_gvas.write(custom_properties=PALWORLD_CUSTOM_PROPERTIES)
    rebuilt = compress_gvas_to_sav(written, save_type)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_bytes(rebuilt)
    try:
        validate_save(output_path)
        _, rebuilt_type, rebuilt_gvas = load_gvas(output_path)
        if rebuilt_type != save_type:
            raise ValueError(
                f"Save type changed from 0x{save_type:02X} to 0x{rebuilt_type:02X}"
            )
        verify_player_stat_points(rebuilt_gvas, result)
    except Exception:
        output_path.unlink(missing_ok=True)
        raise

    log(
        "PLAYER_STAT_POINTS_RESULT "
        + json.dumps(result.to_dict(), ensure_ascii=True)
    )
    log(f"Edited save written to {output_path}")


def edit_player_technology_points(
    file_path: Path,
    output: str,
    player_uid: str,
    expected_technology_points: int,
    expected_ancient_technology_points: int,
    technology_points: int,
    ancient_technology_points: int,
) -> None:
    if not player_uid:
        raise ValueError("--player-uid is required for edit-player-technology-points mode")
    output_path = Path(output or f"{file_path}.edit-player-technology-points.sav")
    if output_path.resolve() == file_path.resolve():
        raise ValueError(
            "Edit-player-technology-points output must not overwrite the input save"
        )

    raw_gvas, save_type, player_gvas = load_gvas(file_path)
    require_lossless_gvas_roundtrip(
        raw_gvas,
        player_gvas,
        "Player GVAS changed during the edit preflight validation pass",
    )
    result = set_player_technology_points(
        player_gvas,
        player_uid,
        expected_technology_points,
        expected_ancient_technology_points,
        technology_points,
        ancient_technology_points,
    )

    written = player_gvas.write(custom_properties=PALWORLD_CUSTOM_PROPERTIES)
    rebuilt = compress_gvas_to_sav(written, save_type)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_bytes(rebuilt)
    try:
        validate_save(output_path)
        _, rebuilt_type, rebuilt_gvas = load_gvas(output_path)
        if rebuilt_type != save_type:
            raise ValueError(
                f"Save type changed from 0x{save_type:02X} to 0x{rebuilt_type:02X}"
            )
        verify_player_technology_points(rebuilt_gvas, result)
    except Exception:
        output_path.unlink(missing_ok=True)
        raise

    log(
        "PLAYER_TECHNOLOGY_POINTS_RESULT "
        + json.dumps(result.to_dict(), ensure_ascii=True)
    )
    log(f"Edited player save written to {output_path}")


def unlock_player_map_save(
    file_path: Path,
    output: str,
    player_uid: str,
    expected_progress_digest: str,
    map_metadata_path: str,
) -> None:
    if not player_uid:
        raise ValueError("--player-uid is required for unlock-player-map mode")
    if not expected_progress_digest:
        raise ValueError(
            "--expected-map-progress-digest is required for unlock-player-map mode"
        )
    output_path = Path(output or f"{file_path}.unlock-player-map.sav")
    if output_path.resolve() == file_path.resolve():
        raise ValueError("Unlock-player-map output must not overwrite the input save")

    raw_gvas, save_type, player_gvas = load_gvas(file_path)
    require_lossless_gvas_roundtrip(
        raw_gvas,
        player_gvas,
        "Player GVAS changed during the map edit preflight validation pass",
    )
    bundle_dir = Path(getattr(sys, "_MEIPASS", Path(__file__).resolve().parent))
    metadata_path = map_metadata_path or str(bundle_dir / "player_map_metadata.json")
    metadata = load_player_map_metadata(metadata_path)
    result = unlock_player_map(
        player_gvas,
        player_uid,
        metadata,
        expected_progress_digest,
    )

    written = player_gvas.write(custom_properties=PALWORLD_CUSTOM_PROPERTIES)
    rebuilt = compress_gvas_to_sav(written, save_type)
    temporary_path = _write_temporary_save(output_path, rebuilt)
    try:
        rebuilt_raw, rebuilt_type, rebuilt_gvas = load_gvas(temporary_path)
        if rebuilt_type != save_type:
            raise ValueError(
                f"Save type changed from 0x{save_type:02X} to 0x{rebuilt_type:02X}"
            )
        if rebuilt_raw != written:
            raise ValueError("Rebuilt player save changed the edited GVAS bytes")
        require_lossless_gvas_roundtrip(
            rebuilt_raw,
            rebuilt_gvas,
            "GVAS changed during the rebuilt player map validation pass",
        )
        verify_player_map_unlock(rebuilt_gvas, metadata, result)
        os.replace(temporary_path, output_path)
    except Exception:
        temporary_path.unlink(missing_ok=True)
        raise

    log(
        "PLAYER_MAP_PROGRESS_RESULT "
        + json.dumps(result.to_dict(), ensure_ascii=True)
    )
    log(f"Edited player save written to {output_path}")


def edit_pal_nickname(
    file_path: Path,
    output: str,
    player_uid: str,
    instance_id: str,
    expected_nickname: str | None,
    expected_level: int,
    expected_exp: int,
    nickname: str | None,
) -> None:
    if not player_uid:
        raise ValueError("--player-uid is required for edit-pal-nickname mode")
    if not instance_id:
        raise ValueError("--instance-id is required for edit-pal-nickname mode")
    if expected_nickname is None:
        raise ValueError(
            "--expected-pal-nickname is required for edit-pal-nickname mode"
        )
    if nickname is None:
        raise ValueError("--pal-nickname is required for edit-pal-nickname mode")
    output_path = Path(output or f"{file_path}.edit-pal-nickname.sav")
    if output_path.resolve() == file_path.resolve():
        raise ValueError("Edit-pal-nickname output must not overwrite the input save")

    save_type, level_gvas = load_level_for_pal_edit(file_path)
    result = rename_pal(
        level_gvas,
        player_uid,
        instance_id,
        expected_nickname,
        expected_level,
        expected_exp,
        nickname,
    )

    written = level_gvas.write(custom_properties=PAL_EDIT_CUSTOM_PROPERTIES)
    rebuilt = compress_gvas_to_sav(written, save_type)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_bytes(rebuilt)
    try:
        rebuilt_raw, rebuilt_type, rebuilt_gvas = load_gvas(
            output_path, PAL_EDIT_CUSTOM_PROPERTIES
        )
        if rebuilt_type != save_type:
            raise ValueError(
                f"Save type changed from 0x{save_type:02X} to 0x{rebuilt_type:02X}"
            )
        require_lossless_gvas_roundtrip(
            rebuilt_raw,
            rebuilt_gvas,
            "GVAS changed during the rebuilt Pal save validation pass",
            PAL_EDIT_CUSTOM_PROPERTIES,
        )
        verify_pal_nickname(rebuilt_gvas, result)
    except Exception:
        output_path.unlink(missing_ok=True)
        raise

    log("PAL_NICKNAME_RESULT " + json.dumps(result.to_dict(), ensure_ascii=True))
    log(f"Edited save written to {output_path}")


def edit_pal_level(
    file_path: Path,
    output: str,
    player_uid: str,
    instance_id: str,
    expected_nickname: str | None,
    expected_level: int,
    expected_exp: int,
    expected_hp: int,
    expected_max_hp: int,
    level: int,
    exp_table_path: str,
    level_metadata_path: str,
) -> None:
    if not player_uid:
        raise ValueError("--player-uid is required for edit-pal-level mode")
    if not instance_id:
        raise ValueError("--instance-id is required for edit-pal-level mode")
    if expected_nickname is None:
        raise ValueError("--expected-pal-nickname is required for edit-pal-level mode")
    output_path = Path(output or f"{file_path}.edit-pal-level.sav")
    if output_path.resolve() == file_path.resolve():
        raise ValueError("Edit-pal-level output must not overwrite the input save")

    save_type, level_gvas = load_level_for_pal_edit(file_path)
    bundle_dir = Path(getattr(sys, "_MEIPASS", Path(__file__).resolve().parent))
    table_path = exp_table_path or str(bundle_dir / "pal_exp_table.json")
    metadata_path = level_metadata_path or str(
        bundle_dir / "pal_level_metadata.json"
    )
    level_metadata, friendship_thresholds = load_pal_level_metadata(metadata_path)
    result = set_pal_level(
        level_gvas,
        player_uid,
        instance_id,
        expected_nickname,
        expected_level,
        expected_exp,
        expected_hp,
        expected_max_hp,
        level,
        load_pal_exp_table(table_path),
        level_metadata,
        friendship_thresholds,
    )

    written = level_gvas.write(custom_properties=PAL_EDIT_CUSTOM_PROPERTIES)
    rebuilt = compress_gvas_to_sav(written, save_type)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_bytes(rebuilt)
    try:
        rebuilt_raw, rebuilt_type, rebuilt_gvas = load_gvas(
            output_path, PAL_EDIT_CUSTOM_PROPERTIES
        )
        if rebuilt_type != save_type:
            raise ValueError(
                f"Save type changed from 0x{save_type:02X} to 0x{rebuilt_type:02X}"
            )
        if rebuilt_raw != written:
            raise ValueError("Rebuilt Pal save changed the edited GVAS bytes")
        require_lossless_gvas_roundtrip(
            rebuilt_raw,
            rebuilt_gvas,
            "GVAS changed during the rebuilt Pal save validation pass",
            PAL_EDIT_CUSTOM_PROPERTIES,
        )
        verify_pal_level(rebuilt_gvas, result)
    except Exception:
        output_path.unlink(missing_ok=True)
        raise

    log("PAL_LEVEL_RESULT " + json.dumps(result.to_dict(), ensure_ascii=True))
    log(f"Edited save written to {output_path}")


def restore_pal_health_save(
    file_path: Path,
    output: str,
    player_uid: str,
    instance_id: str,
    expected_nickname: str | None,
    expected_level: int,
    expected_exp: int,
    expected_hp: int,
    expected_max_hp: int,
) -> None:
    if not player_uid:
        raise ValueError("--player-uid is required for restore-pal-health mode")
    if not instance_id:
        raise ValueError("--instance-id is required for restore-pal-health mode")
    if expected_nickname is None:
        raise ValueError(
            "--expected-pal-nickname is required for restore-pal-health mode"
        )
    output_path = Path(output or f"{file_path}.restore-pal-health.sav")
    if output_path.resolve() == file_path.resolve():
        raise ValueError("Restore-pal-health output must not overwrite the input save")

    save_type, level_gvas = load_level_for_pal_edit(file_path)
    result = restore_pal_health(
        level_gvas,
        player_uid,
        instance_id,
        expected_nickname,
        expected_level,
        expected_exp,
        expected_hp,
        expected_max_hp,
    )

    written = level_gvas.write(custom_properties=PAL_EDIT_CUSTOM_PROPERTIES)
    rebuilt = compress_gvas_to_sav(written, save_type)
    temporary_path = _write_temporary_save(output_path, rebuilt)
    try:
        rebuilt_raw, rebuilt_type, rebuilt_gvas = load_gvas(
            temporary_path, PAL_EDIT_CUSTOM_PROPERTIES
        )
        if rebuilt_type != save_type:
            raise ValueError(
                f"Save type changed from 0x{save_type:02X} to 0x{rebuilt_type:02X}"
            )
        if rebuilt_raw != written:
            raise ValueError("Rebuilt Pal save changed the edited GVAS bytes")
        require_lossless_gvas_roundtrip(
            rebuilt_raw,
            rebuilt_gvas,
            "GVAS changed during the rebuilt Pal save validation pass",
            PAL_EDIT_CUSTOM_PROPERTIES,
        )
        verify_pal_health(rebuilt_gvas, result)
        os.replace(temporary_path, output_path)
    except Exception:
        temporary_path.unlink(missing_ok=True)
        raise

    log("PAL_HEALTH_RESULT " + json.dumps(result.to_dict(), ensure_ascii=True))
    log(f"Edited save written to {output_path}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Palworld save processing tool")
    parser.add_argument(
        "--mode",
        choices=(
            "structure",
            "export",
            "rebuild",
            "validate",
            "roundtrip",
            "give-item",
            "set-item-quantity",
            "edit-player-profile",
            "edit-player-stat-points",
            "edit-player-technology-points",
            "unlock-player-map",
            "edit-pal-nickname",
            "edit-pal-level",
            "restore-pal-health",
        ),
        default="structure",
        help="Processing mode. The default keeps PST's existing sync behavior.",
    )
    parser.add_argument("--file", "-f", default="Level.sav", help="Input file")
    parser.add_argument("--output", "-o", default="", help="Output file")
    parser.add_argument("--request", "-r", default="", help="PST API base URL")
    parser.add_argument("--token", "-t", default="", help="PST API token")
    parser.add_argument("--player-uid", default="", help="Target player UID")
    parser.add_argument("--instance-id", default="", help="Target Pal instance ID")
    parser.add_argument("--nickname", default="", help="New player nickname")
    parser.add_argument(
        "--expected-nickname",
        default="",
        help="Player nickname currently expected in Level.sav",
    )
    parser.add_argument("--level", type=int, default=-1, help="New player level")
    parser.add_argument(
        "--expected-level",
        type=int,
        default=-1,
        help="Player level currently expected in Level.sav",
    )
    parser.add_argument(
        "--pal-nickname",
        default=None,
        help="New Pal nickname; an empty value clears the custom nickname",
    )
    parser.add_argument(
        "--expected-pal-nickname",
        default=None,
        help="Pal nickname currently expected in Level.sav",
    )
    parser.add_argument(
        "--expected-pal-level",
        type=int,
        default=-1,
        help="Pal level currently expected in Level.sav",
    )
    parser.add_argument(
        "--expected-pal-exp",
        type=int,
        default=-1,
        help="Pal EXP currently expected in Level.sav",
    )
    parser.add_argument(
        "--pal-level",
        type=int,
        default=-1,
        help="New Pal level",
    )
    parser.add_argument(
        "--expected-pal-hp",
        type=int,
        default=-1,
        help="Pal HP currently expected in Level.sav",
    )
    parser.add_argument(
        "--expected-pal-max-hp",
        type=int,
        default=-1,
        help="Pal MaxHP currently expected in Level.sav; use zero when absent",
    )
    parser.add_argument(
        "--exp-table",
        default="",
        help="Optional path to the Palworld player experience table",
    )
    parser.add_argument(
        "--pal-level-metadata",
        default="",
        help="Optional path to generated Palworld 1.0 Pal level metadata",
    )
    parser.add_argument(
        "--unused-stat-points",
        type=int,
        default=-1,
        help="New unallocated player stat point balance",
    )
    parser.add_argument(
        "--expected-unused-stat-points",
        type=int,
        default=-1,
        help="Unallocated player stat point balance currently expected in Level.sav",
    )
    parser.add_argument(
        "--technology-points",
        type=int,
        default=-1,
        help="New technology point balance",
    )
    parser.add_argument(
        "--expected-technology-points",
        type=int,
        default=-1,
        help="Technology point balance currently expected in the player save",
    )
    parser.add_argument(
        "--ancient-technology-points",
        type=int,
        default=-1,
        help="New ancient technology point balance",
    )
    parser.add_argument(
        "--expected-ancient-technology-points",
        type=int,
        default=-1,
        help="Ancient technology point balance currently expected in the player save",
    )
    parser.add_argument(
        "--expected-map-progress-digest",
        default="",
        help="SHA-256 digest of the player map progress currently expected in the save",
    )
    parser.add_argument(
        "--map-metadata",
        default="",
        help="Optional path to generated Palworld 1.0 player map metadata",
    )
    parser.add_argument("--item-id", default="", help="Canonical Palworld item ID")
    parser.add_argument("--quantity", type=int, default=1, help="Item quantity")
    parser.add_argument("--slot-index", type=int, default=-1, help="Inventory slot")
    parser.add_argument(
        "--expected-item-id",
        default="",
        help="Item ID currently expected in the target slot",
    )
    parser.add_argument(
        "--expected-quantity",
        type=int,
        default=-1,
        help="Quantity currently expected in the target slot",
    )
    parser.add_argument(
        "--expected-dynamic-id",
        default="00000000-0000-0000-0000-000000000000",
        help="Dynamic item instance ID currently expected in the target slot",
    )
    parser.add_argument(
        "--container",
        choices=("auto", "main", "key", "weapons", "armor", "food", "drop"),
        default="auto",
        help="Target inventory container",
    )
    parser.add_argument(
        "--metadata",
        default="",
        help="Optional path to generated item metadata",
    )
    parser.add_argument(
        "--clear",
        "-c",
        action="store_true",
        help="Remove the temporary input directory after a successful structure sync",
    )
    parser.add_argument("--debug", action="store_true", help="Print a full traceback")
    return parser.parse_args()


def main() -> int:
    start = time.time()
    args = parse_args()
    file_path = Path(args.file)

    if not file_path.is_file():
        log(f"Input file does not exist: {file_path}", "ERROR")
        return 1

    try:
        if args.mode == "structure":
            structure_save(file_path, args.output, args.request, args.token)
        elif args.mode == "export":
            export_json(file_path, args.output)
        elif args.mode == "rebuild":
            rebuild_json(file_path, args.output)
        elif args.mode == "validate":
            validate_save(file_path)
        elif args.mode == "roundtrip":
            roundtrip_save(file_path, args.output)
        elif args.mode == "give-item":
            give_item(
                file_path,
                args.output,
                args.player_uid,
                args.item_id,
                args.quantity,
                args.container,
                args.metadata,
            )
        elif args.mode == "set-item-quantity":
            set_inventory_quantity(
                file_path,
                args.output,
                args.player_uid,
                args.container,
                args.slot_index,
                args.quantity,
                args.expected_item_id,
                args.expected_quantity,
                args.metadata,
                args.expected_dynamic_id,
            )
        elif args.mode == "edit-player-profile":
            edit_player_profile(
                file_path,
                args.output,
                args.player_uid,
                args.expected_nickname,
                args.expected_level,
                args.nickname,
                args.level,
                args.exp_table,
            )
        elif args.mode == "edit-player-stat-points":
            edit_player_stat_points(
                file_path,
                args.output,
                args.player_uid,
                args.expected_unused_stat_points,
                args.unused_stat_points,
            )
        elif args.mode == "edit-player-technology-points":
            edit_player_technology_points(
                file_path,
                args.output,
                args.player_uid,
                args.expected_technology_points,
                args.expected_ancient_technology_points,
                args.technology_points,
                args.ancient_technology_points,
            )
        elif args.mode == "unlock-player-map":
            unlock_player_map_save(
                file_path,
                args.output,
                args.player_uid,
                args.expected_map_progress_digest,
                args.map_metadata,
            )
        elif args.mode == "edit-pal-nickname":
            edit_pal_nickname(
                file_path,
                args.output,
                args.player_uid,
                args.instance_id,
                args.expected_pal_nickname,
                args.expected_pal_level,
                args.expected_pal_exp,
                args.pal_nickname,
            )
        elif args.mode == "edit-pal-level":
            edit_pal_level(
                file_path,
                args.output,
                args.player_uid,
                args.instance_id,
                args.expected_pal_nickname,
                args.expected_pal_level,
                args.expected_pal_exp,
                args.expected_pal_hp,
                args.expected_pal_max_hp,
                args.pal_level,
                args.exp_table,
                args.pal_level_metadata,
            )
        elif args.mode == "restore-pal-health":
            restore_pal_health_save(
                file_path,
                args.output,
                args.player_uid,
                args.instance_id,
                args.expected_pal_nickname,
                args.expected_pal_level,
                args.expected_pal_exp,
                args.expected_pal_hp,
                args.expected_pal_max_hp,
            )

        if args.clear and args.mode == "structure":
            try:
                file_path.unlink(missing_ok=True)
                player_dir = file_path.parent / "Players"
                if player_dir.exists():
                    shutil.rmtree(player_dir)
            except OSError as exc:
                log(f"Unable to clear temporary input: {exc}", "WARNING")
    except (
        InventoryConflictError,
        PalConflictError,
        PlayerConflictError,
        PlayerSaveConflictError,
    ) as exc:
        log(
            "SAVE_EDIT_ERROR "
            + json.dumps(
                {"code": "stale_state", "message": str(exc)},
                ensure_ascii=True,
            ),
            "ERROR",
        )
        if args.debug:
            traceback.print_exc()
        return 1
    except (
        InventoryEditError,
        PalEditError,
        PlayerEditError,
        PlayerSaveEditError,
    ) as exc:
        log(
            "SAVE_EDIT_ERROR "
            + json.dumps(
                {"code": "validation", "message": str(exc)},
                ensure_ascii=True,
            ),
            "ERROR",
        )
        if args.debug:
            traceback.print_exc()
        return 1
    except Exception as exc:
        log(str(exc), "ERROR")
        if args.debug:
            traceback.print_exc()
        return 1

    log(f"Done in {round(time.time() - start, 3)}s")
    return 0


if __name__ == "__main__":
    sys.exit(main())
