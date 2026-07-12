from __future__ import annotations

import copy
import json
import tempfile
import unittest
from pathlib import Path

from player_save_editor import (
    ANCIENT_TECHNOLOGY_FIELD,
    AREA_FIELD,
    EXPECTED_AREA_COUNT,
    EXPECTED_FAST_TRAVEL_COUNT,
    FAST_TRAVEL_FIELD,
    PLAYER_MAP_GAME_VERSION,
    PLAYER_MAP_SOURCE_COMMIT,
    TECHNOLOGY_FIELD,
    WORLD_MAP_FIELD,
    WORLD_MAP_FLAGS,
    PlayerMapMetadata,
    PlayerSaveConflictError,
    PlayerSaveEditError,
    load_player_map_metadata,
    read_player_map_progress,
    set_player_technology_points,
    unlock_player_map,
    verify_player_map_unlock,
    verify_player_technology_points,
)


PLAYER_UID = "7e516548-0000-0000-0000-000000000000"
PLAYER_UID_DECIMAL = "2119263560"
FAST_TRAVEL_GUIDS = tuple(
    f"{index:032X}" for index in range(1, EXPECTED_FAST_TRAVEL_COUNT + 1)
)
AREA_IDS = tuple(f"Area_{index:03d}" for index in range(EXPECTED_AREA_COUNT))
MAP_METADATA = PlayerMapMetadata(
    source_commit=PLAYER_MAP_SOURCE_COMMIT,
    game_version=PLAYER_MAP_GAME_VERSION,
    fast_travel_guids=FAST_TRAVEL_GUIDS,
    areas=AREA_IDS,
    world_flags=WORLD_MAP_FLAGS,
)


def make_player_save(technology: int | None = None, ancient: int | None = None) -> dict:
    value = {
        "PlayerUId": {
            "struct_type": "Guid",
            "type": "StructProperty",
            "value": PLAYER_UID,
        },
        "UnlockedRecipeTechnologyNames": {
            "array_type": "NameProperty",
            "type": "ArrayProperty",
            "value": {"values": ["Workbench", "PalBox"]},
        },
    }
    if technology is not None:
        value[TECHNOLOGY_FIELD] = {
            "id": None,
            "type": "IntProperty",
            "value": technology,
        }
    if ancient is not None:
        value[ANCIENT_TECHNOLOGY_FIELD] = {
            "id": None,
            "type": "IntProperty",
            "value": ancient,
        }
    return {
        "SaveData": {
            "struct_type": "PalWorldPlayerSaveData",
            "type": "StructProperty",
            "value": value,
        }
    }


def map_property(entries: list[dict] | None = None) -> dict:
    return {
        "key_type": "NameProperty",
        "value_type": "BoolProperty",
        "key_struct_type": None,
        "value_struct_type": None,
        "id": None,
        "value": copy.deepcopy(entries or []),
        "type": "MapProperty",
    }


def make_player_map_save(
    fast_travel: list[dict] | None = None,
    areas: list[dict] | None = None,
    world_maps: list[dict] | None = None,
    include_fast_travel: bool = True,
    include_areas: bool = True,
    include_world_maps: bool = True,
) -> dict:
    data = make_player_save(7, 3)
    record_data = {
        "UnrelatedRecordField": {
            "id": None,
            "type": "IntProperty",
            "value": 44,
        }
    }
    if include_fast_travel:
        record_data[FAST_TRAVEL_FIELD] = map_property(fast_travel)
    if include_areas:
        record_data[AREA_FIELD] = map_property(areas)
    if include_world_maps:
        record_data[WORLD_MAP_FIELD] = map_property(world_maps)
    save_data = data["SaveData"]["value"]
    save_data["RecordData"] = {
        "struct_type": "PalLoggedinPlayerSaveDataRecordData",
        "struct_id": "00000000-0000-0000-0000-000000000000",
        "id": None,
        "value": record_data,
        "type": "StructProperty",
    }
    save_data["UnrelatedSaveField"] = {
        "array_type": "NameProperty",
        "id": None,
        "type": "ArrayProperty",
        "value": {"values": ["keep", "unchanged"]},
    }
    return data


def without_map_fields(data: dict) -> dict:
    stripped = copy.deepcopy(data)
    record_data = stripped["SaveData"]["value"]["RecordData"]["value"]
    for field in (FAST_TRAVEL_FIELD, AREA_FIELD, WORLD_MAP_FIELD):
        record_data.pop(field, None)
    return stripped


