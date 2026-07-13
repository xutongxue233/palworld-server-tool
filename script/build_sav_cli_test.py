import json
import tempfile
import unittest
from pathlib import Path

from sav_cli.inventory_editor import (
    InventoryEditError,
    get_item_definition,
    load_item_catalog,
)
from script import build_sav_cli


class ItemCatalogGenerationTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        game_data = self.root / "resources" / "game_data"
        game_data.mkdir(parents=True)
        source_dir = self.root / "src"
        source_dir.mkdir()
        (source_dir / "common.py").write_text(
            "GAME_VERSION = '1.0.0'\n", encoding="utf-8"
        )
        (game_data / "items.json").write_text(
            json.dumps(
                {
                    "items": [
                        {
                            "asset": "Wood",
                            "max_stack": 9999,
                            "type_a": "EPalItemTypeA::Material",
                            "type_b": "EPalItemTypeB::MaterialWood",
                        },
                        {
                            "asset": "PalEgg_Dark_01",
                            "max_stack": 1,
                            "type_a": "EPalItemTypeA::Material",
                            "type_b": build_sav_cli.PAL_EGG_TYPE_B,
                        },
                        {
                            "asset": "BossDefeatReward_Test",
                            "max_stack": 9999,
                            "type_a": "EPalItemTypeA::Essential",
                            "type_b": "EPalItemTypeB::Essential",
                        },
                        {
                            "asset": "AssaultRifle_Default1",
                            "max_stack": 1,
                            "type_a": "EPalItemTypeA::Weapon",
                            "type_b": "EPalItemTypeB::WeaponAssaultRifle",
                        },
                    ],
                    "items_dynamic": {
                        "AssaultRifle_Default1": {
                            "dynamic": {"type": "weapon", "durability": 3000}
                        }
                    },
                },
                separators=(",", ":"),
            ),
            encoding="utf-8",
        )
        (game_data / "characters.json").write_text(
            json.dumps(
                {
                    "pals": [
                        {
                            "asset": "SheepBall",
                            "scaling": {"hp": 70},
                            "stats": {"hp": 71},
                            "friendship_hp": 5.5,
                        },
                        {
                            "asset": "BOSS_SheepBall",
                            "scaling": {"hp": 84},
                            "friendship_hp": 4.8,
                        },
                        {
                            "asset": "LegacyEnemy",
                            "stats": {"hp": 90},
                        },
                        {"asset": "Human"},
                    ]
                },
                separators=(",", ":"),
            ),
            encoding="utf-8",
        )
        (game_data / "friendship.json").write_text(
            json.dumps(
                {
                    **{
                        f"Friendship_Rank_{rank}": {
                            "FriendshipRank": rank,
                            "RequiredPoint": rank * rank * 1000,
                        }
                        for rank in range(11)
                    },
                    "Friendship_Rank_Minus1": {
                        "FriendshipRank": -1,
                        "RequiredPoint": -1,
                    },
                },
                separators=(",", ":"),
            ),
            encoding="utf-8",
        )
        self.fast_travel_guids = [
            f"{index:032X}"
            for index in range(1, build_sav_cli.PLAYER_MAP_FAST_TRAVEL_COUNT + 1)
        ]
        (game_data / "fast_travel_points.json").write_text(
            json.dumps(
                {
                    guid: {"id": f"FastTravel_{index}"}
                    for index, guid in enumerate(self.fast_travel_guids)
                },
                separators=(",", ":"),
            ),
            encoding="utf-8",
        )
        self.world_map_areas = [
            f"Area_{index:03d}"
            for index in range(build_sav_cli.PLAYER_MAP_AREA_COUNT)
        ]
        (game_data / "world_map_areas.json").write_text(
            json.dumps(
                {"areas": self.world_map_areas},
                separators=(",", ":"),
            ),
            encoding="utf-8",
        )

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def test_web_catalog_matches_backend_delivery_rules(self) -> None:
        staging = self.root / "staging"
        staging.mkdir()
        backend_path = build_sav_cli.build_item_metadata(self.root, staging)
        web_path = build_sav_cli.build_web_item_catalog(
            self.root, self.root / "deliverable-items.json"
        )

        backend_catalog = load_item_catalog(str(backend_path))
        backend_deliverable = set()
        for key, definition in backend_catalog.items():
            try:
                get_item_definition(backend_catalog, definition.item_id)
            except InventoryEditError:
                continue
            backend_deliverable.add(key)

        web_item_ids = json.loads(web_path.read_text(encoding="utf-8"))["item_ids"]
        self.assertEqual(
            {item_id.lower() for item_id in web_item_ids}, backend_deliverable
        )
        self.assertEqual(set(web_item_ids), {"Wood", "AssaultRifle_Default1"})

    def test_world_option_metadata_is_pinned_to_palworld_1_0_0(self) -> None:
        source = (
            Path(__file__).resolve().parent.parent
            / "sav_cli"
            / "world_option_metadata.json"
        )
        repo_root = self.root / "metadata-repo"
        metadata_dir = repo_root / "sav_cli"
        metadata_dir.mkdir(parents=True)
        normalized = source.read_bytes().replace(b"\r\n", b"\n")
        (metadata_dir / source.name).write_bytes(
            normalized.replace(b"\n", b"\r\n")
        )
        staging = self.root / "world-option-staging"
        staging.mkdir()
        destination = build_sav_cli.copy_world_option_metadata(repo_root, staging)
        payload = json.loads(destination.read_text(encoding="utf-8"))
        self.assertEqual("1.0.0", payload["game_version"])
        self.assertEqual(build_sav_cli.PAL_CONF_COMMIT, payload["source_commit"])
        self.assertEqual(
            build_sav_cli.WORLD_OPTION_METADATA_ENTRIES,
            len(payload["settings"]),
        )

        (metadata_dir / source.name).write_bytes(b"\xff")
        with self.assertRaisesRegex(RuntimeError, "Unable to read"):
            build_sav_cli.copy_world_option_metadata(repo_root, staging)

    def test_check_rejects_a_stale_generated_catalog(self) -> None:
        destination = build_sav_cli.build_web_item_catalog(
            self.root, self.root / "deliverable-items.json"
        )
        build_sav_cli.check_web_item_catalog(self.root, destination)

        destination.write_text("{}\n", encoding="utf-8")
        with self.assertRaisesRegex(RuntimeError, "stale"):
            build_sav_cli.check_web_item_catalog(self.root, destination)

    def test_pal_level_metadata_matches_1_0_formula_inputs(self) -> None:
        staging = self.root / "staging"
        staging.mkdir()

        destination = build_sav_cli.build_pal_level_metadata(self.root, staging)
        payload = json.loads(destination.read_text(encoding="utf-8"))

        self.assertEqual(1, payload["schema"])
        self.assertEqual("1.0.0", payload["game_version"])
        self.assertEqual(80, payload["max_level"])
        self.assertEqual([rank * rank * 1000 for rank in range(11)], payload["friendship_thresholds"])
        self.assertEqual(
            {"hp_scaling": 70.0, "friendship_hp": 5.5},
            payload["pals"]["sheepball"],
        )
        self.assertEqual(84.0, payload["pals"]["boss_sheepball"]["hp_scaling"])
        self.assertEqual(
            {"hp_scaling": 90.0, "friendship_hp": 0.0},
            payload["pals"]["legacyenemy"],
        )
        self.assertNotIn("human", payload["pals"])

    def test_pal_level_metadata_rejects_wrong_version_and_missing_rank(self) -> None:
        common_path = self.root / "src" / "common.py"
        common_path.write_text("GAME_VERSION = '0.6.8'\n", encoding="utf-8")
        with self.assertRaisesRegex(RuntimeError, "expected 1.0.0"):
            build_sav_cli.pal_level_metadata_payload(self.root)

        common_path.write_text("GAME_VERSION = '1.0.0'\n", encoding="utf-8")
        friendship_path = self.root / "resources" / "game_data" / "friendship.json"
        friendship = json.loads(friendship_path.read_text(encoding="utf-8"))
        del friendship["Friendship_Rank_10"]
        friendship_path.write_text(json.dumps(friendship), encoding="utf-8")
        with self.assertRaisesRegex(RuntimeError, "ranks 0 through 10"):
            build_sav_cli.pal_level_metadata_payload(self.root)

    def test_player_map_metadata_matches_fixed_1_0_0_sources(self) -> None:
        staging = self.root / "staging"
        staging.mkdir()

        destination = build_sav_cli.build_player_map_metadata(self.root, staging)
        payload = json.loads(destination.read_text(encoding="utf-8"))

        self.assertEqual(1, payload["schema"])
        self.assertEqual(build_sav_cli.PST_COMMIT, payload["source_commit"])
        self.assertEqual("1.0.0", payload["game_version"])
        self.assertEqual(
            build_sav_cli.PLAYER_MAP_FAST_TRAVEL_COUNT,
            len(payload["fast_travel_guids"]),
        )
        self.assertEqual(
            build_sav_cli.PLAYER_MAP_AREA_COUNT,
            len(payload["areas"]),
        )
        self.assertEqual(
            sorted(self.fast_travel_guids), payload["fast_travel_guids"]
        )
        self.assertEqual(sorted(self.world_map_areas), payload["areas"])
        self.assertEqual(["MainMap", "Tree"], payload["world_flags"])

    def test_player_map_metadata_rejects_wrong_version_counts_and_duplicates(self) -> None:
        common_path = self.root / "src" / "common.py"
        common_path.write_text("GAME_VERSION = '0.6.8'\n", encoding="utf-8")
        with self.assertRaisesRegex(RuntimeError, "expected 1.0.0"):
            build_sav_cli.player_map_metadata_payload(self.root)
        common_path.write_text("GAME_VERSION = '1.0.0'\n", encoding="utf-8")

        fast_travel_path = (
            self.root / "resources" / "game_data" / "fast_travel_points.json"
        )
        fast_travel = json.loads(fast_travel_path.read_text(encoding="utf-8"))
        invalid_guid_value = fast_travel.pop(self.fast_travel_guids[0])
        fast_travel["not-a-guid"] = invalid_guid_value
        fast_travel_path.write_text(json.dumps(fast_travel), encoding="utf-8")
        with self.assertRaisesRegex(RuntimeError, "invalid GUID"):
            build_sav_cli.player_map_metadata_payload(self.root)

        fast_travel_path.write_text(
            json.dumps(
                {
                    guid: {"id": f"FastTravel_{index}"}
                    for index, guid in enumerate(self.fast_travel_guids[:-1])
                }
            ),
            encoding="utf-8",
        )
        with self.assertRaisesRegex(RuntimeError, "exactly 174"):
            build_sav_cli.player_map_metadata_payload(self.root)

        duplicate_fast_travel = {
            guid: {} for guid in self.fast_travel_guids
        }
        duplicate_fast_travel[self.fast_travel_guids[-1].lower()] = {}
        fast_travel_path.write_text(
            json.dumps(duplicate_fast_travel), encoding="utf-8"
        )
        with self.assertRaisesRegex(RuntimeError, "duplicates GUID"):
            build_sav_cli.player_map_metadata_payload(self.root)

        fast_travel_path.write_text(
            json.dumps({guid: {} for guid in self.fast_travel_guids}),
            encoding="utf-8",
        )
        areas_path = self.root / "resources" / "game_data" / "world_map_areas.json"
        duplicate_areas = list(self.world_map_areas)
        duplicate_areas[-1] = duplicate_areas[0]
        areas_path.write_text(
            json.dumps({"areas": duplicate_areas}), encoding="utf-8"
        )
        with self.assertRaisesRegex(RuntimeError, "duplicates area"):
            build_sav_cli.player_map_metadata_payload(self.root)


if __name__ == "__main__":
    unittest.main()
