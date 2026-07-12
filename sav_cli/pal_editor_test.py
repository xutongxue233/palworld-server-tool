import copy
import json
import tempfile
import unittest
from pathlib import Path

from pal_editor import (
    PalConflictError,
    PalEditError,
    PalLevelMetadata,
    load_pal_exp_table,
    load_pal_level_metadata,
    rename_pal,
    restore_pal_health,
    set_pal_level,
    verify_pal_health,
    verify_pal_level,
    verify_pal_nickname,
)


PLAYER_UID = "7e516548-0000-0000-0000-000000000000"
PLAYER_UID_DECIMAL = "2119263560"
INSTANCE_ID = "12345678-1234-5678-9abc-def012345678"
OTHER_INSTANCE_ID = "87654321-4321-8765-cba9-876543210fed"


def fixed_point(value: int) -> dict:
    return {
        "id": None,
        "type": "StructProperty",
        "struct_type": "FixedPoint64",
        "struct_id": "00000000-0000-0000-0000-000000000000",
        "value": {
            "Value": {"id": None, "type": "Int64Property", "value": value}
        },
    }


def byte_property(value: int) -> dict:
    return {
        "id": None,
        "type": "ByteProperty",
        "value": {"type": "None", "value": value},
    }


def pal_entry(
    instance_id: str,
    nickname: str | None,
    owner_uid: str = PLAYER_UID,
    level: int = 3,
    exp: int = 275,
) -> dict:
    record = {
        "CharacterID": {"id": None, "type": "NameProperty", "value": "SheepBall"},
        "Level": byte_property(level),
        "Exp": {"id": None, "type": "Int64Property", "value": exp},
        "Hp": fixed_point(700000),
        "MaxHP": fixed_point(710000),
        "Talent_HP": byte_property(10),
        "Rank_HP": byte_property(2),
        "Rank": byte_property(3),
        "FriendshipPoint": {"id": None, "type": "IntProperty", "value": 7000},
        "bIsAwakening": {"id": None, "type": "BoolProperty", "value": True},
        "OwnerPlayerUId": {
            "id": None,
            "type": "StructProperty",
            "struct_type": "Guid",
            "struct_id": "00000000-0000-0000-0000-000000000000",
            "value": owner_uid,
        },
        "PassiveSkillList": {
            "id": None,
            "type": "ArrayProperty",
            "array_type": "NameProperty",
            "value": {"values": ["PassiveSkill"]},
        },
    }
    if nickname is not None:
        record["NickName"] = {
            "id": None,
            "type": "StrProperty",
            "value": nickname,
        }
    return {
        "key": {
            "PlayerUId": {
                "type": "StructProperty",
                "value": "00000000-0000-0000-0000-000000000000",
            },
            "InstanceId": {"type": "StructProperty", "value": instance_id},
        },
        "value": {
            "RawData": {
                "value": {
                    "object": {
                        "SaveParameter": {
                            "struct_type": "PalIndividualCharacterSaveParameter",
                            "value": record,
                        }
                    }
                }
            }
        },
    }


def make_level(nickname: str | None = "Old Pal") -> dict:
    return {
        "worldSaveData": {
            "value": {
                "CharacterSaveParameterMap": {
                    "value": [
                        pal_entry(INSTANCE_ID, nickname),
                        pal_entry(OTHER_INSTANCE_ID, "Other Pal"),
                    ]
                }
            }
        }
    }


def entries(data: dict) -> list[dict]:
    return data["worldSaveData"]["value"]["CharacterSaveParameterMap"]["value"]


def record(entry: dict) -> dict:
    return entry["value"]["RawData"]["value"]["object"]["SaveParameter"][
        "value"
    ]


PAL_EXP_TABLE = {level: level * 100 for level in range(1, 81)}
PAL_LEVEL_METADATA = {"sheepball": PalLevelMetadata(100.0, 4.0)}
FRIENDSHIP_THRESHOLDS = (
    0,
    6000,
    13000,
    21000,
    30000,
    40000,
    55000,
    80000,
    110000,
    150000,
    200000,
)


