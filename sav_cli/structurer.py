import copy
import os
import sys
import zlib
import json
import time
from functools import lru_cache
from pathlib import Path
from typing import Any

from palsav.gvas import GvasFile
from palsav.core import decompress_sav_to_gvas
from palsav.paltypes import PALWORLD_CUSTOM_PROPERTIES, PALWORLD_TYPE_HINTS
from palsav.archive import FArchiveReader, FArchiveWriter

from world_types import Player, Pal, Guild, BaseCamp
from logger import log, redirect_stdout_stderr
from player_save_editor import load_player_map_metadata, read_player_map_progress

wsd = None
gvas_file = None


@lru_cache(maxsize=1)
def _load_bundled_player_map_metadata():
    bundle_dir = Path(getattr(sys, "_MEIPASS", Path(__file__).resolve().parent))
    return load_player_map_metadata(str(bundle_dir / "player_map_metadata.json"))


def skip_decode(
    reader: FArchiveReader, type_name: str, size: int, path: str
) -> dict[str, Any]:
    if type_name == "ArrayProperty":
        array_type = reader.fstring()
        value = {
            "skip_type": type_name,
            "array_type": array_type,
            "id": reader.optional_guid(),
            "value": reader.read(size),
        }
    elif type_name == "MapProperty":
        key_type = reader.fstring()
        value_type = reader.fstring()
        _id = reader.optional_guid()
        value = {
            "skip_type": type_name,
            "key_type": key_type,
            "value_type": value_type,
            "id": _id,
            "value": reader.read(size),
        }
    elif type_name == "StructProperty":
        value = {
            "skip_type": type_name,
            "struct_type": reader.fstring(),
            "struct_id": reader.guid(),
            "id": reader.optional_guid(),
            "value": reader.read(size),
        }
    elif type_name == "SetProperty":
        set_type = reader.fstring()
        value = {
            "skip_type": type_name,
            "set_type": set_type,
            "id": reader.optional_guid(),
            "value": reader.read(size),
        }
    else:
        raise Exception(
            f"Expected ArrayProperty or MapProperty or StructProperty, got {type_name} in {path}"
        )
    return value


def skip_encode(
    writer: FArchiveWriter, property_type: str, properties: dict[str, Any]
) -> int:
    if "skip_type" not in properties:
        if properties["custom_type"] in PALWORLD_CUSTOM_PROPERTIES is not None:
            return PALWORLD_CUSTOM_PROPERTIES[properties["custom_type"]][1](
                writer, property_type, properties
            )
    if property_type == "ArrayProperty":
        del properties["custom_type"]
        del properties["skip_type"]
        writer.fstring(properties["array_type"])
        writer.optional_guid(properties.get("id", None))
        writer.write(properties["value"])
        return len(properties["value"])
    elif property_type == "MapProperty":
        del properties["custom_type"]
        del properties["skip_type"]
        writer.fstring(properties["key_type"])
        writer.fstring(properties["value_type"])
        writer.optional_guid(properties.get("id", None))
        writer.write(properties["value"])
        return len(properties["value"])
    elif property_type == "StructProperty":
        del properties["custom_type"]
        del properties["skip_type"]
        writer.fstring(properties["struct_type"])
        writer.guid(properties["struct_id"])
        writer.optional_guid(properties.get("id", None))
        writer.write(properties["value"])
        return len(properties["value"])
    elif property_type == "SetProperty":
        del properties["custom_type"]
        del properties["skip_type"]
        writer.fstring(properties["set_type"])
        writer.optional_guid(properties.get("id", None))
        writer.write(properties["value"])
        return len(properties["value"])
    else:
        raise Exception(
            f"Expected ArrayProperty or MapProperty or StructProperty, got {property_type}"
        )


def load_skiped_decode(wsd, skip_paths, recursive=True):
    if isinstance(skip_paths, str):
        skip_paths = [skip_paths]
    for skip_path in skip_paths:
        properties = wsd[skip_path]

        if "skip_type" not in properties:
            continue
        parse_skiped_item(properties, skip_path, recursive)
        if ".worldSaveData.%s" % skip_path in SKP_PALWORLD_CUSTOM_PROPERTIES:
            del SKP_PALWORLD_CUSTOM_PROPERTIES[".worldSaveData.%s" % skip_path]


