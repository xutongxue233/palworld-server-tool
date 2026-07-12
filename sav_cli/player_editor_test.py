from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from player_editor import (
    PlayerConflictError,
    PlayerEditError,
    load_exp_table,
    set_player_profile,
    set_player_stat_points,
    verify_player_profile,
    verify_player_stat_points,
)


PLAYER_UID = "7e516548-0000-0000-0000-000000000000"
PLAYER_UID_DECIMAL = "2119263560"


def make_level(nickname: str = "Old name", level: int = 2, exp: int = 50) -> dict:
    save_parameter = {
        "struct_type": "PalIndividualCharacterSaveParameter",
        "value": {
            "IsPlayer": {"value": True},
            "NickName": {"value": nickname},
            "FilteredNickName": {"value": nickname},
            "Level": {"value": {"value": level}},
            "Exp": {"value": exp},
            "UnusedStatusPoint": {
                "id": None,
                "type": "UInt16Property",
                "value": 1,
            },
            "GotStatusPointList": {"value": {"values": [{"keep": True}]}},
            "GotExStatusPointList": {"value": {"values": [{"keep": True}]}},
        },
    }
    return {
        "worldSaveData": {
            "value": {
                "CharacterSaveParameterMap": {
                    "value": [
                        {
                            "key": {"PlayerUId": {"value": PLAYER_UID}},
                            "value": {
                                "RawData": {
                                    "value": {
                                        "object": {"SaveParameter": save_parameter}
                                    }
                                }
                            },
                        }
                    ]
                },
                "GroupSaveDataMap": {
                    "value": [
                        {
                            "value": {
                                "GroupType": {
                                    "value": {"value": "EPalGroupType::Guild"}
                                },
                                "RawData": {
                                    "value": {
                                        "players": [
                                            {
                                                "player_uid": PLAYER_UID,
                                                "player_info": {
                                                    "player_name": nickname
                                                },
                                            }
                                        ]
                                    }
                                },
                            }
                        }
                    ]
                },
            }
        }
    }