class PalEditorTest(unittest.TestCase):
    def test_rename_changes_only_target_nickname(self) -> None:
        data = make_level()
        before = copy.deepcopy(data)

        result = rename_pal(
            data,
            PLAYER_UID_DECIMAL,
            INSTANCE_ID.replace("-", ""),
            "Old Pal",
            3,
            275,
            "New Pal",
        )

        edited = record(entries(data)[0])
        original = record(entries(before)[0])
        self.assertEqual("New Pal", edited["NickName"]["value"])
        edited_without_name = copy.deepcopy(edited)
        original_without_name = copy.deepcopy(original)
        del edited_without_name["NickName"]
        del original_without_name["NickName"]
        self.assertEqual(original_without_name, edited_without_name)
        self.assertEqual(entries(before)[1], entries(data)[1])
        self.assertEqual(INSTANCE_ID, result.instance_id)
        verify_pal_nickname(data, result)

    def test_missing_nickname_is_empty_and_created_with_standard_property(self) -> None:
        data = make_level(nickname=None)

        result = rename_pal(
            data,
            PLAYER_UID_DECIMAL,
            INSTANCE_ID,
            "",
            3,
            275,
            "Named",
        )

        self.assertTrue(result.nickname_created)
        self.assertEqual(
            {"id": None, "type": "StrProperty", "value": "Named"},
            record(entries(data)[0])["NickName"],
        )
        verify_pal_nickname(data, result)

    def test_existing_nickname_can_be_cleared(self) -> None:
        data = make_level()
        result = rename_pal(
            data,
            PLAYER_UID_DECIMAL,
            INSTANCE_ID,
            "Old Pal",
            3,
            275,
            "",
        )
        self.assertEqual("", record(entries(data)[0])["NickName"]["value"])
        verify_pal_nickname(data, result)

    def test_legacy_int_level_and_exp_fields_are_read_without_rewriting(self) -> None:
        data = make_level(nickname=None)
        target = record(entries(data)[0])
        target["Level"] = {"id": None, "type": "IntProperty", "value": 3}
        target["Exp"] = {"id": None, "type": "IntProperty", "value": 275}
        before_level = copy.deepcopy(target["Level"])
        before_exp = copy.deepcopy(target["Exp"])

        result = rename_pal(
            data,
            PLAYER_UID,
            INSTANCE_ID,
            "",
            3,
            275,
            "Legacy",
        )

        self.assertEqual(before_level, target["Level"])
        self.assertEqual(before_exp, target["Exp"])
        verify_pal_nickname(data, result)

    def test_stale_profile_and_owner_are_rejected_without_mutation(self) -> None:
        cases = (
            ("nickname", "Other", 3, 275, PLAYER_UID, "nickname changed"),
            ("level", "Old Pal", 2, 275, PLAYER_UID, "level changed"),
            ("exp", "Old Pal", 3, 274, PLAYER_UID, "EXP changed"),
            (
                "owner",
                "Old Pal",
                3,
                275,
                "00000001-0000-0000-0000-000000000000",
                "owner changed",
            ),
        )
        for name, nickname, level, exp, owner_uid, message in cases:
            with self.subTest(name=name):
                data = make_level()
                record(entries(data)[0])["OwnerPlayerUId"]["value"] = owner_uid
                before = json.dumps(data, sort_keys=True)
                with self.assertRaisesRegex(PalConflictError, message):
                    rename_pal(
                        data,
                        PLAYER_UID_DECIMAL,
                        INSTANCE_ID,
                        nickname,
                        level,
                        exp,
                        "New Pal",
                    )
                self.assertEqual(before, json.dumps(data, sort_keys=True))

    def test_invalid_zero_missing_duplicate_and_player_records_are_rejected(self) -> None:
        with self.assertRaisesRegex(PalEditError, "cannot be zero"):
            rename_pal(
                make_level(),
                PLAYER_UID_DECIMAL,
                "00000000-0000-0000-0000-000000000000",
                "Old Pal",
                3,
                275,
                "New Pal",
            )

        for mutation, message in (
            ("missing", "found 0"),
            ("duplicate", "found 2"),
            ("player", "found 0"),
        ):
            with self.subTest(mutation=mutation):
                data = make_level()
                if mutation == "missing":
                    entries(data).pop(0)
                elif mutation == "duplicate":
                    entries(data).append(copy.deepcopy(entries(data)[0]))
                else:
                    record(entries(data)[0])["IsPlayer"] = {
                        "type": "BoolProperty",
                        "value": True,
                    }
                with self.assertRaisesRegex(PalEditError, message):
                    rename_pal(
                        data,
                        PLAYER_UID_DECIMAL,
                        INSTANCE_ID,
                        "Old Pal",
                        3,
                        275,
                        "New Pal",
                    )

    def test_validation_rejects_unchanged_control_characters_and_stale_level_range(self) -> None:
        for nickname, level, message in (
            ("Old Pal", 3, "unchanged"),
            ("Bad\nName", 3, "control characters"),
            ("New Pal", 81, "between 1 and 80"),
        ):
            with self.subTest(nickname=nickname, level=level):
                with self.assertRaisesRegex(PalEditError, message):
                    rename_pal(
                        make_level(),
                        PLAYER_UID_DECIMAL,
                        INSTANCE_ID,
                        "Old Pal",
                        level,
                        275,
                        nickname,
                    )

    def test_verify_rejects_profile_owner_type_and_count_changes(self) -> None:
        for mutation, message in (
            ("nickname", "only the Pal nickname"),
            ("owner", "owner"),
            ("type", "type"),
            ("count", "record count"),
        ):
            with self.subTest(mutation=mutation):
                data = make_level()
                result = rename_pal(
                    data,
                    PLAYER_UID_DECIMAL,
                    INSTANCE_ID,
                    "Old Pal",
                    3,
                    275,
                    "New Pal",
                )
                target = record(entries(data)[0])
                if mutation == "nickname":
                    target["NickName"]["value"] = "Other"
                elif mutation == "owner":
                    target["OwnerPlayerUId"]["value"] = (
                        "00000001-0000-0000-0000-000000000000"
                    )
                elif mutation == "type":
                    target["CharacterID"]["value"] = "ChickenPal"
                else:
                    entries(data).append(copy.deepcopy(entries(data)[0]))
                with self.assertRaisesRegex(PalEditError, message):
                    verify_pal_nickname(data, result)


