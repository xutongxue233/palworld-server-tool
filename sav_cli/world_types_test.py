from __future__ import annotations

import unittest

from world_types import Pal, Player


class PlayerWorldTypeTest(unittest.TestCase):
    @staticmethod
    def player_data(**overrides):
        data = {
            "GotStatusPointList": {"value": {"values": []}},
            "Items": None,
        }
        data.update(overrides)
        return data

    def test_serializes_unused_status_points(self) -> None:
        player = Player(
            "7e516548-0000-0000-0000-000000000000",
            self.player_data(UnusedStatusPoint={"value": 17}),
        )

        self.assertEqual(17, player.to_dict()["unused_status_points"])

    def test_missing_unused_status_points_stays_unknown(self) -> None:
        player = Player(
            "7e516548-0000-0000-0000-000000000000",
            self.player_data(),
        )

        self.assertIsNone(player.to_dict()["unused_status_points"])

    def test_serializes_technology_points_from_successful_player_save(self) -> None:
        map_progress = {
            "fast_travel_unlocked": 1,
            "fast_travel_total": 174,
            "areas_found": 2,
            "areas_total": 123,
            "world_maps_unlocked": 1,
            "world_maps_total": 2,
            "progress_digest": "a" * 64,
            "game_version": "1.0.0",
        }
        player = Player(
            "7e516548-0000-0000-0000-000000000000",
            self.player_data(
                PlayerSaveData={
                    "items": {},
                    "technology_points": 19,
                    "ancient_technology_points": 6,
                    "map_progress": map_progress,
                }
            ),
        )

        serialized = player.to_dict()
        self.assertEqual(19, serialized["technology_points"])
        self.assertEqual(6, serialized["ancient_technology_points"])
        self.assertEqual(map_progress, serialized["map_progress"])

    def test_omits_technology_points_without_successful_player_save(self) -> None:
        serialized = Player(
            "7e516548-0000-0000-0000-000000000000",
            self.player_data(),
        ).to_dict()

        self.assertNotIn("technology_points", serialized)
        self.assertNotIn("ancient_technology_points", serialized)
        self.assertNotIn("map_progress", serialized)


class PalWorldTypeTest(unittest.TestCase):
    @staticmethod
    def pal_data(health_field: str) -> dict:
        return {
            "OwnerPlayerUId": {
                "value": "7e516548-0000-0000-0000-000000000000"
            },
            "Level": {"value": {"value": 2}},
            "Exp": {"value": 25},
            health_field: {"value": {"Value": {"value": 583000}}},
            "MaxHP": {"value": {"Value": {"value": 600000}}},
        }

    def test_serializes_full_instance_id_and_modern_hp_field(self) -> None:
        instance_id = "c410c416-475c-0638-eb35-269338f2a320"
        serialized = Pal(instance_id, self.pal_data("Hp"), 0, 0).to_dict()

        self.assertEqual(instance_id, serialized["instance_id"])
        self.assertEqual(583000, serialized["hp"])
        self.assertEqual(600000, serialized["max_hp"])

    def test_legacy_uppercase_hp_field_remains_supported(self) -> None:
        serialized = Pal(
            "c410c416-475c-0638-eb35-269338f2a320",
            self.pal_data("HP"),
            0,
            0,
        ).to_dict()

        self.assertEqual(583000, serialized["hp"])

    def test_legacy_scalar_level_exp_and_stats_remain_supported(self) -> None:
        data = self.pal_data("HP")
        data.update(
            {
                "Level": {"type": "IntProperty", "value": 7},
                "Exp": {"type": "IntProperty", "value": 1234},
                "Talent_Shot": {"type": "IntProperty", "value": 88},
                "Talent_Defense": {"type": "IntProperty", "value": 77},
                "Rank": {"type": "IntProperty", "value": 4},
            }
        )

        serialized = Pal(
            "c410c416-475c-0638-eb35-269338f2a320", data, 0, 0
        ).to_dict()

        self.assertEqual(7, serialized["level"])
        self.assertEqual(1234, serialized["exp"])
        self.assertEqual(88, serialized["ranged"])
        self.assertEqual(77, serialized["defense"])
        self.assertEqual(4, serialized["rank"])

    def test_palworld_1_0_hp_talent_is_exposed_as_melee(self) -> None:
        data = self.pal_data("Hp")
        data["Talent_HP"] = {"value": {"value": 73}}

        serialized = Pal(
            "c410c416-475c-0638-eb35-269338f2a320", data, 0, 0
        ).to_dict()

        self.assertEqual(73, serialized["melee"])

    def test_pre_1_0_melee_talent_remains_supported(self) -> None:
        data = self.pal_data("HP")
        data["Talent_Melee"] = {"type": "IntProperty", "value": 41}

        serialized = Pal(
            "c410c416-475c-0638-eb35-269338f2a320", data, 0, 0
        ).to_dict()

        self.assertEqual(41, serialized["melee"])


if __name__ == "__main__":
    unittest.main()
