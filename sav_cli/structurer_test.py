from __future__ import annotations

import json
import os
import tempfile
import unittest
import uuid
from types import SimpleNamespace
from unittest.mock import patch

import structurer


class StructurerInventoryTest(unittest.TestCase):
    def test_player_item_dynamic_id_is_json_serializable(self) -> None:
        dynamic_id = uuid.UUID("e8af6046-c152-4c80-b69f-cb04e801be37")
        raw_item = {
            "RawData": {
                "value": {
                    "slot_index": 0,
                    "count": 1,
                    "item": {
                        "static_id": "AssaultRifle_Default1",
                        "dynamic_id": {
                            "local_id_in_created_world": dynamic_id
                        },
                    },
                }
            }
        }

        item = structurer.serialize_player_item(raw_item)

        self.assertEqual(str(dynamic_id), item["DynamicId"])
        json.dumps(item)

    def test_reads_player_technology_points(self) -> None:
        points = structurer.read_player_technology_points(
            {
                "TechnologyPoint": {
                    "type": "IntProperty",
                    "value": 23,
                },
                "bossTechnologyPoint": {
                    "type": "IntProperty",
                    "value": 7,
                },
            }
        )

        self.assertEqual(
            {
                "technology_points": 23,
                "ancient_technology_points": 7,
            },
            points,
        )

    def test_missing_player_technology_fields_default_to_zero(self) -> None:
        self.assertEqual(
            {
                "technology_points": 0,
                "ancient_technology_points": 0,
            },
            structurer.read_player_technology_points({}),
        )

    def test_bad_player_technology_field_types_are_rejected(self) -> None:
        bad_fields = (
            {"TechnologyPoint": {"type": "UInt32Property", "value": 4}},
            {"TechnologyPoint": {"type": "IntProperty", "value": "4"}},
            {"bossTechnologyPoint": {"type": "IntProperty", "value": True}},
            {"bossTechnologyPoint": {"type": "IntProperty"}},
        )

        for save_data in bad_fields:
            with self.subTest(save_data=save_data):
                with self.assertRaises(ValueError):
                    structurer.read_player_technology_points(save_data)

    def test_player_save_parse_returns_items_and_technology_points(self) -> None:
        player_uid = "7e516548-0000-0000-0000-000000000000"
        map_metadata = object()
        player_gvas = SimpleNamespace(
            properties={
                "SaveData": {
                    "value": {
                        "TechnologyPoint": {
                            "type": "IntProperty",
                            "value": 11,
                        },
                        "bossTechnologyPoint": {
                            "type": "IntProperty",
                            "value": 3,
                        },
                    }
                }
            }
        )
        old_wsd = structurer.wsd
        try:
            structurer.wsd = {"ItemContainerSaveData": {"value": []}}
            with tempfile.TemporaryDirectory() as temp_dir:
                player_path = os.path.join(
                    temp_dir, player_uid.upper().replace("-", "") + ".sav"
                )
                with open(player_path, "wb") as player_file:
                    player_file.write(b"player-save")
                with (
                    patch.object(
                        structurer,
                        "decompress_sav_to_gvas",
                        return_value=(b"gvas", None),
                    ),
                    patch.object(structurer.GvasFile, "read", return_value=player_gvas),
                    patch.object(
                        structurer,
                        "_load_bundled_player_map_metadata",
                        return_value=map_metadata,
                    ),
                    patch.object(
                        structurer,
                        "read_player_map_progress",
                        return_value=SimpleNamespace(
                            to_dict=lambda: {
                                "fast_travel_unlocked": 1,
                                "fast_travel_total": 174,
                                "areas_found": 2,
                                "areas_total": 123,
                                "world_maps_unlocked": 1,
                                "world_maps_total": 2,
                                "progress_digest": "a" * 64,
                                "game_version": "1.0.0",
                            }
                        ),
                    ) as read_map_progress,
                ):
                    result = structurer.getPlayerItems(player_uid, temp_dir)
        finally:
            structurer.wsd = old_wsd

        self.assertIsNotNone(result)
        self.assertEqual(11, result["technology_points"])
        self.assertEqual(3, result["ancient_technology_points"])
        self.assertEqual(1, result["map_progress"]["fast_travel_unlocked"])
        self.assertEqual("a" * 64, result["map_progress"]["progress_digest"])
        self.assertEqual([], result["items"]["CommonContainerId"])
        read_map_progress.assert_called_once_with(player_gvas, map_metadata)

    def test_structure_player_only_exposes_points_after_player_save_parse(self) -> None:
        player_uid = "7e516548-0000-0000-0000-000000000000"
        world = {
            "CharacterSaveParameterMap": {
                "value": [
                    {
                        "key": {"PlayerUId": {"value": player_uid}},
                        "value": {
                            "RawData": {
                                "value": {
                                    "object": {
                                        "SaveParameter": {
                                            "value": {
                                                "IsPlayer": {"value": True},
                                                "GotStatusPointList": {
                                                    "value": {"values": []}
                                                },
                                            }
                                        }
                                    }
                                }
                            }
                        },
                    }
                ]
            },
            "GameTimeSaveData": {
                "value": {"RealDateTimeTicks": {"value": 0}}
            },
        }
        parsed_player_save = {
            "items": {},
            "technology_points": 14,
            "ancient_technology_points": 5,
            "map_progress": {
                "fast_travel_unlocked": 1,
                "fast_travel_total": 174,
                "areas_found": 2,
                "areas_total": 123,
                "world_maps_unlocked": 1,
                "world_maps_total": 2,
                "progress_digest": "b" * 64,
                "game_version": "1.0.0",
            },
        }
        old_wsd = structurer.wsd
        try:
            structurer.wsd = world
            with patch.object(
                structurer, "getPlayerItems", return_value=parsed_player_save
            ):
                parsed_player = structurer.structure_player("", world)[0]
            with patch.object(structurer, "getPlayerItems", return_value=None):
                missing_player = structurer.structure_player("", world)[0]
        finally:
            structurer.wsd = old_wsd

        self.assertEqual(14, parsed_player["technology_points"])
        self.assertEqual(5, parsed_player["ancient_technology_points"])
        self.assertEqual(174, parsed_player["map_progress"]["fast_travel_total"])
        self.assertNotIn("technology_points", missing_player)
        self.assertNotIn("ancient_technology_points", missing_player)
        self.assertNotIn("map_progress", missing_player)

    def test_structure_player_keeps_full_pal_instance_id(self) -> None:
        player_uid = "7e516548-0000-0000-0000-000000000000"
        instance_id = uuid.UUID("c410c416-475c-0638-eb35-269338f2a320")
        player_record = {
            "IsPlayer": {"value": True},
            "GotStatusPointList": {"value": {"values": []}},
        }
        pal_record = {
            "OwnerPlayerUId": {"value": player_uid},
            "CharacterID": {"value": "ChickenPal"},
            "Level": {"value": {"value": 2}},
            "Exp": {"value": 25},
            "Hp": {"value": {"Value": {"value": 583000}}},
        }
        world = {
            "CharacterSaveParameterMap": {
                "value": [
                    {
                        "key": {
                            "PlayerUId": {"value": player_uid},
                            "InstanceId": {"value": uuid.uuid4()},
                        },
                        "value": {
                            "RawData": {
                                "value": {
                                    "object": {
                                        "SaveParameter": {"value": player_record}
                                    }
                                }
                            }
                        },
                    },
                    {
                        "key": {
                            "PlayerUId": {
                                "value": "00000000-0000-0000-0000-000000000000"
                            },
                            "InstanceId": {"value": instance_id},
                        },
                        "value": {
                            "RawData": {
                                "value": {
                                    "object": {
                                        "SaveParameter": {"value": pal_record}
                                    }
                                }
                            }
                        },
                    },
                ]
            },
            "GameTimeSaveData": {
                "value": {"RealDateTimeTicks": {"value": 0}}
            },
        }
        old_wsd = structurer.wsd
        try:
            structurer.wsd = world
            with patch.object(structurer, "getPlayerItems", return_value=None):
                parsed_player = structurer.structure_player("", world)[0]
        finally:
            structurer.wsd = old_wsd

        self.assertEqual(str(instance_id), parsed_player["pals"][0]["instance_id"])
        self.assertNotIn("owner", parsed_player["pals"][0])


if __name__ == "__main__":
    unittest.main()