class PalLevelEditorTest(unittest.TestCase):
    def test_level_edit_updates_only_target_level_exp_and_health(self) -> None:
        data = make_level()
        before = copy.deepcopy(data)

        result = set_pal_level(
            data,
            PLAYER_UID_DECIMAL,
            INSTANCE_ID,
            "Old Pal",
            3,
            275,
            700000,
            710000,
            4,
            PAL_EXP_TABLE,
            PAL_LEVEL_METADATA,
            FRIENDSHIP_THRESHOLDS,
        )

        edited = record(entries(data)[0])
        original = record(entries(before)[0])
        self.assertEqual(4, edited["Level"]["value"]["value"])
        self.assertEqual(400, edited["Exp"]["value"])
        self.assertEqual(887000, edited["Hp"]["value"]["Value"]["value"])
        self.assertEqual(887000, edited["MaxHP"]["value"]["Value"]["value"])
        for field in (
            "CharacterID",
            "NickName",
            "OwnerPlayerUId",
            "Talent_HP",
            "Rank_HP",
            "Rank",
            "FriendshipPoint",
            "bIsAwakening",
            "PassiveSkillList",
        ):
            self.assertEqual(original[field], edited[field])
        self.assertEqual(entries(before)[1], entries(data)[1])
        self.assertEqual(887000, result.hp_after)
        self.assertFalse(result.max_hp_created)
        verify_pal_level(data, result)

    def test_legacy_int_properties_and_uppercase_hp_keep_their_shapes(self) -> None:
        data = make_level()
        target = record(entries(data)[0])
        target["Level"] = {"id": None, "type": "IntProperty", "value": 3}
        target["Exp"] = {"id": None, "type": "IntProperty", "value": 275}
        target["HP"] = target.pop("Hp")

        result = set_pal_level(
            data,
            PLAYER_UID,
            INSTANCE_ID,
            "Old Pal",
            3,
            275,
            700000,
            710000,
            4,
            PAL_EXP_TABLE,
            PAL_LEVEL_METADATA,
            FRIENDSHIP_THRESHOLDS,
        )

        self.assertEqual({"id": None, "type": "IntProperty", "value": 4}, target["Level"])
        self.assertEqual({"id": None, "type": "IntProperty", "value": 400}, target["Exp"])
        self.assertIn("HP", target)
        self.assertNotIn("Hp", target)
        self.assertEqual("HP", result.health_field)
        verify_pal_level(data, result)

    def test_missing_max_hp_is_created_from_existing_health_shape(self) -> None:
        data = make_level()
        target = record(entries(data)[0])
        del target["MaxHP"]
        hp_shape = copy.deepcopy(target["Hp"])

        result = set_pal_level(
            data,
            PLAYER_UID_DECIMAL,
            INSTANCE_ID,
            "Old Pal",
            3,
            275,
            700000,
            0,
            4,
            PAL_EXP_TABLE,
            PAL_LEVEL_METADATA,
            FRIENDSHIP_THRESHOLDS,
        )

        hp_shape["value"]["Value"]["value"] = 887000
        self.assertEqual(hp_shape, target["Hp"])
        self.assertEqual(hp_shape, target["MaxHP"])
        self.assertTrue(result.max_hp_created)
        verify_pal_level(data, result)

    def test_stale_level_exp_and_health_are_rejected_without_mutation(self) -> None:
        cases = (
            ("nickname", "Other", 3, 275, 700000, 710000, "nickname changed"),
            ("level", "Old Pal", 2, 275, 700000, 710000, "level changed"),
            ("exp", "Old Pal", 3, 274, 700000, 710000, "EXP changed"),
            ("hp", "Old Pal", 3, 275, 699999, 710000, "HP changed"),
            ("max", "Old Pal", 3, 275, 700000, 709999, "MaxHP changed"),
        )
        for name, nickname, level, exp, hp, max_hp, message in cases:
            with self.subTest(name=name):
                data = make_level()
                before = json.dumps(data, sort_keys=True)
                with self.assertRaisesRegex(PalConflictError, message):
                    set_pal_level(
                        data,
                        PLAYER_UID_DECIMAL,
                        INSTANCE_ID,
                        nickname,
                        level,
                        exp,
                        hp,
                        max_hp,
                        4,
                        PAL_EXP_TABLE,
                        PAL_LEVEL_METADATA,
                        FRIENDSHIP_THRESHOLDS,
                    )
                self.assertEqual(before, json.dumps(data, sort_keys=True))

    def test_level_validation_and_metadata_fail_before_mutation(self) -> None:
        for new_level, metadata, message in (
            (3, PAL_LEVEL_METADATA, "unchanged"),
            (81, PAL_LEVEL_METADATA, "between 1 and 80"),
            (4, {}, "No level metadata"),
        ):
            with self.subTest(new_level=new_level, metadata=metadata):
                data = make_level()
                before = json.dumps(data, sort_keys=True)
                with self.assertRaisesRegex(PalEditError, message):
                    set_pal_level(
                        data,
                        PLAYER_UID_DECIMAL,
                        INSTANCE_ID,
                        "Old Pal",
                        3,
                        275,
                        700000,
                        710000,
                        new_level,
                        PAL_EXP_TABLE,
                        metadata,
                        FRIENDSHIP_THRESHOLDS,
                    )
                self.assertEqual(before, json.dumps(data, sort_keys=True))

    def test_malformed_formula_inputs_and_duplicate_health_fields_fail_closed(self) -> None:
        for mutation, message in (
            ("talent", "Talent_HP must contain an integer"),
            ("rank_hp", "Rank_HP must be between 0 and 20"),
            ("rank", "Rank must be between 0 and 5"),
            ("health", "both Hp and HP"),
            ("level_shape", "invalid value shape"),
            ("rank_shape", "invalid value shape"),
        ):
            with self.subTest(mutation=mutation):
                data = make_level()
                target = record(entries(data)[0])
                if mutation == "talent":
                    target["Talent_HP"]["value"]["value"] = "10"
                elif mutation == "rank_hp":
                    target["Rank_HP"]["value"]["value"] = 21
                elif mutation == "rank":
                    target["Rank"]["value"]["value"] = 6
                elif mutation == "level_shape":
                    target["Level"]["value"] = 3
                elif mutation == "rank_shape":
                    target["Rank"]["value"] = 3
                else:
                    target["HP"] = copy.deepcopy(target["Hp"])
                before = copy.deepcopy(data)
                with self.assertRaisesRegex(PalEditError, message):
                    set_pal_level(
                        data,
                        PLAYER_UID_DECIMAL,
                        INSTANCE_ID,
                        "Old Pal",
                        3,
                        275,
                        700000,
                        710000,
                        4,
                        PAL_EXP_TABLE,
                        PAL_LEVEL_METADATA,
                        FRIENDSHIP_THRESHOLDS,
                    )
                self.assertEqual(before, data)

    def test_unknown_pal_type_does_not_use_substring_metadata(self) -> None:
        data = make_level()
        target = record(entries(data)[0])
        target["CharacterID"]["value"] = "SheepBall_NewVariant"

        with self.assertRaisesRegex(PalEditError, "No level metadata"):
            set_pal_level(
                data,
                PLAYER_UID_DECIMAL,
                INSTANCE_ID,
                "Old Pal",
                3,
                275,
                700000,
                710000,
                4,
                PAL_EXP_TABLE,
                PAL_LEVEL_METADATA,
                FRIENDSHIP_THRESHOLDS,
            )

    def test_verify_rejects_level_exp_health_identity_and_type_tampering(self) -> None:
        for mutation, message in (
            ("level", "level and EXP"),
            ("health", "health values"),
            ("identity", "identity"),
            ("type", "property type"),
            ("other_field", "unexpected Pal fields"),
            ("other_record", "other character records"),
        ):
            with self.subTest(mutation=mutation):
                data = make_level()
                result = set_pal_level(
                    data,
                    PLAYER_UID_DECIMAL,
                    INSTANCE_ID,
                    "Old Pal",
                    3,
                    275,
                    700000,
                    710000,
                    4,
                    PAL_EXP_TABLE,
                    PAL_LEVEL_METADATA,
                    FRIENDSHIP_THRESHOLDS,
                )
                target = record(entries(data)[0])
                if mutation == "level":
                    target["Level"]["value"]["value"] = 5
                elif mutation == "health":
                    target["MaxHP"]["value"]["Value"]["value"] += 1
                elif mutation == "identity":
                    target["NickName"]["value"] = "Other"
                elif mutation == "other_field":
                    target["PassiveSkillList"]["value"]["values"].append("Other")
                elif mutation == "other_record":
                    record(entries(data)[1])["NickName"]["value"] = "Changed"
                else:
                    target["Exp"]["type"] = "IntProperty"
                with self.assertRaisesRegex(PalEditError, message):
                    verify_pal_level(data, result)


