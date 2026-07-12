from __future__ import annotations

import copy
import json
import tempfile
import unittest
from pathlib import Path

from inventory_editor import (
    DYNAMIC_RAW_TYPE,
    SLOT_RAW_TYPE,
    InventoryConflictError,
    InventoryEditError,
    ItemDefinition,
    deliver_item,
    get_item_definition,
    load_item_catalog,
    resolve_player_save,
    set_item_quantity,
)


ZERO_UUID = "00000000-0000-0000-0000-000000000000"
CONTAINER_ID = "69f36429-4713-6224-a5f8-e89ba5f3fcb2"


def make_slot(
    index: int,
    item_id: str,
    count: int,
    dynamic_id: str = ZERO_UUID,
) -> dict:
    return {
        "RawData": {
            "array_type": "ByteProperty",
            "id": None,
            "value": {
                "slot_index": index,
                "count": count,
                "item": {
                    "static_id": item_id,
                    "dynamic_id": {
                        "created_world_id": ZERO_UUID,
                        "local_id_in_created_world": dynamic_id,
                    },
                },
                "trailing_bytes": [0] * 20,
            },
            "type": "ArrayProperty",
            "custom_type": SLOT_RAW_TYPE,
        },
        "CustomVersionData": {
            "array_type": "ByteProperty",
            "id": None,
            "value": {"values": [1, 2, 3, 4]},
            "type": "ArrayProperty",
        },
    }


def make_level(slot_count: int = 3) -> dict:
    return {
        "worldSaveData": {
            "value": {
                "ItemContainerSaveData": {
                    "value": [
                        {
                            "key": {"ID": {"value": CONTAINER_ID}},
                            "value": {
                                "SlotNum": {"value": slot_count},
                                "Slots": {
                                    "value": {
                                        "values": [make_slot(0, "Berries", 9)]
                                    }
                                },
                            },
                        }
                    ]
                },
                "DynamicItemSaveData": {
                    "array_type": "StructProperty",
                    "id": None,
                    "value": {
                        "prop_name": "DynamicItemSaveData",
                        "prop_type": "StructProperty",
                        "values": [],
                        "type_name": "PalDynamicItemSaveData",
                        "id": ZERO_UUID,
                    },
                    "type": "ArrayProperty",
                },
            }
        }
    }


def make_player() -> dict:
    container_value = {"value": {"ID": {"value": CONTAINER_ID}}}
    return {
        "SaveData": {
            "value": {
                "InventoryInfo": {
                    "value": {
                        "CommonContainerId": copy.deepcopy(container_value),
                        "EssentialContainerId": copy.deepcopy(container_value),
                        "WeaponLoadOutContainerId": copy.deepcopy(container_value),
                        "PlayerEquipArmorContainerId": copy.deepcopy(container_value),
                        "FoodEquipContainerId": copy.deepcopy(container_value),
                        "DropSlotContainerId": copy.deepcopy(container_value),
                    }
                }
            }
        }
    }


