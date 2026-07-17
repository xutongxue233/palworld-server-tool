import unittest
from pathlib import Path

import map_down


class MapDownloadTest(unittest.TestCase):
    def test_extract_json_assignment_does_not_execute_javascript(self):
        script = 'alert("ignored");var fixedDungeon = [{"type":"Tower"}];next();'

        value = map_down.extract_json_assignment(script, "fixedDungeon")

        self.assertEqual(value, [{"type": "Tower"}])

    def test_build_points_converts_and_deduplicates_paldb_coordinates(self):
        locations = [
            {"type": "Tower", "ipos": {"X": 36, "Y": -311}},
            {"type": "Tower", "ipos": {"X": 36, "Y": -311}},
            {"type": "Fast Travel", "ipos": {"X": -172, "Y": 33}},
            {"type": "Dungeon", "ipos": {"X": 1, "Y": 2}},
        ]

        points = map_down.build_points(locations)

        self.assertEqual(points["boss_tower"], [[-266637.0, 174524.0]])
        self.assertEqual(points["fast_travel"], [[-108741.0, 79052.0]])

    def test_build_points_accepts_current_paldb_world_positions(self):
        locations = [
            {
                "type": "Tower",
                "pos": {"X": -266563, "Y": 174506, "Z": 5930},
                "ipos": {"X": 36, "Y": -311},
            },
            {
                "type": "Fast Travel",
                "pos": {"X": -108667, "Y": 79120, "Z": 396},
            },
            {"type": "Tower", "pos": {"X": -266563, "Y": 174506}},
            {"type": "Tower", "pos": {"X": 1}},
        ]

        points = map_down.build_points(locations)

        self.assertEqual(points["boss_tower"], [[-266563.0, 174506.0]])
        self.assertEqual(points["fast_travel"], [[-108667.0, 79120.0]])

    def test_tile_jobs_cover_all_native_zoom_tiles(self):
        jobs = list(map_down.tile_jobs(Path("map"), "z{z}x{x}y{y}"))

        self.assertEqual(len(jobs), 341)
        self.assertEqual(jobs[0][0], "z0x0y0")
        self.assertEqual(jobs[-1][0], "z4x15y15")


if __name__ == "__main__":
    unittest.main()