class PalLevelMetadataTest(unittest.TestCase):
    def test_loads_bundled_exp_shape_and_compact_1_0_metadata(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            exp_path = root / "exp.json"
            metadata_path = root / "metadata.json"
            exp_path.write_text(
                json.dumps(
                    {
                        str(level): {"PalTotalEXP": (level - 1) * 25}
                        for level in range(1, 101)
                    }
                ),
                encoding="utf-8",
            )
            metadata_path.write_text(
                json.dumps(
                    {
                        "schema": 1,
                        "game_version": "1.0.0",
                        "max_level": 80,
                        "friendship_thresholds": list(FRIENDSHIP_THRESHOLDS),
                        "pals": {
                            "sheepball": {
                                "hp_scaling": 70,
                                "friendship_hp": 5.5,
                            }
                        },
                    }
                ),
                encoding="utf-8",
            )

            exp = load_pal_exp_table(str(exp_path))
            metadata, thresholds = load_pal_level_metadata(str(metadata_path))

        self.assertEqual(list(range(1, 81)), sorted(exp))
        self.assertEqual(1975, exp[80])
        self.assertEqual(PalLevelMetadata(70.0, 5.5), metadata["sheepball"])
        self.assertEqual(FRIENDSHIP_THRESHOLDS, thresholds)

    def test_rejects_incomplete_exp_and_wrong_game_version(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            exp_path = root / "exp.json"
            metadata_path = root / "metadata.json"
            exp_path.write_text(
                json.dumps({"1": {"PalTotalEXP": 0}}), encoding="utf-8"
            )
            metadata_path.write_text(
                json.dumps(
                    {
                        "schema": 1,
                        "game_version": "0.6.8",
                        "max_level": 80,
                        "friendship_thresholds": [0],
                        "pals": {"sheepball": {"hp_scaling": 70}},
                    }
                ),
                encoding="utf-8",
            )
            with self.assertRaisesRegex(PalEditError, "levels 1 through 80"):
                load_pal_exp_table(str(exp_path))
            with self.assertRaisesRegex(PalEditError, "game version"):
                load_pal_level_metadata(str(metadata_path))

    def test_rejects_truncated_friendship_thresholds(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            metadata_path = Path(temp_dir) / "metadata.json"
            metadata_path.write_text(
                json.dumps(
                    {
                        "schema": 1,
                        "game_version": "1.0.0",
                        "max_level": 80,
                        "friendship_thresholds": [0, 6000, 13000],
                        "pals": {"sheepball": {"hp_scaling": 70}},
                    }
                ),
                encoding="utf-8",
            )

            with self.assertRaisesRegex(PalEditError, "friendship thresholds"):
                load_pal_level_metadata(str(metadata_path))


class PalHealthEditorTest(unittest.TestCase):
    def test_restore_changes_only_target_current_health(self) -> None:
        data = make_level()
        before = copy.deepcopy(data)

        result = restore_pal_health(
            data,
            PLAYER_UID_DECIMAL,
            INSTANCE_ID,
            "Old Pal",
            3,
            275,
            700000,
            710000,
        )

        target = record(entries(data)[0])
        expected = record(entries(before)[0])
        expected["Hp"]["value"]["Value"]["value"] = 710000
        self.assertEqual(expected, target)
        self.assertEqual(entries(before)[1], entries(data)[1])
        self.assertEqual(710000, result.hp_after)
        verify_pal_health(data, result)

    def test_restore_preserves_legacy_uppercase_health_shape(self) -> None:
        data = make_level()
        target = record(entries(data)[0])
        target["HP"] = target.pop("Hp")

        result = restore_pal_health(
            data,
            PLAYER_UID,
            INSTANCE_ID,
            "Old Pal",
            3,
            275,
            700000,
            710000,
        )

        self.assertEqual("HP", result.health_field)
        self.assertEqual(710000, target["HP"]["value"]["Value"]["value"])
        self.assertNotIn("Hp", target)
        verify_pal_health(data, result)

    def test_invalid_or_unrestorable_health_fails_without_mutation(self) -> None:
        for mutation, expected_max_hp, error_type, message in (
            ("missing", 710000, PalConflictError, "missing MaxHP"),
            ("full", 710000, PalEditError, "already full"),
            ("overflow", 710000, PalEditError, "cannot exceed MaxHP"),
            ("duplicate", 710000, PalEditError, "both Hp and HP"),
            ("expected_missing", 0, PalEditError, "must be positive"),
        ):
            with self.subTest(mutation=mutation):
                data = make_level()
                target = record(entries(data)[0])
                expected_hp = 700000
                if mutation == "missing":
                    del target["MaxHP"]
                elif mutation == "full":
                    target["Hp"]["value"]["Value"]["value"] = 710000
                    expected_hp = 710000
                elif mutation == "overflow":
                    target["Hp"]["value"]["Value"]["value"] = 720000
                    expected_hp = 720000
                elif mutation == "duplicate":
                    target["HP"] = copy.deepcopy(target["Hp"])
                before = copy.deepcopy(data)

                with self.assertRaisesRegex(error_type, message):
                    restore_pal_health(
                        data,
                        PLAYER_UID_DECIMAL,
                        INSTANCE_ID,
                        "Old Pal",
                        3,
                        275,
                        expected_hp,
                        expected_max_hp,
                    )
                self.assertEqual(before, data)

    def test_missing_target_record_is_a_conflict(self) -> None:
        data = make_level()
        entries(data).pop(0)
        before = copy.deepcopy(data)

        with self.assertRaisesRegex(PalConflictError, "found 0"):
            restore_pal_health(
                data,
                PLAYER_UID_DECIMAL,
                INSTANCE_ID,
                "Old Pal",
                3,
                275,
                700000,
                710000,
            )

        self.assertEqual(before, data)

    def test_stale_health_identity_and_owner_fail_without_mutation(self) -> None:
        cases = (
            (
                "nickname",
                "Other",
                3,
                275,
                700000,
                710000,
                PLAYER_UID,
                "nickname changed",
            ),
            (
                "level",
                "Old Pal",
                2,
                275,
                700000,
                710000,
                PLAYER_UID,
                "level changed",
            ),
            ("exp", "Old Pal", 3, 274, 700000, 710000, PLAYER_UID, "EXP changed"),
            ("hp", "Old Pal", 3, 275, 699999, 710000, PLAYER_UID, "HP changed"),
            (
                "max",
                "Old Pal",
                3,
                275,
                700000,
                709999,
                PLAYER_UID,
                "MaxHP changed",
            ),
            (
                "owner",
                "Old Pal",
                3,
                275,
                700000,
                710000,
                "00000001-0000-0000-0000-000000000000",
                "owner changed",
            ),
        )
        for name, nickname, level, exp, hp, max_hp, owner, message in cases:
            with self.subTest(name=name):
                data = make_level()
                record(entries(data)[0])["OwnerPlayerUId"]["value"] = owner
                before = copy.deepcopy(data)
                with self.assertRaisesRegex(PalConflictError, message):
                    restore_pal_health(
                        data,
                        PLAYER_UID_DECIMAL,
                        INSTANCE_ID,
                        nickname,
                        level,
                        exp,
                        hp,
                        max_hp,
                    )
                self.assertEqual(before, data)

    def test_verify_rejects_target_and_other_record_tampering(self) -> None:
        for mutation, message in (
            ("target", "restored Pal health"),
            ("other_field", "unexpected Pal fields"),
            ("other_record", "other character records"),
        ):
            with self.subTest(mutation=mutation):
                data = make_level()
                result = restore_pal_health(
                    data,
                    PLAYER_UID_DECIMAL,
                    INSTANCE_ID,
                    "Old Pal",
                    3,
                    275,
                    700000,
                    710000,
                )
                if mutation == "target":
                    record(entries(data)[0])["Hp"]["value"]["Value"]["value"] -= 1
                elif mutation == "other_field":
                    record(entries(data)[0])["PassiveSkillList"]["value"][
                        "values"
                    ].append("Other")
                else:
                    record(entries(data)[1])["NickName"]["value"] = "Changed"
                with self.assertRaisesRegex(PalEditError, message):
                    verify_pal_health(data, result)

if __name__ == "__main__":
    unittest.main()