class PlayerEditorTest(unittest.TestCase):
    def test_stat_points_edit_changes_only_unused_value(self) -> None:
        data = make_level()
        before = json.loads(json.dumps(data))

        result = set_player_stat_points(data, PLAYER_UID_DECIMAL, 1, 7)

        character = data["worldSaveData"]["value"]["CharacterSaveParameterMap"][
            "value"
        ][0]["value"]["RawData"]["value"]["object"]["SaveParameter"]["value"]
        self.assertEqual(7, character["UnusedStatusPoint"]["value"])
        self.assertEqual("UInt16Property", character["UnusedStatusPoint"]["type"])
        self.assertEqual(
            before["worldSaveData"]["value"]["CharacterSaveParameterMap"]["value"][
                0
            ]["value"]["RawData"]["value"]["object"]["SaveParameter"]["value"][
                "GotStatusPointList"
            ],
            character["GotStatusPointList"],
        )
        self.assertEqual(1, result.before)
        self.assertEqual(7, result.after)
        verify_player_stat_points(data, result)

    def test_stat_points_edit_rejects_stale_value_without_mutation(self) -> None:
        data = make_level()
        before = json.dumps(data, sort_keys=True)

        with self.assertRaisesRegex(PlayerConflictError, "changed from 0 to 1"):
            set_player_stat_points(data, PLAYER_UID_DECIMAL, 0, 2)

        self.assertEqual(before, json.dumps(data, sort_keys=True))

    def test_stat_points_edit_requires_real_uint16_property(self) -> None:
        for property_value, message in (
            (None, "missing UnusedStatusPoint"),
            ({"type": "IntProperty", "value": 1}, "must be a UInt16Property"),
        ):
            with self.subTest(property_value=property_value):
                data = make_level()
                character = data["worldSaveData"]["value"][
                    "CharacterSaveParameterMap"
                ]["value"][0]["value"]["RawData"]["value"]["object"][
                    "SaveParameter"
                ]["value"]
                if property_value is None:
                    del character["UnusedStatusPoint"]
                else:
                    character["UnusedStatusPoint"] = property_value
                with self.assertRaisesRegex(PlayerEditError, message):
                    set_player_stat_points(data, PLAYER_UID_DECIMAL, 1, 2)

    def test_stat_points_edit_rejects_out_of_range_and_unchanged(self) -> None:
        for expected, value, message in (
            (-1, 2, "Expected unused stat points must be between"),
            (1, 65536, "Unused stat points must be between"),
            (1, 1, "are unchanged"),
        ):
            with self.subTest(expected=expected, value=value):
                with self.assertRaisesRegex(PlayerEditError, message):
                    set_player_stat_points(
                        make_level(),
                        PLAYER_UID_DECIMAL,
                        expected,
                        value,
                    )

    def test_stat_points_edit_requires_exactly_one_player_record(self) -> None:
        data = make_level()
        entries = data["worldSaveData"]["value"]["CharacterSaveParameterMap"][
            "value"
        ]
        entries.append(json.loads(json.dumps(entries[0])))

        with self.assertRaisesRegex(PlayerEditError, "found 2"):
            set_player_stat_points(data, PLAYER_UID_DECIMAL, 1, 2)

    def test_verify_stat_points_rejects_type_value_and_record_count_changes(self) -> None:
        for mutation, message in (
            ("type", "must be a UInt16Property"),
            ("value", "did not persist"),
            ("count", "record count"),
        ):
            with self.subTest(mutation=mutation):
                data = make_level()
                result = set_player_stat_points(data, PLAYER_UID_DECIMAL, 1, 2)
                entries = data["worldSaveData"]["value"][
                    "CharacterSaveParameterMap"
                ]["value"]
                character = entries[0]["value"]["RawData"]["value"]["object"][
                    "SaveParameter"
                ]["value"]
                if mutation == "type":
                    character["UnusedStatusPoint"]["type"] = "IntProperty"
                elif mutation == "value":
                    character["UnusedStatusPoint"]["value"] = 3
                else:
                    entries.append(json.loads(json.dumps(entries[0])))
                with self.assertRaisesRegex(PlayerEditError, message):
                    verify_player_stat_points(data, result)

    def test_profile_edit_updates_character_and_guild(self) -> None:
        data = make_level()
        result = set_player_profile(
            data,
            PLAYER_UID_DECIMAL,
            "Old name",
            2,
            "New name",
            3,
            {1: 0, 2: 50, 3: 125},
        )
        character = data["worldSaveData"]["value"]["CharacterSaveParameterMap"][
            "value"
        ][0]["value"]["RawData"]["value"]["object"]["SaveParameter"]["value"]
        guild_player = data["worldSaveData"]["value"]["GroupSaveDataMap"]["value"][
            0
        ]["value"]["RawData"]["value"]["players"][0]
        self.assertEqual("New name", character["NickName"]["value"])
        self.assertEqual("New name", character["FilteredNickName"]["value"])
        self.assertEqual(3, character["Level"]["value"]["value"])
        self.assertEqual(125, character["Exp"]["value"])
        self.assertEqual("New name", guild_player["player_info"]["player_name"])
        verify_player_profile(data, result)

    def test_nickname_only_edit_preserves_current_level_progress(self) -> None:
        data = make_level(exp=64)
        result = set_player_profile(
            data,
            PLAYER_UID_DECIMAL,
            "Old name",
            2,
            "New name",
            2,
            {1: 0, 2: 50, 3: 125},
        )
        character = data["worldSaveData"]["value"]["CharacterSaveParameterMap"][
            "value"
        ][0]["value"]["RawData"]["value"]["object"]["SaveParameter"]["value"]
        self.assertEqual(64, character["Exp"]["value"])
        self.assertEqual(64, result.exp_after)
        verify_player_profile(data, result)

    def test_profile_edit_rejects_inconsistent_filtered_nickname(self) -> None:
        data = make_level()
        character = data["worldSaveData"]["value"]["CharacterSaveParameterMap"][
            "value"
        ][0]["value"]["RawData"]["value"]["object"]["SaveParameter"]["value"]
        character["FilteredNickName"]["value"] = "Filtered old name"
        with self.assertRaisesRegex(PlayerEditError, "Filtered nickname"):
            set_player_profile(
                data,
                PLAYER_UID_DECIMAL,
                "Old name",
                2,
                "New name",
                3,
                {1: 0, 2: 50, 3: 125},
            )

    def test_profile_edit_rejects_reserved_level(self) -> None:
        data = make_level()
        with self.assertRaisesRegex(PlayerEditError, "between 1 and 80"):
            set_player_profile(
                data,
                PLAYER_UID_DECIMAL,
                "Old name",
                2,
                "New name",
                81,
                {level: level * 50 for level in range(1, 101)},
            )

    def test_profile_edit_rejects_stale_values_without_mutation(self) -> None:
        data = make_level()
        before = json.dumps(data, sort_keys=True)
        with self.assertRaisesRegex(PlayerConflictError, "level changed"):
            set_player_profile(
                data,
                PLAYER_UID,
                "Old name",
                1,
                "New name",
                3,
                {1: 0, 2: 50, 3: 125},
            )
        self.assertEqual(before, json.dumps(data, sort_keys=True))

    def test_profile_edit_rejects_inconsistent_guild_name(self) -> None:
        data = make_level()
        data["worldSaveData"]["value"]["GroupSaveDataMap"]["value"][0]["value"][
            "RawData"
        ]["value"]["players"][0]["player_info"]["player_name"] = "Other"
        with self.assertRaisesRegex(PlayerEditError, "does not match"):
            set_player_profile(
                data,
                PLAYER_UID,
                "Old name",
                2,
                "New name",
                3,
                {1: 0, 2: 50, 3: 125},
            )

    def test_exp_table_validation(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "exp.json"
            path.write_text(
                json.dumps(
                    {
                        "1": {"TotalEXP": 0},
                        "2": {"TotalEXP": 50},
                        "3": {"TotalEXP": 125},
                    }
                ),
                encoding="utf-8",
            )
            table = load_exp_table(str(path))
        self.assertEqual({1: 0, 2: 50, 3: 125}, table)


if __name__ == "__main__":
    unittest.main()