SKP_PALWORLD_CUSTOM_PROPERTIES = copy.deepcopy(PALWORLD_CUSTOM_PROPERTIES)
SKP_PALWORLD_CUSTOM_PROPERTIES[".worldSaveData.MapObjectSaveData"] = (
    skip_decode,
    skip_encode,
)
SKP_PALWORLD_CUSTOM_PROPERTIES[".worldSaveData.FoliageGridSaveDataMap"] = (
    skip_decode,
    skip_encode,
)
SKP_PALWORLD_CUSTOM_PROPERTIES[".worldSaveData.MapObjectSpawnerInStageSaveData"] = (
    skip_decode,
    skip_encode,
)
SKP_PALWORLD_CUSTOM_PROPERTIES[".worldSaveData.DynamicItemSaveData"] = (
    skip_decode,
    skip_encode,
)
SKP_PALWORLD_CUSTOM_PROPERTIES[".worldSaveData.RandomizerSaveData"] = (
    skip_decode,
    skip_encode,
)
SKP_PALWORLD_CUSTOM_PROPERTIES[".worldSaveData.InLockerCharacterInstanceIDArray"] = (
    skip_decode,
    skip_encode,
)


def convert_sav(file):
    global gvas_file, wsd
    if file.endswith(".sav.json"):
        log("Loading...")
        with open(file, "r", encoding="utf-8") as f:
            return f.read()
    log("Converting...")
    with redirect_stdout_stderr():
        try:
            with open(file, "rb") as f:
                data = f.read()
                raw_gvas, _ = decompress_sav_to_gvas(data)
            gvas_file = GvasFile.read(
                raw_gvas, PALWORLD_TYPE_HINTS, SKP_PALWORLD_CUSTOM_PROPERTIES
            )
        except zlib.error:
            log("This .sav file is corrupted. :(", "ERROR")
            sys.exit(1)
    # return json.dumps(gvas_file.dump(), cls=CustomEncoder)
    wsd = gvas_file.properties["worldSaveData"]["value"]


def structure_player(dir_path, data_source=None, filetime: int = -1):
    log("Structuring players...")
    global wsd
    if data_source is None:
        data_source = wsd
    if not data_source.get("CharacterSaveParameterMap"):
        return []
    uid_character = (
        (
            c["key"]["PlayerUId"]["value"],
            c["key"].get("InstanceId", {}).get("value"),
            c["value"]["RawData"]["value"]["object"]["SaveParameter"]["value"],
        )
        for c in wsd["CharacterSaveParameterMap"]["value"]
    )

    players = []
    pals = []
    ticks = wsd["GameTimeSaveData"]["value"]["RealDateTimeTicks"]["value"]
    for uid, instance_id, c in uid_character:
        if c.get("IsPlayer") and c["IsPlayer"]["value"]:
            player_data = dict(c)
            player_save_data = getPlayerItems(uid, dir_path)
            player_data["Items"] = (
                player_save_data["items"] if player_save_data is not None else None
            )
            if player_save_data is not None:
                player_data["PlayerSaveData"] = player_save_data
            else:
                player_data.pop("PlayerSaveData", None)
            players.append(Player(uid, player_data).to_dict())
        else:
            if not c.get("OwnerPlayerUId"):
                continue
            pals.append(Pal(instance_id, c, ticks, filetime).to_dict())

    unique_players_dict = {}
    for player in players:
        player_uid = player["player_uid"]
        if player_uid in unique_players_dict:
            existing_player = unique_players_dict[player_uid]
            if player["level"] > existing_player["level"]:
                unique_players_dict[player_uid] = player
        else:
            unique_players_dict[player_uid] = player

    unique_players = list(unique_players_dict.values())
    for pal in pals:
        for player in unique_players:
            if player["player_uid"] == pal["owner"]:
                pal.pop("owner")
                player["pals"].append(pal)
                break

    sorted_players = sorted(unique_players, key=lambda p: p["level"], reverse=True)

    return sorted_players


