from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import sav_cli


class FakeGvas:
    def __init__(self, written: bytes) -> None:
        self.written = written
        self.write_calls = 0

    def write(self, *, custom_properties: object) -> bytes:
        self.write_calls += 1
        self.custom_properties = custom_properties
        return self.written


class EditPreflightTest(unittest.TestCase):
    def test_load_level_for_edit_keeps_validated_gvas_instance(self) -> None:
        gvas = FakeGvas(b"unchanged")

        with patch.object(
            sav_cli,
            "load_gvas",
            return_value=(b"unchanged", 0x32, gvas),
        ):
            save_type, loaded = sav_cli.load_level_for_edit(Path("Level.sav"))

        self.assertEqual(save_type, 0x32)
        self.assertIs(loaded, gvas)
        self.assertEqual(gvas.write_calls, 1)
        self.assertIs(gvas.custom_properties, sav_cli.PALWORLD_CUSTOM_PROPERTIES)

    def test_load_level_for_edit_rejects_lossy_decode(self) -> None:
        gvas = FakeGvas(b"changed")

        with patch.object(
            sav_cli,
            "load_gvas",
            return_value=(b"original", 0x32, gvas),
        ):
            with self.assertRaisesRegex(ValueError, "edit preflight"):
                sav_cli.load_level_for_edit(Path("Level.sav"))

    def test_give_item_preflight_failure_does_not_mutate_or_write(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            output_path = save_dir / "edited" / "Level.sav"
            mutation = Mock()
            resolve_player = Mock()

            with (
                patch.object(
                    sav_cli,
                    "load_gvas",
                    return_value=(b"original", 0x32, FakeGvas(b"changed")),
                ),
                patch.object(sav_cli, "deliver_item", mutation),
                patch.object(sav_cli, "resolve_player_save", resolve_player),
            ):
                with self.assertRaisesRegex(ValueError, "edit preflight"):
                    sav_cli.give_item(
                        save_dir / "Level.sav",
                        str(output_path),
                        "2119263560",
                        "Wood",
                        1,
                        "auto",
                        "",
                    )

            mutation.assert_not_called()
            resolve_player.assert_not_called()
            self.assertFalse(output_path.exists())
            self.assertFalse(output_path.parent.exists())
    def test_set_quantity_preflight_failure_does_not_mutate_or_write(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            output_path = save_dir / "edited" / "Level.sav"
            mutation = Mock()
            resolve_player = Mock()

            with (
                patch.object(
                    sav_cli,
                    "load_gvas",
                    return_value=(b"original", 0x32, FakeGvas(b"changed")),
                ),
                patch.object(sav_cli, "set_item_quantity", mutation),
                patch.object(sav_cli, "resolve_player_save", resolve_player),
            ):
                with self.assertRaisesRegex(ValueError, "edit preflight"):
                    sav_cli.set_inventory_quantity(
                        save_dir / "Level.sav",
                        str(output_path),
                        "2119263560",
                        "main",
                        0,
                        0,
                        "Wood",
                        1,
                        "",
                        "00000000-0000-0000-0000-000000000000",
                    )

            mutation.assert_not_called()
            resolve_player.assert_not_called()
            self.assertFalse(output_path.exists())
            self.assertFalse(output_path.parent.exists())

    def test_profile_preflight_failure_does_not_mutate_or_write(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            output_path = save_dir / "edited" / "Level.sav"
            mutation = Mock()
            load_table = Mock()

            with (
                patch.object(
                    sav_cli,
                    "load_gvas",
                    return_value=(b"original", 0x32, FakeGvas(b"changed")),
                ),
                patch.object(sav_cli, "set_player_profile", mutation),
                patch.object(sav_cli, "load_exp_table", load_table),
            ):
                with self.assertRaisesRegex(ValueError, "edit preflight"):
                    sav_cli.edit_player_profile(
                        save_dir / "Level.sav",
                        str(output_path),
                        "2119263560",
                        "Old name",
                        2,
                        "New name",
                        3,
                        "exp.json",
                    )

            mutation.assert_not_called()
            load_table.assert_not_called()
            self.assertFalse(output_path.exists())
            self.assertFalse(output_path.parent.exists())

    def test_stat_points_preflight_failure_does_not_mutate_or_write(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            output_path = save_dir / "edited" / "Level.sav"
            mutation = Mock()

            with (
                patch.object(
                    sav_cli,
                    "load_gvas",
                    return_value=(b"original", 0x32, FakeGvas(b"changed")),
                ),
                patch.object(sav_cli, "set_player_stat_points", mutation),
            ):
                with self.assertRaisesRegex(ValueError, "edit preflight"):
                    sav_cli.edit_player_stat_points(
                        save_dir / "Level.sav",
                        str(output_path),
                        "2119263560",
                        1,
                        2,
                    )

            mutation.assert_not_called()
            self.assertFalse(output_path.exists())
            self.assertFalse(output_path.parent.exists())

    def test_stat_points_cli_arguments(self) -> None:
        with patch.object(
            sav_cli.sys,
            "argv",
            [
                "sav_cli.py",
                "--mode",
                "edit-player-stat-points",
                "--unused-stat-points",
                "7",
                "--expected-unused-stat-points",
                "1",
            ],
        ):
            args = sav_cli.parse_args()

        self.assertEqual("edit-player-stat-points", args.mode)
        self.assertEqual(7, args.unused_stat_points)
        self.assertEqual(1, args.expected_unused_stat_points)

    def test_technology_points_cli_arguments(self) -> None:
        with patch.object(
            sav_cli.sys,
            "argv",
            [
                "sav_cli.py",
                "--mode",
                "edit-player-technology-points",
                "--technology-points",
                "9",
                "--expected-technology-points",
                "1",
                "--ancient-technology-points",
                "4",
                "--expected-ancient-technology-points",
                "2",
            ],
        ):
            args = sav_cli.parse_args()

        self.assertEqual("edit-player-technology-points", args.mode)
        self.assertEqual(9, args.technology_points)
        self.assertEqual(1, args.expected_technology_points)
        self.assertEqual(4, args.ancient_technology_points)
        self.assertEqual(2, args.expected_ancient_technology_points)

    def test_technology_points_preflight_failure_does_not_mutate_or_write(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            output_path = save_dir / "edited" / "Player.sav"
            mutation = Mock()

            with (
                patch.object(
                    sav_cli,
                    "load_gvas",
                    return_value=(b"original", 0x32, FakeGvas(b"changed")),
                ),
                patch.object(sav_cli, "set_player_technology_points", mutation),
            ):
                with self.assertRaisesRegex(ValueError, "Player GVAS"):
                    sav_cli.edit_player_technology_points(
                        save_dir / "Player.sav",
                        str(output_path),
                        "2119263560",
                        0,
                        0,
                        1,
                        2,
                    )

            mutation.assert_not_called()
            self.assertFalse(output_path.exists())
            self.assertFalse(output_path.parent.exists())

    def test_unlock_player_map_cli_arguments_preserve_digest_and_metadata(self) -> None:
        digest = "a" * 64
        with patch.object(
            sav_cli.sys,
            "argv",
            [
                "sav_cli.py",
                "--mode",
                "unlock-player-map",
                "--expected-map-progress-digest",
                digest,
                "--map-metadata",
                "custom-map.json",
            ],
        ):
            args = sav_cli.parse_args()

        self.assertEqual("unlock-player-map", args.mode)
        self.assertEqual(digest, args.expected_map_progress_digest)
        self.assertEqual("custom-map.json", args.map_metadata)

    def test_unlock_player_map_preflight_failure_does_not_load_metadata_or_write(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            output_path = save_dir / "edited" / "Player.sav"
            metadata = Mock()
            mutation = Mock()

            with (
                patch.object(
                    sav_cli,
                    "load_gvas",
                    return_value=(b"original", 0x32, FakeGvas(b"changed")),
                ),
                patch.object(sav_cli, "load_player_map_metadata", metadata),
                patch.object(sav_cli, "unlock_player_map", mutation),
            ):
                with self.assertRaisesRegex(ValueError, "map edit preflight"):
                    sav_cli.unlock_player_map_save(
                        save_dir / "Player.sav",
                        str(output_path),
                        "2119263560",
                        "a" * 64,
                        "custom-map.json",
                    )

            metadata.assert_not_called()
            mutation.assert_not_called()
            self.assertFalse(output_path.exists())
            self.assertFalse(output_path.parent.exists())

    def test_pal_nickname_cli_arguments_preserve_explicit_empty_values(self) -> None:
        with patch.object(
            sav_cli.sys,
            "argv",
            [
                "sav_cli.py",
                "--mode",
                "edit-pal-nickname",
                "--instance-id",
                "c410c416-475c-0638-eb35-269338f2a320",
                "--expected-pal-nickname",
                "Old Pal",
                "--pal-nickname",
                "",
                "--expected-pal-level",
                "2",
                "--expected-pal-exp",
                "25",
            ],
        ):
            args = sav_cli.parse_args()

        self.assertEqual("edit-pal-nickname", args.mode)
        self.assertEqual("c410c416-475c-0638-eb35-269338f2a320", args.instance_id)
        self.assertEqual("Old Pal", args.expected_pal_nickname)
        self.assertEqual("", args.pal_nickname)
        self.assertEqual(2, args.expected_pal_level)
        self.assertEqual(25, args.expected_pal_exp)

    def test_pal_nickname_preflight_failure_does_not_mutate_or_write(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            output_path = save_dir / "edited" / "Level.sav"
            mutation = Mock()

            with (
                patch.object(
                    sav_cli,
                    "load_gvas",
                    return_value=(b"original", 0x32, FakeGvas(b"changed")),
                ),
                patch.object(sav_cli, "rename_pal", mutation),
            ):
                with self.assertRaisesRegex(ValueError, "edit preflight"):
                    sav_cli.edit_pal_nickname(
                        save_dir / "Level.sav",
                        str(output_path),
                        "2119263560",
                        "c410c416-475c-0638-eb35-269338f2a320",
                        "Old Pal",
                        2,
                        25,
                        "New Pal",
                    )

            mutation.assert_not_called()
            self.assertFalse(output_path.exists())
            self.assertFalse(output_path.parent.exists())

    def test_pal_level_cli_arguments_preserve_cas_values(self) -> None:
        with patch.object(
            sav_cli.sys,
            "argv",
            [
                "sav_cli.py",
                "--mode",
                "edit-pal-level",
                "--instance-id",
                "ff38bdad-4710-966d-982f-3ca7cb107b56",
                "--expected-pal-nickname",
                "",
                "--expected-pal-level",
                "55",
                "--expected-pal-exp",
                "6678888",
                "--expected-pal-hp",
                "7286000",
                "--expected-pal-max-hp",
                "0",
                "--pal-level",
                "56",
                "--pal-level-metadata",
                "custom.json",
            ],
        ):
            args = sav_cli.parse_args()

        self.assertEqual("edit-pal-level", args.mode)
        self.assertEqual("", args.expected_pal_nickname)
        self.assertEqual(55, args.expected_pal_level)
        self.assertEqual(6678888, args.expected_pal_exp)
        self.assertEqual(7286000, args.expected_pal_hp)
        self.assertEqual(0, args.expected_pal_max_hp)
        self.assertEqual(56, args.pal_level)
        self.assertEqual("custom.json", args.pal_level_metadata)

    def test_pal_level_preflight_failure_does_not_load_metadata_mutate_or_write(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            output_path = save_dir / "edited" / "Level.sav"
            mutation = Mock()
            metadata = Mock()

            with (
                patch.object(
                    sav_cli,
                    "load_gvas",
                    return_value=(b"original", 0x32, FakeGvas(b"changed")),
                ),
                patch.object(sav_cli, "set_pal_level", mutation),
                patch.object(sav_cli, "load_pal_level_metadata", metadata),
            ):
                with self.assertRaisesRegex(ValueError, "edit preflight"):
                    sav_cli.edit_pal_level(
                        save_dir / "Level.sav",
                        str(output_path),
                        "2119263560",
                        "ff38bdad-4710-966d-982f-3ca7cb107b56",
                        "Raid Jet",
                        55,
                        6678888,
                        7286000,
                        0,
                        56,
                        "",
                        "",
                    )

            mutation.assert_not_called()
            metadata.assert_not_called()
            self.assertFalse(output_path.exists())
            self.assertFalse(output_path.parent.exists())

    def test_restore_pal_health_cli_arguments_preserve_explicit_empty_nickname(self) -> None:
        with patch.object(
            sav_cli.sys,
            "argv",
            [
                "sav_cli.py",
                "--mode",
                "restore-pal-health",
                "--instance-id",
                "c410c416-475c-0638-eb35-269338f2a320",
                "--expected-pal-nickname",
                "",
                "--expected-pal-level",
                "2",
                "--expected-pal-exp",
                "25",
                "--expected-pal-hp",
                "136741",
                "--expected-pal-max-hp",
                "583000",
            ],
        ):
            args = sav_cli.parse_args()

        self.assertEqual("restore-pal-health", args.mode)
        self.assertEqual("", args.expected_pal_nickname)
        self.assertEqual(136741, args.expected_pal_hp)
        self.assertEqual(583000, args.expected_pal_max_hp)

    def test_restore_pal_health_preflight_failure_does_not_mutate_or_write(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            output_path = save_dir / "edited" / "Level.sav"
            mutation = Mock()

            with (
                patch.object(
                    sav_cli,
                    "load_gvas",
                    return_value=(b"original", 0x32, FakeGvas(b"changed")),
                ),
                patch.object(sav_cli, "restore_pal_health", mutation),
            ):
                with self.assertRaisesRegex(ValueError, "edit preflight"):
                    sav_cli.restore_pal_health_save(
                        save_dir / "Level.sav",
                        str(output_path),
                        "2119263560",
                        "c410c416-475c-0638-eb35-269338f2a320",
                        "",
                        2,
                        25,
                        136741,
                        583000,
                    )

            mutation.assert_not_called()
            self.assertFalse(output_path.exists())
            self.assertFalse(output_path.parent.exists())

    def test_restore_pal_health_validation_failure_preserves_existing_output(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            save_dir = Path(temp_dir)
            output_path = save_dir / "edited" / "Level.sav"
            output_path.parent.mkdir(parents=True)
            output_path.write_bytes(b"existing-output")
            result = Mock()

            with (
                patch.object(
                    sav_cli,
                    "load_level_for_pal_edit",
                    return_value=(0x32, FakeGvas(b"edited-gvas")),
                ),
                patch.object(sav_cli, "restore_pal_health", return_value=result),
                patch.object(
                    sav_cli,
                    "compress_gvas_to_sav",
                    return_value=b"rebuilt-save",
                ),
                patch.object(
                    sav_cli,
                    "load_gvas",
                    side_effect=ValueError("post-write validation failed"),
                ),
            ):
                with self.assertRaisesRegex(ValueError, "post-write validation"):
                    sav_cli.restore_pal_health_save(
                        save_dir / "Level.sav",
                        str(output_path),
                        "2119263560",
                        "c410c416-475c-0638-eb35-269338f2a320",
                        "",
                        2,
                        25,
                        136741,
                        583000,
                    )

            self.assertEqual(b"existing-output", output_path.read_bytes())
            self.assertEqual([output_path], list(output_path.parent.iterdir()))


class MachineReadableResultTest(unittest.TestCase):
    def test_unicode_result_json_is_ascii_safe(self) -> None:
        payload = {"nickname_before": "可爱的徐同学哟"}

        encoded = sav_cli.json.dumps(payload, ensure_ascii=True)

        self.assertTrue(encoded.isascii())
        self.assertEqual(payload, sav_cli.json.loads(encoded))

    def test_conflict_emits_stable_machine_error(self) -> None:
        with (
            tempfile.TemporaryDirectory() as temp_dir,
            patch.object(
                sav_cli.sys,
                "argv",
                [
                    "sav_cli.py",
                    "--mode",
                    "edit-player-stat-points",
                    "--file",
                    str(Path(temp_dir) / "Level.sav"),
                ],
            ),
            patch.object(sav_cli.Path, "is_file", return_value=True),
            patch.object(
                sav_cli,
                "edit_player_stat_points",
                side_effect=sav_cli.PlayerConflictError("changed"),
            ),
            patch.object(sav_cli, "log") as log,
        ):
            self.assertEqual(1, sav_cli.main())

        line = next(
            call.args[0]
            for call in log.call_args_list
            if call.args[0].startswith("SAVE_EDIT_ERROR ")
        )
        payload = sav_cli.json.loads(line.removeprefix("SAVE_EDIT_ERROR "))
        self.assertEqual("stale_state", payload["code"])
        self.assertEqual("changed", payload["message"])


if __name__ == "__main__":
    unittest.main()