class PlayerSaveEditorTest(unittest.TestCase):
    def test_missing_zero_fields_are_created_only_when_needed(self) -> None:
        data = make_player_save()
        before_recipes = copy.deepcopy(
            data["SaveData"]["value"]["UnlockedRecipeTechnologyNames"]
        )
        result = set_player_technology_points(
            data, PLAYER_UID_DECIMAL, 0, 0, 12, 0
        )

        save_data = data["SaveData"]["value"]
        self.assertEqual(12, save_data[TECHNOLOGY_FIELD]["value"])
        self.assertEqual("IntProperty", save_data[TECHNOLOGY_FIELD]["type"])
        self.assertNotIn(ANCIENT_TECHNOLOGY_FIELD, save_data)
        self.assertEqual((TECHNOLOGY_FIELD,), result.created_fields)
        self.assertEqual(
            before_recipes, save_data["UnlockedRecipeTechnologyNames"]
        )
        verify_player_technology_points(data, result)

    def test_existing_fields_are_updated_and_zero_is_preserved(self) -> None:
        data = make_player_save(23, 16)
        result = set_player_technology_points(
            data, PLAYER_UID, 23, 16, 0, 4
        )
        save_data = data["SaveData"]["value"]
        self.assertEqual(0, save_data[TECHNOLOGY_FIELD]["value"])
        self.assertEqual(4, save_data[ANCIENT_TECHNOLOGY_FIELD]["value"])
        self.assertEqual((), result.created_fields)
        verify_player_technology_points(data, result)

    def test_stale_balances_do_not_mutate(self) -> None:
        data = make_player_save(3, 2)
        before = json.dumps(data, sort_keys=True)
        with self.assertRaisesRegex(PlayerSaveConflictError, "changed from 1 to 3"):
            set_player_technology_points(data, PLAYER_UID, 1, 2, 4, 3)
        self.assertEqual(before, json.dumps(data, sort_keys=True))

    def test_rejects_bad_types_ranges_uid_and_unchanged(self) -> None:
        cases = []
        bad_type = make_player_save(1, 2)
        bad_type["SaveData"]["value"][TECHNOLOGY_FIELD]["type"] = "UInt16Property"
        cases.append((bad_type, PLAYER_UID, 1, 2, 2, 3, "IntProperty"))
        cases.append((make_player_save(), PLAYER_UID, 0, 0, 1_000_000, 0, "between"))
        cases.append((make_player_save(1, 2), "00000001", 1, 2, 2, 3, "does not match"))
        cases.append((make_player_save(1, 2), PLAYER_UID, 1, 2, 1, 2, "unchanged"))
        for data, uid, expected, expected_ancient, new, new_ancient, message in cases:
            with self.subTest(message=message):
                before = json.dumps(data, sort_keys=True)
                with self.assertRaisesRegex(PlayerSaveEditError, message):
                    set_player_technology_points(
                        data, uid, expected, expected_ancient, new, new_ancient
                    )
                self.assertEqual(before, json.dumps(data, sort_keys=True))

    def test_verify_rejects_changed_value_or_type(self) -> None:
        data = make_player_save(1, 2)
        result = set_player_technology_points(data, PLAYER_UID, 1, 2, 3, 4)
        broken_value = copy.deepcopy(data)
        broken_value["SaveData"]["value"][TECHNOLOGY_FIELD]["value"] = 5
        with self.assertRaisesRegex(PlayerSaveEditError, "persist technology"):
            verify_player_technology_points(broken_value, result)
        broken_type = copy.deepcopy(data)
        broken_type["SaveData"]["value"][ANCIENT_TECHNOLOGY_FIELD]["type"] = "ByteProperty"
        with self.assertRaisesRegex(PlayerSaveEditError, "IntProperty"):
            verify_player_technology_points(broken_type, result)

    def test_all_digit_guid_is_not_misread_as_decimal_uid(self) -> None:
        data = make_player_save(1, 2)
        digit_guid = "00000000-0000-0000-0000-000000000001"
        data["SaveData"]["value"]["PlayerUId"]["value"] = digit_guid

        result = set_player_technology_points(data, digit_guid, 1, 2, 3, 4)

        self.assertEqual(3, result.technology_after)


class PlayerMapMetadataTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.path = Path(self.temporary.name) / "player_map_metadata.json"
        self.payload = {
            "schema": 1,
            "source_commit": PLAYER_MAP_SOURCE_COMMIT,
            "game_version": PLAYER_MAP_GAME_VERSION,
            "fast_travel_guids": list(FAST_TRAVEL_GUIDS),
            "areas": list(AREA_IDS),
            "world_flags": list(WORLD_MAP_FLAGS),
        }

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def write_payload(self, payload: dict) -> None:
        self.path.write_text(json.dumps(payload), encoding="utf-8")

    def test_loads_fixed_1_0_0_catalog(self) -> None:
        self.write_payload(self.payload)

        metadata = load_player_map_metadata(str(self.path))

        self.assertEqual(PLAYER_MAP_SOURCE_COMMIT, metadata.source_commit)
        self.assertEqual("1.0.0", metadata.game_version)
        self.assertEqual(EXPECTED_FAST_TRAVEL_COUNT, len(metadata.fast_travel_guids))
        self.assertEqual(EXPECTED_AREA_COUNT, len(metadata.areas))
        self.assertEqual(WORLD_MAP_FLAGS, metadata.world_flags)

    def test_rejects_wrong_version_counts_duplicates_and_invalid_guids(self) -> None:
        cases: list[tuple[dict, str]] = []

        wrong_version = copy.deepcopy(self.payload)
        wrong_version["game_version"] = "0.6.8"
        cases.append((wrong_version, "game version"))

        missing_guid = copy.deepcopy(self.payload)
        missing_guid["fast_travel_guids"].pop()
        cases.append((missing_guid, "exactly 174"))

        duplicate_guid = copy.deepcopy(self.payload)
        duplicate_guid["fast_travel_guids"][-1] = duplicate_guid[
            "fast_travel_guids"
        ][0]
        cases.append((duplicate_guid, "duplicates fast travel"))

        invalid_guid = copy.deepcopy(self.payload)
        invalid_guid["fast_travel_guids"][0] = "not-a-guid"
        cases.append((invalid_guid, "Invalid fast travel GUID"))

        noncanonical_guid = copy.deepcopy(self.payload)
        noncanonical_guid["fast_travel_guids"][9] = noncanonical_guid[
            "fast_travel_guids"
        ][9].lower()
        cases.append((noncanonical_guid, "uppercase GUID"))

        duplicate_area = copy.deepcopy(self.payload)
        duplicate_area["areas"][-1] = duplicate_area["areas"][0]
        cases.append((duplicate_area, "duplicates area"))

        wrong_world_flags = copy.deepcopy(self.payload)
        wrong_world_flags["world_flags"] = ["Tree", "MainMap"]
        cases.append((wrong_world_flags, "MainMap and Tree"))

        for payload, message in cases:
            with self.subTest(message=message):
                self.write_payload(payload)
                with self.assertRaisesRegex(PlayerSaveEditError, message):
                    load_player_map_metadata(str(self.path))


