#!/usr/bin/env python3

import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import time
import urllib.error
import urllib.request


RETRY_TIMES = 3
DEFAULT_WORKERS = 16
DEFAULT_TIMEOUT = 30
DEFAULT_TILE_URL = "https://cdn.paldb.cc/image/map8/z{z}x{x}y{y}.webp"
DEFAULT_MAP_DATA_URL = "https://paldb.cc/js/map_data_cn.js"
DEFAULT_SAVE_DIR = Path("./map")
DEFAULT_POINTS_FILE = Path("web/src/assets/map/points.json")
MAX_NATIVE_ZOOM = 4
USER_AGENT = (
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
    "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131 Safari/537.36"
)

# Older PalDB data stored fixed locations in its compact in-game map coordinate
# system. Current data exposes world coordinates directly through `pos`, while
# `ipos` remains supported for compatibility with cached or mirrored payloads.
PALDB_WORLD_UNITS_PER_IPOS = 459
PALDB_WORLD_X_OFFSET = -123_888
PALDB_WORLD_Y_OFFSET = 158_000


def fetch_bytes(url, timeout=DEFAULT_TIMEOUT):
    request = urllib.request.Request(
        url,
        headers={
            "User-Agent": USER_AGENT,
            "Referer": "https://paldb.cc/",
        },
    )
    last_error = None
    for attempt in range(1, RETRY_TIMES + 1):
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                return response.read()
        except (OSError, urllib.error.URLError) as error:
            last_error = error
            if attempt < RETRY_TIMES:
                time.sleep(attempt)
    raise RuntimeError(f"failed to download {url}: {last_error}")


def atomic_write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary_path = path.with_name(f"{path.name}.tmp")
    with temporary_path.open("wb") as file:
        file.write(content)
    os.replace(temporary_path, path)


def is_webp(content):
    return (
        len(content) >= 12
        and content[:4] == b"RIFF"
        and content[8:12] == b"WEBP"
    )


def tile_jobs(save_dir, base_url):
    for zoom in range(MAX_NATIVE_ZOOM + 1):
        tile_count = 2**zoom
        for x in range(tile_count):
            for y in range(tile_count):
                yield (
                    base_url.format(z=zoom, x=x, y=y),
                    save_dir / str(zoom) / str(x) / f"{y}.webp",
                )


def download_tile(job, redownload=False, timeout=DEFAULT_TIMEOUT):
    url, path = job
    if path.exists() and not redownload:
        return "cached", path

    content = fetch_bytes(url, timeout=timeout)
    if not is_webp(content):
        raise RuntimeError(f"downloaded tile is not WebP: {url}")
    atomic_write(path, content)
    return "downloaded", path


def download_tiles(
    save_dir,
    base_url=DEFAULT_TILE_URL,
    redownload=False,
    workers=DEFAULT_WORKERS,
    timeout=DEFAULT_TIMEOUT,
):
    jobs = list(tile_jobs(save_dir, base_url))
    completed = 0
    downloaded = 0
    failures = []

    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as executor:
        futures = {
            executor.submit(download_tile, job, redownload, timeout): job
            for job in jobs
        }
        for future in concurrent.futures.as_completed(futures):
            completed += 1
            try:
                status, _ = future.result()
                if status == "downloaded":
                    downloaded += 1
            except Exception as error:  # noqa: BLE001
                failures.append(str(error))
            if completed % 25 == 0 or completed == len(jobs):
                print(f"Map tiles: {completed}/{len(jobs)}", flush=True)

    if failures:
        preview = "\n".join(failures[:10])
        raise RuntimeError(f"failed to download {len(failures)} map tiles:\n{preview}")

    print(
        f"Map tiles ready: {len(jobs)} total, {downloaded} downloaded, "
        f"{len(jobs) - downloaded} cached"
    )