def parse_skiped_item(properties, skip_path, recursive=True):
    if "skip_type" not in properties:
        return properties

    with FArchiveReader(
        properties["value"],
        PALWORLD_TYPE_HINTS,
        (
            SKP_PALWORLD_CUSTOM_PROPERTIES
            if recursive == False
            else PALWORLD_CUSTOM_PROPERTIES
        ),
    ) as reader:
        if properties["skip_type"] == "ArrayProperty":
            # hack: 0.3.7 later version has a bug that the array type include bytes
            # current use custom item_container_slots.decode to fix it
            properties["value"] = reader.array_property(
                properties["array_type"],
                len(properties["value"]) - 4,
                ".worldSaveData.%s" % skip_path,
            )
        elif properties["skip_type"] == "StructProperty":
            properties["value"] = reader.struct_value(
                properties["struct_type"], ".worldSaveData.%s" % skip_path
            )
        elif properties["skip_type"] == "MapProperty":
            reader.u32()
            count = reader.u32()
            path = ".worldSaveData.%s" % skip_path
            key_path = path + ".Key"
            key_type = properties["key_type"]
            value_type = properties["value_type"]
            if key_type == "StructProperty":
                key_struct_type = reader.get_type_or(key_path, "Guid")
            else:
                key_struct_type = None
            value_path = path + ".Value"
            if value_type == "StructProperty":
                value_struct_type = reader.get_type_or(value_path, "StructProperty")
            else:
                value_struct_type = None
            values: list[dict[str, Any]] = []
            for _ in range(count):
                key = reader.prop_value(key_type, key_struct_type, key_path)
                value = reader.prop_value(value_type, value_struct_type, value_path)
                values.append(
                    {
                        "key": key,
                        "value": value,
                    }
                )
            properties["key_struct_type"] = key_struct_type
            properties["value_struct_type"] = value_struct_type
            properties["value"] = values
        del properties["custom_type"]
        del properties["skip_type"]
    return properties


def parse_item(properties, skip_path):
    if isinstance(properties, dict):
        for key in properties:
            call_skip_path = skip_path + "." + key[0].upper() + key[1:]
            if (
                isinstance(properties[key], dict)
                and "type" in properties[key]
                and properties[key]["type"]
                in ["StructProperty", "ArrayProperty", "MapProperty"]
            ):
                if "skip_type" in properties[key]:
                    # print("Parsing worldSaveData.%s..." % call_skip_path, end="", flush=True)
                    properties[key] = parse_skiped_item(
                        properties[key], call_skip_path, True
                    )
                    # print("Done")
                else:
                    properties[key]["value"] = parse_item(
                        properties[key]["value"], call_skip_path
                    )
            else:
                properties[key] = parse_item(properties[key], call_skip_path)
    elif isinstance(properties, list):
        top_skip_path = ".".join(skip_path.split(".")[:-1])
        for idx, item in enumerate(properties):
            properties[idx] = parse_item(item, top_skip_path)
    return properties


def serialize_player_item(item):
    raw = item["RawData"]["value"]
    return {
        "SlotIndex": raw["slot_index"],
        "ItemId": raw["item"]["static_id"].lower(),
        "StackCount": raw["count"],
        "DynamicId": str(
            raw["item"]["dynamic_id"]["local_id_in_created_world"]
        ),
    }


def read_player_technology_points(save_data):
    def read_int_property(field):
        property_value = save_data.get(field)
        if property_value is None:
            return 0
        if (
            not isinstance(property_value, dict)
            or property_value.get("type") != "IntProperty"
        ):
            raise ValueError(f"Player save field {field} must be an IntProperty")
        if "value" not in property_value:
            raise ValueError(f"Player save field {field} is missing its value")
        value = property_value["value"]
        if isinstance(value, bool) or not isinstance(value, int):
            raise ValueError(f"Player save field {field} must contain an integer")
        return value

    if not isinstance(save_data, dict):
        raise ValueError("Player SaveData must be an object")
    return {
        "technology_points": read_int_property("TechnologyPoint"),
        "ancient_technology_points": read_int_property("bossTechnologyPoint"),
    }