class PlayerMapEditorTest(unittest.TestCase):
    def test_reads_counts_and_uses_order_independent_normalized_digest(self) -> None:
        data = make_player_map_save(
            fast_travel=[
                {"key": FAST_TRAVEL_GUIDS[0].lower(), "value": True},
                {"key": FAST_TRAVEL_GUIDS[1], "value": False},
                {"key": f"{9999:032X}", "value": True},
                {"key": "ModFastTravel", "value": False},
            ],
            areas=[
                {"key": AREA_IDS[0], "value": True},
                {"key": AREA_IDS[1], "value": False},
                {"key": "ModArea", "value": True},
            ],
            world_maps=[
                {"key": "MainMap", "value": True},
                {"key": "Tree", "value": False},
                {"key": "ModMap", "value": True},
            ],
        )

        progress = read_player_map_progress(data, MAP_METADATA)
        reordered = copy.deepcopy(data)
        record_data = reordered["SaveData"]["value"]["RecordData"]["value"]
        for field in (FAST_TRAVEL_FIELD, AREA_FIELD, WORLD_MAP_FIELD):
            record_data[field]["value"].reverse()
        reordered_progress = read_player_map_progress(reordered, MAP_METADATA)

        self.assertEqual(1, progress.fast_travel_unlocked)
        self.assertEqual(EXPECTED_FAST_TRAVEL_COUNT, progress.fast_travel_total)
        self.assertEqual(1, progress.areas_found)
        self.assertEqual(EXPECTED_AREA_COUNT, progress.areas_total)
        self.assertEqual(1, progress.world_maps_unlocked)
        self.assertEqual(2, progress.world_maps_total)
        self.assertEqual(64, len(progress.progress_digest))
        self.assertEqual(progress.progress_digest, reordered_progress.progress_digest)
        self.assertEqual(
            {
                "fast_travel_unlocked",
                "fast_travel_total",
                "areas_found",
                "areas_total",
                "world_maps_unlocked",
                "world_maps_total",
                "progress_digest",
                "game_version",
            },
            set(progress.to_dict()),
        )

    def test_unlocks_all_targets_preserves_unknowns_and_creates_missing_maps(self) -> None:
        unknown_entry = {
            "key": f"{9999:032X}",
            "value": False,
            "mod_metadata": {"keep": True},
        }
        data = make_player_map_save(
            fast_travel=[
                {"key": FAST_TRAVEL_GUIDS[0], "value": True},
                {"key": FAST_TRAVEL_GUIDS[1], "value": False},
                unknown_entry,
            ],
            include_areas=False,
            include_world_maps=False,
        )
        fast_travel_property = data["SaveData"]["value"]["RecordData"]["value"][
            FAST_TRAVEL_FIELD
        ]
        fast_travel_property["mod_property_metadata"] = "keep"
        non_target_before = without_map_fields(data)
        before = read_player_map_progress(data, MAP_METADATA)

        result = unlock_player_map(
            data,
            PLAYER_UID_DECIMAL,
            MAP_METADATA,
            before.progress_digest.upper(),
        )

        progress = read_player_map_progress(data, MAP_METADATA)
        record_data = data["SaveData"]["value"]["RecordData"]["value"]
        self.assertEqual(EXPECTED_FAST_TRAVEL_COUNT, progress.fast_travel_unlocked)
        self.assertEqual(EXPECTED_AREA_COUNT, progress.areas_found)
        self.assertEqual(2, progress.world_maps_unlocked)
        self.assertEqual((AREA_FIELD, WORLD_MAP_FIELD), result.created_fields)
        self.assertEqual(1, result.fast_travel_before)
        self.assertEqual(EXPECTED_FAST_TRAVEL_COUNT, result.fast_travel_after)
        self.assertEqual(0, result.areas_before)
        self.assertEqual(EXPECTED_AREA_COUNT, result.areas_after)
        self.assertEqual(before.progress_digest, result.progress_digest_before)
        self.assertEqual(progress.progress_digest, result.progress_digest_after)
        self.assertEqual("keep", record_data[FAST_TRAVEL_FIELD]["mod_property_metadata"])
        self.assertEqual(
            unknown_entry,
            next(
                entry
                for entry in record_data[FAST_TRAVEL_FIELD]["value"]
                if entry["key"] == unknown_entry["key"]
            ),
        )
        self.assertEqual(non_target_before, without_map_fields(data))
        for field in (AREA_FIELD, WORLD_MAP_FIELD):
            self.assertEqual("MapProperty", record_data[field]["type"])
            self.assertEqual("NameProperty", record_data[field]["key_type"])
            self.assertEqual("BoolProperty", record_data[field]["value_type"])
            self.assertIsNone(record_data[field]["key_struct_type"])
            self.assertIsNone(record_data[field]["value_struct_type"])
        self.assertEqual(
            {
                "player_uid",
                "fast_travel_before",
                "fast_travel_after",
                "fast_travel_total",
                "areas_before",
                "areas_after",
                "areas_total",
                "world_maps_before",
                "world_maps_after",
                "world_maps_total",
                "progress_digest_before",
                "progress_digest_after",
                "game_version",
                "created_fields",
            },
            set(result.to_dict()),
        )
        verify_player_map_unlock(data, MAP_METADATA, result)

    def test_stale_digest_invalid_digest_and_uid_mismatch_do_not_mutate(self) -> None:
        for expected_digest, uid, message in (
            ("0" * 64, PLAYER_UID, "progress changed"),
            ("not-a-digest", PLAYER_UID, "digest is invalid"),
            (None, "00000001", "does not match"),
        ):
            with self.subTest(message=message):
                data = make_player_map_save()
                before_progress = read_player_map_progress(data, MAP_METADATA)
                before = copy.deepcopy(data)
                digest = (
                    before_progress.progress_digest
                    if expected_digest is None
                    else expected_digest
                )
                with self.assertRaisesRegex(PlayerSaveEditError, message):
                    unlock_player_map(data, uid, MAP_METADATA, digest)
                self.assertEqual(before, data)

    def test_cas_digest_tracks_unknown_map_entries(self) -> None:
        data = make_player_map_save(
            fast_travel=[{"key": "ModFastTravel", "value": False}]
        )
        before = read_player_map_progress(data, MAP_METADATA)
        data["SaveData"]["value"]["RecordData"]["value"][FAST_TRAVEL_FIELD][
            "value"
        ][0]["value"] = True
        changed = copy.deepcopy(data)

        with self.assertRaisesRegex(PlayerSaveConflictError, "progress changed"):
            unlock_player_map(
                data,
                PLAYER_UID,
                MAP_METADATA,
                before.progress_digest,
            )

        self.assertEqual(changed, data)

    def test_already_fully_unlocked_is_rejected_without_mutation(self) -> None:
        data = make_player_map_save()
        before = read_player_map_progress(data, MAP_METADATA)
        result = unlock_player_map(
            data, PLAYER_UID, MAP_METADATA, before.progress_digest
        )
        fully_unlocked = copy.deepcopy(data)

        with self.assertRaisesRegex(PlayerSaveEditError, "already fully unlocked"):
            unlock_player_map(
                data,
                PLAYER_UID,
                MAP_METADATA,
                result.progress_digest_after,
            )

        self.assertEqual(fully_unlocked, data)

    def test_rejects_malformed_map_properties_and_duplicate_keys(self) -> None:
        wrong_type = make_player_map_save()
        wrong_type["SaveData"]["value"]["RecordData"]["value"][
            FAST_TRAVEL_FIELD
        ]["value_type"] = "IntProperty"

        duplicate = make_player_map_save(
            fast_travel=[
                {"key": FAST_TRAVEL_GUIDS[0], "value": True},
                {"key": FAST_TRAVEL_GUIDS[0].lower(), "value": False},
            ]
        )

        non_boolean = make_player_map_save(
            areas=[{"key": AREA_IDS[0], "value": 1}]
        )

        missing_struct_shape = make_player_map_save()
        del missing_struct_shape["SaveData"]["value"]["RecordData"]["value"][
            WORLD_MAP_FIELD
        ]["key_struct_type"]

        for data, message in (
            (wrong_type, "NameProperty keys to BoolProperty"),
            (duplicate, "duplicate key"),
            (non_boolean, "non-boolean"),
            (missing_struct_shape, "NameProperty keys to BoolProperty"),
        ):
            with self.subTest(message=message):
                before = copy.deepcopy(data)
                with self.assertRaisesRegex(PlayerSaveEditError, message):
                    read_player_map_progress(data, MAP_METADATA)
                self.assertEqual(before, data)

    def test_verify_detects_target_structure_and_non_target_changes(self) -> None:
        data = make_player_map_save()
        before = read_player_map_progress(data, MAP_METADATA)
        result = unlock_player_map(
            data, PLAYER_UID, MAP_METADATA, before.progress_digest
        )

        changed_progress = copy.deepcopy(data)
        changed_progress["SaveData"]["value"]["RecordData"]["value"][
            FAST_TRAVEL_FIELD
        ]["value"][0]["value"] = False
        with self.assertRaisesRegex(PlayerSaveEditError, "persist full map"):
            verify_player_map_unlock(changed_progress, MAP_METADATA, result)

        reordered_progress = copy.deepcopy(data)
        reordered_progress["SaveData"]["value"]["RecordData"]["value"][
            FAST_TRAVEL_FIELD
        ]["value"].reverse()
        with self.assertRaisesRegex(PlayerSaveEditError, "expected map progress structure"):
            verify_player_map_unlock(reordered_progress, MAP_METADATA, result)

        changed_unrelated = copy.deepcopy(data)
        changed_unrelated["SaveData"]["value"]["UnrelatedSaveField"]["value"][
            "values"
        ].append("changed")
        with self.assertRaisesRegex(PlayerSaveEditError, "outside map progress"):
            verify_player_map_unlock(changed_unrelated, MAP_METADATA, result)


if __name__ == "__main__":
    unittest.main()
