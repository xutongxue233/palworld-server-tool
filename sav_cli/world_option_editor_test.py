from __future__ import annotations

import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

from world_option_editor import (
    WORLD_OPTION_TEMPLATE_SHA256,
    WorldOptionEditError,
    default_world_option_template,
    load_world_option_metadata,
    parse_palworld_settings,
    sync_world_option,
    verify_world_option_sync,
)


def world_option_fixture() -> dict:
    return {
        "properties": {
            "Timestamp": {
                "struct_type": "DateTime",
                "value": 1,
                "type": "StructProperty",
            },
            "OptionWorldData": {
                "struct_type": "PalOptionWorldSaveData",
                "value": {
                    "Settings": {
                        "struct_type": "PalOptionWorldSettings",
                        "value": {
                            "FutureSetting": {
                                "id": None,
                                "value": 7,
                                "type": "IntProperty",
                            }
                        },
                        "type": "StructProperty",
                    }
                },
                "type": "StructProperty",
            },
        }
    }


class WorldOptionEditorTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.metadata = load_world_option_metadata(
            Path(__file__).with_name("world_option_metadata.json")
        )

    def test_bundled_template_checksum_is_pinned(self) -> None:
        template = default_world_option_template()
        self.assertEqual(1539, len(template))
        self.assertEqual(
            WORLD_OPTION_TEMPLATE_SHA256,
            hashlib.sha256(template).hexdigest(),
        )

    def test_metadata_checksum_is_semantic_and_enforced(self) -> None:
        source = Path(__file__).with_name("world_option_metadata.json")
        with tempfile.TemporaryDirectory() as temporary_directory:
            normalized = source.read_bytes().replace(b"\r\n", b"\n")
            crlf = Path(temporary_directory) / "crlf.json"
            crlf.write_bytes(normalized.replace(b"\n", b"\r\n"))
            self.assertEqual(self.metadata, load_world_option_metadata(crlf))

            payload = json.loads(normalized.decode("utf-8"))
            payload["game_version"] = "9.9.9"
            tampered = Path(temporary_directory) / "tampered.json"
            tampered.write_text(json.dumps(payload), encoding="utf-8")
            with self.assertRaisesRegex(WorldOptionEditError, "checksum"):
                load_world_option_metadata(tampered)

            invalid = Path(temporary_directory) / "invalid.json"
            invalid.write_bytes(b"\xff")
            with self.assertRaisesRegex(WorldOptionEditError, "Unable to load"):
                load_world_option_metadata(invalid)

    def test_parser_handles_strings_arrays_booleans_and_numbers(self) -> None:
        settings = parse_palworld_settings(
            "[/Script/Pal.PalGameWorldSettings]\n"
            "OptionSettings=(ServerName=\"PST, World\","
            "ServerDescription=\"quote: \\\"ok\\\"\","
            "CrossplayPlatforms=(Steam,Xbox,PS5,Mac),"
            "bIsUseBackupSaveData=True,AutoSaveSpan=30.000000,"
            "PublicPort=8211,Difficulty=None)\n"
        )
        self.assertEqual("PST, World", settings["ServerName"])
        self.assertEqual('quote: "ok"', settings["ServerDescription"])
        self.assertEqual(
            ["Steam", "Xbox", "PS5", "Mac"],
            settings["CrossplayPlatforms"],
        )
        self.assertIs(True, settings["bIsUseBackupSaveData"])
        self.assertEqual(30.0, settings["AutoSaveSpan"])
        self.assertEqual(8211, settings["PublicPort"])
        self.assertEqual("None", settings["Difficulty"])

    def test_sync_writes_typed_properties_and_preserves_unknowns(self) -> None:
        gvas = world_option_fixture()
        before_timestamp = gvas["properties"]["Timestamp"]["value"]
        result = sync_world_option(
            gvas,
            "[/Script/Pal.PalGameWorldSettings]\n"
            "OptionSettings=(ServerName=\"PST\",PublicPort=8211,"
            "AutoSaveSpan=45.000000,bIsUseBackupSaveData=False,"
            "RandomizerType=Region,CrossplayPlatforms=(Xbox,PS5),"
            "bEnableVoiceChat=True)\n",
            self.metadata,
        )
        settings = gvas["properties"]["OptionWorldData"]["value"]["Settings"][
            "value"
        ]
        self.assertEqual("StrProperty", settings["ServerName"]["type"])
        self.assertEqual("IntProperty", settings["PublicPort"]["type"])
        self.assertEqual("FloatProperty", settings["AutoSaveSpan"]["type"])
        self.assertEqual("BoolProperty", settings["bIsUseBackupSaveData"]["type"])
        self.assertEqual(
            "EPalRandomizerType::Region",
            settings["RandomizerType"]["value"]["value"],
        )
        self.assertEqual(
            ["EPalAllowConnectPlatform::Xbox", "EPalAllowConnectPlatform::PS5"],
            settings["CrossplayPlatforms"]["value"]["values"],
        )
        self.assertEqual(7, settings["FutureSetting"]["value"])
        self.assertIn("bEnableVoiceChat", result.skipped_keys)
        self.assertGreater(
            gvas["properties"]["Timestamp"]["value"], before_timestamp
        )
        verify_world_option_sync(gvas, result)

        tampered = copy.deepcopy(gvas)
        tampered["properties"]["OptionWorldData"]["value"]["Settings"]["value"][
            "FutureSetting"
        ]["value"] = 8
        with self.assertRaisesRegex(WorldOptionEditError, "outside"):
            verify_world_option_sync(tampered, result)

    def test_parser_rejects_duplicate_and_incomplete_entries(self) -> None:
        for content in (
            "[/Script/Pal.PalGameWorldSettings]\nOptionSettings=(A=1,A=2)",
            "[/Script/Pal.PalGameWorldSettings]\nOptionSettings=(A=\"broken)",
        ):
            with self.assertRaises(WorldOptionEditError):
                parse_palworld_settings(content)

    def test_sync_rejects_unknown_enum_values(self) -> None:
        for setting in (
            "DeathPenalty=Everything",
            "CrossplayPlatforms=(Steam,UnknownPlatform)",
        ):
            with self.subTest(setting=setting):
                with self.assertRaises(WorldOptionEditError):
                    sync_world_option(
                        world_option_fixture(),
                        "[/Script/Pal.PalGameWorldSettings]\n"
                        f"OptionSettings=({setting})\n",
                        self.metadata,
                    )


if __name__ == "__main__":
    unittest.main()