class InventoryEditorTest(unittest.TestCase):
    def test_stack_delivery_merges_and_splits(self) -> None:
        level = make_level()
        result = deliver_item(
            level,
            make_player(),
            "2119263560",
            ItemDefinition("Berries", max_stack=10),
            12,
        )

        slots = level["worldSaveData"]["value"]["ItemContainerSaveData"]["value"][
            0
        ]["value"]["Slots"]["value"]["values"]
        counts = [slot["RawData"]["value"]["count"] for slot in slots]
        self.assertEqual([10, 10, 1], counts)
        self.assertEqual((0, 1, 2), result.modified_slots)
        self.assertEqual(9, result.before)
        self.assertEqual(21, result.after)

    def test_space_failure_does_not_mutate_save(self) -> None:
        level = make_level()
        before = copy.deepcopy(level)

        with self.assertRaisesRegex(InventoryEditError, "empty slots"):
            deliver_item(
                level,
                make_player(),
                "2119263560",
                ItemDefinition("Berries", max_stack=10),
                22,
            )

        self.assertEqual(before, level)

    def test_dynamic_weapon_records_match_slot_references(self) -> None:
        level = make_level()
        result = deliver_item(
            level,
            make_player(),
            "2119263560",
            ItemDefinition(
                "AssaultRifle_Default1",
                max_stack=1,
                dynamic_type="weapon",
                durability=3000,
            ),
            2,
        )

        world = level["worldSaveData"]["value"]
        slots = world["ItemContainerSaveData"]["value"][0]["value"]["Slots"][
            "value"
        ]["values"]
        rifles = [
            slot
            for slot in slots
            if slot["RawData"]["value"]["item"]["static_id"]
            == "AssaultRifle_Default1"
        ]
        dynamics = world["DynamicItemSaveData"]["value"]["values"]
        slot_ids = {
            slot["RawData"]["value"]["item"]["dynamic_id"][
                "local_id_in_created_world"
            ]
            for slot in rifles
        }
        dynamic_ids = {
            item["RawData"]["value"]["id"]["local_id_in_created_world"]
            for item in dynamics
        }
        self.assertEqual(2, len(rifles))
        self.assertEqual(slot_ids, dynamic_ids)
        self.assertEqual((1, 2), result.modified_slots)
        self.assertEqual(slot_ids, set(result.dynamic_ids.values()))
        self.assertTrue(
            all(item["RawData"]["custom_type"] == DYNAMIC_RAW_TYPE for item in dynamics)
        )

    def test_existing_stack_quantity_can_be_updated_or_removed(self) -> None:
        catalog = {"berries": ItemDefinition("Berries", max_stack=10)}
        level = make_level()
        result = set_item_quantity(
            level,
            make_player(),
            "2119263560",
            catalog,
            "main",
            0,
            5,
            "berries",
            9,
        )
        slots = level["worldSaveData"]["value"]["ItemContainerSaveData"][
            "value"
        ][0]["value"]["Slots"]["value"]["values"]
        self.assertEqual(5, slots[0]["RawData"]["value"]["count"])
        self.assertFalse(result.removed)

        removed = set_item_quantity(
            level,
            make_player(),
            "2119263560",
            catalog,
            "main",
            0,
            0,
            "Berries",
            5,
        )
        self.assertEqual([], slots)
        self.assertTrue(removed.removed)

    def test_removing_dynamic_item_cleans_registry_record(self) -> None:
        dynamic_id = "e8af6046-c152-4c80-b69f-cb04e801be37"
        level = make_level()
        world = level["worldSaveData"]["value"]
        slots = world["ItemContainerSaveData"]["value"][0]["value"]["Slots"][
            "value"
        ]["values"]
        slots[0] = make_slot(0, "AssaultRifle_Default1", 1, dynamic_id)
        world["DynamicItemSaveData"]["value"]["values"].append(
            {
                "RawData": {
                    "value": {
                        "id": {
                            "local_id_in_created_world": dynamic_id,
                            "static_id": "AssaultRifle_Default1",
                        },
                        "type": "weapon",
                    }
                }
            }
        )

        result = set_item_quantity(
            level,
            make_player(),
            "2119263560",
            {
                "assaultrifle_default1": ItemDefinition(
                    "AssaultRifle_Default1",
                    max_stack=1,
                    dynamic_type="weapon",
                )
            },
            "weapons",
            0,
            0,
            "AssaultRifle_Default1",
            1,
            dynamic_id,
        )

        self.assertEqual([], slots)
        self.assertEqual([], world["DynamicItemSaveData"]["value"]["values"])
        self.assertTrue(result.dynamic_record_removed)

    def test_dynamic_item_edit_rejects_replaced_instance(self) -> None:
        current_dynamic_id = "e8af6046-c152-4c80-b69f-cb04e801be37"
        stale_dynamic_id = "f02aeaf1-1589-4e12-ab53-a67ea1972482"
        level = make_level()
        world = level["worldSaveData"]["value"]
        slots = world["ItemContainerSaveData"]["value"][0]["value"]["Slots"][
            "value"
        ]["values"]
        slots[0] = make_slot(
            0, "AssaultRifle_Default1", 1, current_dynamic_id
        )
        world["DynamicItemSaveData"]["value"]["values"].append(
            {
                "RawData": {
                    "value": {
                        "id": {
                            "local_id_in_created_world": current_dynamic_id,
                            "static_id": "AssaultRifle_Default1",
                        },
                        "type": "weapon",
                    }
                }
            }
        )
        before = copy.deepcopy(level)

        with self.assertRaisesRegex(InventoryConflictError, "dynamic item changed"):
            set_item_quantity(
                level,
                make_player(),
                "2119263560",
                {},
                "weapons",
                0,
                0,
                "AssaultRifle_Default1",
                1,
                stale_dynamic_id,
            )

        self.assertEqual(before, level)

    def test_delivery_rejects_incompatible_explicit_container(self) -> None:
        with self.assertRaisesRegex(InventoryEditError, "Non-essential"):
            deliver_item(
                make_level(),
                make_player(),
                "2119263560",
                ItemDefinition("Wood", max_stack=9999),
                1,
                "key",
            )
        with self.assertRaisesRegex(InventoryEditError, "must be delivered"):
            deliver_item(
                make_level(),
                make_player(),
                "2119263560",
                ItemDefinition(
                    "TechnologyBook_G1",
                    max_stack=9999,
                    type_a="EPalItemTypeA::Essential",
                ),
                1,
                "main",
            )

    def test_quantity_edit_rejects_stale_or_oversized_values(self) -> None:
        catalog = {"berries": ItemDefinition("Berries", max_stack=10)}
        with self.assertRaisesRegex(InventoryConflictError, "changed from 8 to 9"):
            set_item_quantity(
                make_level(),
                make_player(),
                "2119263560",
                catalog,
                "main",
                0,
                5,
                "Berries",
                8,
            )
        with self.assertRaisesRegex(InventoryEditError, "maximum stack"):
            set_item_quantity(
                make_level(),
                make_player(),
                "2119263560",
                catalog,
                "main",
                0,
                11,
                "Berries",
                9,
            )

    def test_catalog_rejects_pal_eggs(self) -> None:
        catalog = {
            "palegg_dark_01": ItemDefinition(
                "PalEgg_Dark_01",
                max_stack=1,
                type_b="EPalItemTypeB::MaterialPalEgg",
            )
        }
        with self.assertRaisesRegex(InventoryEditError, "Pal-specific"):
            get_item_definition(catalog, "palegg_dark_01")

    def test_metadata_lookup_is_case_insensitive(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "items.json"
            path.write_text(
                json.dumps(
                    {
                        "items": {
                            "wood": {
                                "id": "Wood",
                                "max_stack": 9999,
                                "type_a": "EPalItemTypeA::Material",
                            }
                        }
                    }
                ),
                encoding="utf-8",
            )
            catalog = load_item_catalog(str(path))

        definition = get_item_definition(catalog, "WOOD")
        self.assertEqual("Wood", definition.item_id)
        self.assertEqual(9999, definition.max_stack)

    def test_decimal_uid_resolves_cross_platform_filename_prefix(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            players_dir = save_dir / "Players"
            players_dir.mkdir()
            level_path = save_dir / "Level.sav"
            level_path.touch()
            player_path = players_dir / "7E516548AABBCCDDEEFF001122334455.sav"
            player_path.touch()

            resolved = resolve_player_save(level_path, "2119263560")

        self.assertEqual(player_path.name, resolved.name)

    def test_all_digit_guid_resolves_as_guid(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            players_dir = save_dir / "Players"
            players_dir.mkdir()
            level_path = save_dir / "Level.sav"
            level_path.touch()
            player_path = players_dir / "00000000000000000000000000000001.sav"
            player_path.touch()

            resolved = resolve_player_save(
                level_path,
                "00000000-0000-0000-0000-000000000001",
            )

        self.assertEqual(player_path.name, resolved.name)


if __name__ == "__main__":
    unittest.main()