def extract_json_assignment(script, variable_name):
    marker = f"var {variable_name} = "
    marker_index = script.find(marker)
    if marker_index < 0:
        raise ValueError(f"{variable_name} assignment was not found")
    value_index = marker_index + len(marker)
    value, _ = json.JSONDecoder().raw_decode(script[value_index:])
    return value


def ipos_to_world(ipos):
    x = float(ipos["Y"]) * PALDB_WORLD_UNITS_PER_IPOS + PALDB_WORLD_X_OFFSET
    y = float(ipos["X"]) * PALDB_WORLD_UNITS_PER_IPOS + PALDB_WORLD_Y_OFFSET
    return [round(x, 3), round(y, 3)]


def location_to_world(location):
    pos = location.get("pos")
    if isinstance(pos, dict) and "X" in pos and "Y" in pos:
        return [round(float(pos["X"]), 3), round(float(pos["Y"]), 3)]

    ipos = location.get("ipos")
    if isinstance(ipos, dict) and "X" in ipos and "Y" in ipos:
        return ipos_to_world(ipos)
    return None


def build_points(fixed_locations):
    points = {"boss_tower": [], "fast_travel": []}
    seen = {"boss_tower": set(), "fast_travel": set()}
    type_to_key = {"Tower": "boss_tower", "Fast Travel": "fast_travel"}

    for location in fixed_locations:
        key = type_to_key.get(location.get("type"))
        if key is None:
            continue

        position = location_to_world(location)
        if position is None:
            continue
        identity = tuple(position)
        if identity in seen[key]:
            continue
        seen[key].add(identity)
        points[key].append(position)

    return points


def refresh_points(
    points_file,
    map_data_url=DEFAULT_MAP_DATA_URL,
    timeout=DEFAULT_TIMEOUT,
):
    script = fetch_bytes(map_data_url, timeout=timeout).decode("utf-8")
    fixed_locations = extract_json_assignment(script, "fixedDungeon")
    if not isinstance(fixed_locations, list):
        raise ValueError("fixedDungeon must be an array")

    points = build_points(fixed_locations)
    if not points["boss_tower"] or not points["fast_travel"]:
        raise ValueError("PalDB map data did not contain required map points")

    content = (json.dumps(points, ensure_ascii=False, indent=2) + "\n").encode(
        "utf-8"
    )
    atomic_write(points_file, content)
    print(
        "Map points ready: "
        f"{len(points['boss_tower'])} towers, "
        f"{len(points['fast_travel'])} fast travel points"
    )
    return points


def parse_args():
    parser = argparse.ArgumentParser(
        description="Download current PalDB map tiles and marker data"
    )
    parser.add_argument("--redownload", action="store_true")
    parser.add_argument("--skip-tiles", action="store_true")
    parser.add_argument("--skip-points", action="store_true")
    parser.add_argument("--workers", type=int, default=DEFAULT_WORKERS)
    parser.add_argument("--timeout", type=int, default=DEFAULT_TIMEOUT)
    parser.add_argument("--save-dir", type=Path, default=DEFAULT_SAVE_DIR)
    parser.add_argument("--points-file", type=Path, default=DEFAULT_POINTS_FILE)
    parser.add_argument("--tile-base-url", default=DEFAULT_TILE_URL)
    parser.add_argument("--map-data-url", default=DEFAULT_MAP_DATA_URL)
    args = parser.parse_args()
    if args.skip_tiles and args.skip_points:
        parser.error("at least one of tiles or points must be refreshed")
    if args.workers < 1:
        parser.error("workers must be positive")
    if args.timeout < 1:
        parser.error("timeout must be positive")
    return args


def main():
    args = parse_args()
    if not args.skip_tiles:
        download_tiles(
            args.save_dir,
            base_url=args.tile_base_url,
            redownload=args.redownload,
            workers=args.workers,
            timeout=args.timeout,
        )
    if not args.skip_points:
        refresh_points(
            args.points_file,
            map_data_url=args.map_data_url,
            timeout=args.timeout,
        )


if __name__ == "__main__":
    main()