def getPlayerItems(player_uid, dir_path):
    item_containers = {}
    for item_container in wsd["ItemContainerSaveData"]["value"]:
        item_containers[str(item_container["key"]["ID"]["value"])] = item_container

    player_sav_file = os.path.join(
        dir_path, str(player_uid).upper().replace("-", "") + ".sav"
    )
    if not os.path.exists(player_sav_file):
        # log("Player Sav file Not exists: %s" % player_sav_file)
        return
    else:
        with redirect_stdout_stderr():
            try:
                with open(player_sav_file, "rb") as f:
                    raw_gvas, _ = decompress_sav_to_gvas(f.read())
                    player_gvas_file = GvasFile.read(
                        raw_gvas, PALWORLD_TYPE_HINTS, PALWORLD_CUSTOM_PROPERTIES
                    )
                player_gvas = player_gvas_file.properties["SaveData"]["value"]
            except Exception as e:
                log(
                    f"Player Sav file is corrupted: {os.path.basename(player_sav_file)}: {str(e)}",
                    "ERROR",
                )
                return
    technology_points = read_player_technology_points(player_gvas)
    map_progress = read_player_map_progress(
        player_gvas_file, _load_bundled_player_map_metadata()
    ).to_dict()
    containers_data = {
        "CommonContainerId": [],
        "DropSlotContainerId": [],
        "EssentialContainerId": [],
        "FoodEquipContainerId": [],
        "PlayerEquipArmorContainerId": [],
        "WeaponLoadOutContainerId": [],
    }
    for idx_key in containers_data.keys():
        if player_gvas.get("InventoryInfo") is None:
            continue
        if player_gvas["InventoryInfo"]["value"].get(idx_key) is None:
            continue
        container_id = str(
            player_gvas["InventoryInfo"]["value"][idx_key]["value"]["ID"]["value"]
        )
        if container_id in item_containers:
            item_container = item_containers[container_id]
            containers_data[idx_key] = [
                serialize_player_item(item)
                for item in item_container["value"]["Slots"]["value"]["values"]
                if item["RawData"]["value"]["item"]["static_id"].lower() != "none"
            ]
    return {
        "items": containers_data,
        **technology_points,
        "map_progress": map_progress,
    }


def structure_base_camp():
    log("Structuring base camps...")
    if not wsd.get("BaseCampSaveData"):
        return []
    base_camps = (
        b["value"]["RawData"]["value"] for b in wsd["BaseCampSaveData"]["value"]
    )
    base_camps_generator = (BaseCamp(b).to_dict() for b in base_camps)
    return list(base_camps_generator)


def structure_guild(filetime: int = -1):
    log("Structuring guilds...")
    if not wsd.get("GroupSaveDataMap"):
        return []
    base_camps = structure_base_camp()
    groups = (
        g["value"]["RawData"]["value"]
        for g in wsd["GroupSaveDataMap"]["value"]
        if g["value"]["GroupType"]["value"]["value"] == "EPalGroupType::Guild"
    )
    Ticks = wsd["GameTimeSaveData"]["value"]["RealDateTimeTicks"]["value"]
    guilds_generator = (Guild(g, Ticks, filetime).to_dict() for g in groups)
    sorted_guilds = sorted(
        guilds_generator, key=lambda g: g["base_camp_level"], reverse=True
    )
    for guild in sorted_guilds:
        for camp in base_camps:
            if camp["id"] in guild["base_ids"]:
                guild["base_camp"].append(
                    {
                        "id": camp["id"],
                        "area": camp["area_range"],
                        "location_x": camp["transform"]["x"],
                        "location_y": camp["transform"]["y"],
                    }
                )
    return list(sorted_guilds)


if __name__ == "__main__":
    import time

    start = time.time()
    file = "./Level.sav"
    converted = convert_sav(file)
    players = structure_player(converted)
    log("Saving players...")
    with open("players.json", "w", encoding="utf-8") as f:
        json.dump(players, f, indent=4, ensure_ascii=False)
    guilds = structure_guild(converted)
    log("Saving guilds...")
    with open("guilds.json", "w", encoding="utf-8") as f:
        json.dump(guilds, f, indent=4, ensure_ascii=False)
    log(f"Done in {time.time() - start}s")
