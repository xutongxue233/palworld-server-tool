from __future__ import annotations

import argparse
import ast
import hashlib
import importlib.machinery
import json
import math
import os
import platform
import shutil
import subprocess
import sys
import urllib.request
import zipfile
from pathlib import Path
from uuid import UUID


PST_COMMIT = "2cb6fd963120b002f0732dad153786e624f64b38"
PST_SOURCE_URL = (
    "https://codeload.github.com/deafdudecomputers/PalworldSaveTools/zip/"
    + PST_COMMIT
)
PST_SOURCE_SHA256 = "37c9375b53790312be44c7eab1f5d2b50b6444719ba82bb487467522859146d8"
PST_RELEASE_URL = (
    "https://github.com/deafdudecomputers/PalworldSaveTools/releases/download/"
    "v2.0.0/PST_standalone_v2.0.0.7z"
)
PST_RELEASE_SHA256 = "0e75e8018eaa8a56dfa22465e5bbf7a232c9cda52a314b6ebdc088029347668c"
PALOOZ_MEMBER = "lib/palooz.cp313-win_amd64.pyd"
PAL_EGG_TYPE_B = "EPalItemTypeB::MaterialPalEgg"
BOSS_REWARD_PREFIX = "BossDefeatReward_"
PAL_LEVEL_GAME_VERSION = "1.0.0"
PAL_LEVEL_MAX_LEVEL = 80
PAL_FRIENDSHIP_RANKS = tuple(range(11))
PLAYER_MAP_GAME_VERSION = "1.0.0"
PLAYER_MAP_FAST_TRAVEL_COUNT = 174
PLAYER_MAP_AREA_COUNT = 123
PLAYER_MAP_WORLD_FLAGS = ("MainMap", "Tree")
PAL_CONF_COMMIT = "a0f75513a99684922b0ad58692304f8ddfcc06d3"
WORLD_OPTION_METADATA_SHA256 = (
    "81bb88b68cc6427ed94ea9e29fafa51b443202db514e627eb7163d0674222f34"
)
WORLD_OPTION_METADATA_ENTRIES = 109


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def fetch(url: str, destination: Path, expected_hash: str) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.is_file() and sha256(destination) == expected_hash:
        print(f"Using cached {destination.name}")
        return

    request = urllib.request.Request(url, headers={"User-Agent": "pst-sav-cli-builder"})
    temporary = destination.with_suffix(destination.suffix + ".part")
    temporary.unlink(missing_ok=True)
    with urllib.request.urlopen(request) as response, temporary.open("wb") as output:
        shutil.copyfileobj(response, output)

    actual_hash = sha256(temporary)
    if actual_hash != expected_hash:
        temporary.unlink(missing_ok=True)
        raise RuntimeError(
            f"SHA-256 mismatch for {destination.name}: {actual_hash}"
        )
    temporary.replace(destination)


def extract_source(archive: Path, destination: Path) -> Path:
    marker = destination / ".commit"
    if marker.is_file() and marker.read_text(encoding="ascii") == PST_COMMIT:
        roots = [path for path in destination.iterdir() if path.is_dir()]
        if len(roots) == 1:
            return roots[0]

    shutil.rmtree(destination, ignore_errors=True)
    destination.mkdir(parents=True)
    with zipfile.ZipFile(archive) as source:
        source.extractall(destination)
    marker.write_text(PST_COMMIT, encoding="ascii")
    roots = [path for path in destination.iterdir() if path.is_dir()]
    if len(roots) != 1:
        raise RuntimeError("Unexpected PalworldSaveTools source archive layout")
    return roots[0]


def extract_palooz(archive: Path, destination: Path) -> Path:
    destination.mkdir(parents=True, exist_ok=True)
    output = destination / Path(PALOOZ_MEMBER).name
    if output.is_file():
        return output

    tar = shutil.which("tar")
    if not tar:
        raise RuntimeError("Windows tar.exe is required to extract the verified 7z asset")
    subprocess.run(
        [tar, "-xf", str(archive), "-C", str(destination), PALOOZ_MEMBER],
        check=True,
    )
    extracted = destination / PALOOZ_MEMBER
    if not extracted.is_file():
        raise RuntimeError("palooz runtime was not present in the verified release asset")
    shutil.copy2(extracted, output)
    return output


def require_build_modules() -> None:
    missing: list[str] = []
    for module in ("PyInstaller", "orjson", "requests", "setuptools", "wheel"):
        try:
            __import__(module)
        except ImportError:
            missing.append(module)
    if missing:
        raise RuntimeError(
            "Missing build modules: "
            + ", ".join(missing)
            + ". Run with uv and the pinned build dependencies."
        )


def build_palooz(source_root: Path, staging: Path) -> None:
    source = source_root / "src" / "palsav" / "palooz"
    subprocess.run(
        [sys.executable, "setup.py", "build_ext", "--inplace"],
        check=True,
        cwd=source,
    )
    candidates = [
        candidate
        for candidate in source.glob("palooz*")
        if candidate.is_file()
        and any(
            candidate.name.endswith(suffix)
            for suffix in importlib.machinery.EXTENSION_SUFFIXES
        )
    ]
    if len(candidates) != 1:
        raise RuntimeError(
            f"Expected one compiled palooz extension, found {len(candidates)}"
        )
    shutil.copy2(candidates[0], staging / candidates[0].name)


def load_item_metadata(source_root: Path) -> dict[str, dict[str, object]]:
    source = source_root / "resources" / "game_data" / "items.json"
    payload = json.loads(source.read_text(encoding="utf-8"))
    dynamic_items = payload.get("items_dynamic", {})
    catalog: dict[str, dict[str, object]] = {}
    for item in payload.get("items", []):
        item_id = str(item.get("asset") or "").strip()
        if not item_id:
            continue
        dynamic = dynamic_items.get(item_id, {}).get("dynamic", {})
        catalog[item_id.lower()] = {
            "id": item_id,
            "max_stack": max(1, int(item.get("max_stack") or 1)),
            "type_a": str(item.get("type_a") or ""),
            "type_b": str(item.get("type_b") or ""),
            "dynamic_type": str(dynamic.get("type") or ""),
            "durability": float(dynamic.get("durability") or 0.0),
        }

    return dict(sorted(catalog.items()))


def is_deliverable_item(item: dict[str, object]) -> bool:
    return (
        item.get("type_b") != PAL_EGG_TYPE_B
        and not str(item.get("id") or "").startswith(BOSS_REWARD_PREFIX)
    )


def validate_delivery_rules(catalog: dict[str, dict[str, object]]) -> None:
    repo_root = Path(__file__).resolve().parent.parent
    repo_root_text = str(repo_root)
    added_path = repo_root_text not in sys.path
    if added_path:
        sys.path.insert(0, repo_root_text)
    try:
        from sav_cli.inventory_editor import (
            InventoryEditError,
            ItemDefinition,
            get_item_definition,
        )
    finally:
        if added_path:
            sys.path.remove(repo_root_text)

    backend_catalog = {
        key: ItemDefinition(
            item_id=str(item["id"]),
            max_stack=int(item["max_stack"]),
            type_a=str(item["type_a"]),
            type_b=str(item["type_b"]),
            dynamic_type=str(item["dynamic_type"]),
            durability=float(item["durability"]),
        )
        for key, item in catalog.items()
    }
    backend_deliverable: set[str] = set()
    for key, definition in backend_catalog.items():
        try:
            get_item_definition(backend_catalog, definition.item_id)
        except InventoryEditError:
            continue
        backend_deliverable.add(key)

    generated_deliverable = {
        key for key, item in catalog.items() if is_deliverable_item(item)
    }
    if backend_deliverable != generated_deliverable:
        missing = sorted(backend_deliverable - generated_deliverable)[:5]
        unsupported = sorted(generated_deliverable - backend_deliverable)[:5]
        raise RuntimeError(
            "Frontend item delivery rules do not match the backend: "
            f"missing={missing}, unsupported={unsupported}"
        )


def metadata_payload(
    catalog: dict[str, dict[str, object]],
) -> dict[str, object]:
    return {"source_commit": PST_COMMIT, "items": catalog}


def web_item_catalog_payload(
    catalog: dict[str, dict[str, object]],
) -> dict[str, object]:
    validate_delivery_rules(catalog)
    return {
        "source_commit": PST_COMMIT,
        "item_ids": [
            str(item["id"])
            for item in catalog.values()
            if is_deliverable_item(item)
        ],
    }


def serialize_metadata(payload: dict[str, object]) -> str:
    return json.dumps(
        payload,
        ensure_ascii=True,
        separators=(",", ":"),
    ) + "\n"


def write_metadata(destination: Path, payload: dict[str, object]) -> Path:
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(serialize_metadata(payload), encoding="utf-8")
    return destination


def build_item_metadata(source_root: Path, staging: Path) -> Path:
    catalog = load_item_metadata(source_root)
    return write_metadata(
        staging / "item_metadata.json",
        metadata_payload(catalog),
    )


def build_web_item_catalog(source_root: Path, destination: Path) -> Path:
    catalog = load_item_metadata(source_root)
    return write_metadata(
        destination,
        web_item_catalog_payload(catalog),
    )


def load_source_game_version(source_root: Path) -> str:
    common_path = source_root / "src" / "common.py"
    try:
        module = ast.parse(common_path.read_text(encoding="utf-8"))
    except (OSError, SyntaxError) as exc:
        raise RuntimeError(f"Unable to read upstream game version: {common_path}") from exc
    for statement in module.body:
        if not isinstance(statement, ast.Assign):
            continue
        if not any(
            isinstance(target, ast.Name) and target.id == "GAME_VERSION"
            for target in statement.targets
        ):
            continue
        if isinstance(statement.value, ast.Constant) and isinstance(
            statement.value.value, str
        ):
            return statement.value.value
    raise RuntimeError("Upstream source does not declare GAME_VERSION")


def load_friendship_thresholds(source_root: Path) -> list[int]:
    source = source_root / "resources" / "game_data" / "friendship.json"
    payload = json.loads(source.read_text(encoding="utf-8"))
    if not isinstance(payload, dict):
        raise RuntimeError("Upstream friendship metadata must be a JSON object")
    thresholds: dict[int, int] = {}
    for value in payload.values():
        if not isinstance(value, dict):
            continue
        rank = value.get("FriendshipRank")
        points = value.get("RequiredPoint")
        if rank not in PAL_FRIENDSHIP_RANKS:
            continue
        if (
            isinstance(points, bool)
            or not isinstance(points, int)
            or points < 0
            or rank in thresholds
        ):
            raise RuntimeError("Upstream friendship metadata contains invalid ranks")
        thresholds[rank] = points
    if tuple(sorted(thresholds)) != PAL_FRIENDSHIP_RANKS:
        raise RuntimeError("Upstream friendship metadata must contain ranks 0 through 10")
    values = [thresholds[rank] for rank in PAL_FRIENDSHIP_RANKS]
    if values[0] != 0 or values != sorted(set(values)):
        raise RuntimeError("Upstream friendship thresholds must start at zero and increase")
    return values


def load_pal_level_catalog(source_root: Path) -> dict[str, dict[str, float]]:
    source = source_root / "resources" / "game_data" / "characters.json"
    payload = json.loads(source.read_text(encoding="utf-8"))
    if not isinstance(payload, dict) or not isinstance(payload.get("pals"), list):
        raise RuntimeError("Upstream character metadata is missing its Pal catalog")
    catalog: dict[str, dict[str, float]] = {}
    for pal in payload["pals"]:
        if not isinstance(pal, dict):
            continue
        asset = str(pal.get("asset") or "").strip()
        if not asset:
            continue
        key = asset.lower()
        if key in catalog:
            raise RuntimeError(f"Upstream character metadata duplicates Pal {asset}")
        stats = pal.get("scaling") or pal.get("stats") or {}
        if not isinstance(stats, dict):
            raise RuntimeError(f"Upstream Pal {asset} has invalid stat metadata")
        hp_scaling = stats.get("hp")
        if hp_scaling is None:
            continue
        friendship_hp = pal.get("friendship_hp") or 0
        if (
            isinstance(hp_scaling, bool)
            or not isinstance(hp_scaling, (int, float))
            or not math.isfinite(float(hp_scaling))
            or float(hp_scaling) <= 0
            or isinstance(friendship_hp, bool)
            or not isinstance(friendship_hp, (int, float))
            or not math.isfinite(float(friendship_hp))
            or float(friendship_hp) < 0
        ):
            raise RuntimeError(f"Upstream Pal {asset} has invalid HP metadata")
        catalog[key] = {
            "hp_scaling": float(hp_scaling),
            "friendship_hp": float(friendship_hp),
        }
    if not catalog:
        raise RuntimeError("Upstream character metadata has no usable Pal HP entries")
    return dict(sorted(catalog.items()))


def pal_level_metadata_payload(source_root: Path) -> dict[str, object]:
    game_version = load_source_game_version(source_root)
    if game_version != PAL_LEVEL_GAME_VERSION:
        raise RuntimeError(
            "Pinned PalworldSaveTools source targets game version "
            f"{game_version}, expected {PAL_LEVEL_GAME_VERSION}"
        )
    return {
        "schema": 1,
        "source_commit": PST_COMMIT,
        "game_version": game_version,
        "max_level": PAL_LEVEL_MAX_LEVEL,
        "friendship_thresholds": load_friendship_thresholds(source_root),
        "pals": load_pal_level_catalog(source_root),
    }


def build_pal_level_metadata(source_root: Path, staging: Path) -> Path:
    return write_metadata(
        staging / "pal_level_metadata.json",
        pal_level_metadata_payload(source_root),
    )


def load_fast_travel_guids(source_root: Path) -> list[str]:
    source = source_root / "resources" / "game_data" / "fast_travel_points.json"
    payload = json.loads(source.read_text(encoding="utf-8"))
    if not isinstance(payload, dict):
        raise RuntimeError("Upstream fast travel metadata must be a JSON object")
    guids: list[str] = []
    seen: set[str] = set()
    for raw_guid in payload:
        if not isinstance(raw_guid, str) or not raw_guid or raw_guid != raw_guid.strip():
            raise RuntimeError("Upstream fast travel metadata contains an invalid GUID")
        try:
            guid = UUID(raw_guid).hex.upper()
        except (AttributeError, TypeError, ValueError) as exc:
            raise RuntimeError(
                f"Upstream fast travel metadata contains invalid GUID {raw_guid}"
            ) from exc
        if guid == "0" * 32:
            raise RuntimeError("Upstream fast travel metadata contains a zero GUID")
        if guid in seen:
            raise RuntimeError(
                f"Upstream fast travel metadata duplicates GUID {guid}"
            )
        seen.add(guid)
        guids.append(guid)
    if len(guids) != PLAYER_MAP_FAST_TRAVEL_COUNT:
        raise RuntimeError(
            "Upstream fast travel metadata must contain exactly "
            f"{PLAYER_MAP_FAST_TRAVEL_COUNT} GUIDs"
        )
    return sorted(guids)


def load_world_map_areas(source_root: Path) -> list[str]:
    source = source_root / "resources" / "game_data" / "world_map_areas.json"
    payload = json.loads(source.read_text(encoding="utf-8"))
    if not isinstance(payload, dict) or not isinstance(payload.get("areas"), list):
        raise RuntimeError("Upstream world map metadata is missing its area list")
    areas: list[str] = []
    seen: set[str] = set()
    for area in payload["areas"]:
        if not isinstance(area, str) or not area or area != area.strip():
            raise RuntimeError("Upstream world map metadata contains an invalid area ID")
        if area in seen:
            raise RuntimeError(
                f"Upstream world map metadata duplicates area ID {area}"
            )
        seen.add(area)
        areas.append(area)
    if len(areas) != PLAYER_MAP_AREA_COUNT:
        raise RuntimeError(
            "Upstream world map metadata must contain exactly "
            f"{PLAYER_MAP_AREA_COUNT} area IDs"
        )
    return sorted(areas)


def player_map_metadata_payload(source_root: Path) -> dict[str, object]:
    game_version = load_source_game_version(source_root)
    if game_version != PLAYER_MAP_GAME_VERSION:
        raise RuntimeError(
            "Pinned PalworldSaveTools source targets game version "
            f"{game_version}, expected {PLAYER_MAP_GAME_VERSION}"
        )
    return {
        "schema": 1,
        "source_commit": PST_COMMIT,
        "game_version": game_version,
        "fast_travel_guids": load_fast_travel_guids(source_root),
        "areas": load_world_map_areas(source_root),
        "world_flags": list(PLAYER_MAP_WORLD_FLAGS),
    }


def build_player_map_metadata(source_root: Path, staging: Path) -> Path:
    return write_metadata(
        staging / "player_map_metadata.json",
        player_map_metadata_payload(source_root),
    )


def check_web_item_catalog(source_root: Path, destination: Path) -> None:
    expected = serialize_metadata(
        web_item_catalog_payload(load_item_metadata(source_root))
    )
    if not destination.is_file():
        raise RuntimeError(
            f"Generated web item catalog is missing: {destination}. "
            "Run with --generate-web-item-catalog."
        )
    if destination.read_text(encoding="utf-8") != expected:
        raise RuntimeError(
            f"Generated web item catalog is stale: {destination}. "
            "Run with --generate-web-item-catalog."
        )


def prepare_source(cache: Path) -> Path:
    source_archive = cache / f"PalworldSaveTools-{PST_COMMIT}.zip"
    fetch(PST_SOURCE_URL, source_archive, PST_SOURCE_SHA256)
    return extract_source(source_archive, cache / "source")


def copy_player_exp_table(source_root: Path, staging: Path) -> Path:
    source = source_root / "resources" / "game_data" / "pal_exp_table.json"
    destination = staging / "pal_exp_table.json"
    shutil.copy2(source, destination)
    return destination


def copy_world_option_metadata(repo_root: Path, staging: Path) -> Path:
    source = repo_root / "sav_cli" / "world_option_metadata.json"
    if not source.is_file() or sha256(source) != WORLD_OPTION_METADATA_SHA256:
        raise RuntimeError("Pinned Palworld 1.0.0 WorldOption metadata checksum mismatch")
    try:
        payload = json.loads(source.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise RuntimeError("Unable to read WorldOption metadata") from exc
    if (
        payload.get("schema") != 1
        or payload.get("game_version") != "1.0.0"
        or payload.get("source_commit") != PAL_CONF_COMMIT
        or not isinstance(payload.get("settings"), dict)
        or len(payload["settings"]) != WORLD_OPTION_METADATA_ENTRIES
    ):
        raise RuntimeError("Pinned WorldOption metadata is incomplete or stale")
    destination = staging / source.name
    shutil.copy2(source, destination)
    return destination


def build(
    repo_root: Path,
    output: Path,
    cache: Path,
    web_item_catalog: Path,
) -> None:
    machine = platform.machine().lower()
    if machine not in {"amd64", "x86_64", "arm64", "aarch64"}:
        raise RuntimeError(f"Unsupported architecture: {machine}")
    if sys.maxsize <= 2**32:
        raise RuntimeError("A 64-bit Python runtime is required")
    if os.name == "nt" and sys.version_info[:2] != (3, 13):
        raise RuntimeError("The verified palooz runtime requires CPython 3.13")

    require_build_modules()
    source_root = prepare_source(cache)
    staging = cache / "staging"
    shutil.rmtree(staging, ignore_errors=True)
    staging.mkdir(parents=True)

    shutil.copytree(source_root / "src" / "palsav" / "palsav", staging / "palsav")
    if os.name == "nt":
        release_archive = cache / "PST_standalone_v2.0.0.7z"
        fetch(PST_RELEASE_URL, release_archive, PST_RELEASE_SHA256)
        palooz = extract_palooz(release_archive, cache / "runtime")
        shutil.copy2(palooz, staging / palooz.name)
    else:
        build_palooz(source_root, staging)
    for name in (
        "inventory_editor.py",
        "logger.py",
        "pal_editor.py",
        "player_editor.py",
        "player_save_editor.py",
        "sav_cli.py",
        "structurer.py",
        "world_types.py",
        "world_option_editor.py",
    ):
        shutil.copy2(repo_root / "sav_cli" / name, staging / name)
    metadata = build_item_metadata(source_root, staging)
    pal_level_metadata = build_pal_level_metadata(source_root, staging)
    player_map_metadata = build_player_map_metadata(source_root, staging)
    world_option_metadata = copy_world_option_metadata(repo_root, staging)
    build_web_item_catalog(source_root, web_item_catalog)
    exp_table = copy_player_exp_table(source_root, staging)

    build_root = cache / "pyinstaller"
    dist = build_root / "dist"
    shutil.rmtree(build_root, ignore_errors=True)
    output.parent.mkdir(parents=True, exist_ok=True)

    command = [
        sys.executable,
        "-m",
        "PyInstaller",
        "--noconfirm",
        "--clean",
        "--onefile",
        "--name",
        "sav_cli",
        "--paths",
        str(staging),
        "--collect-submodules",
        "palsav",
        "--hidden-import",
        "palooz",
        "--add-data",
        f"{metadata}{os.pathsep}.",
        "--add-data",
        f"{exp_table}{os.pathsep}.",
        "--add-data",
        f"{pal_level_metadata}{os.pathsep}.",
        "--add-data",
        f"{player_map_metadata}{os.pathsep}.",
        "--add-data",
        f"{world_option_metadata}{os.pathsep}.",
        "--distpath",
        str(dist),
        "--workpath",
        str(build_root / "work"),
        "--specpath",
        str(build_root / "spec"),
        str(staging / "sav_cli.py"),
    ]
    if os.name == "nt":
        command[3:3] = ["--icon", str(repo_root / "build" / "windows" / "pst.ico")]
    subprocess.run(command, check=True, cwd=staging)
    executable_name = "sav_cli.exe" if os.name == "nt" else "sav_cli"
    built = dist / executable_name
    if not built.is_file():
        raise RuntimeError(f"PyInstaller did not produce {executable_name}")
    shutil.copy2(built, output)
    if os.name != "nt":
        output.chmod(0o755)

    license_target = output.parent / "sav_cli-GPL-3.0.txt"
    shutil.copy2(source_root / "src" / "palsav" / "LICENSE", license_target)
    print(f"sav_cli built: {output}")
    print(f"sav_cli SHA-256: {sha256(output)}")


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Build PST sav_cli with the current palsav engine"
    )
    parser.add_argument("--output", default="dist/sav_cli.exe")
    parser.add_argument("--cache-dir", default=".cache/sav-cli-build")
    parser.add_argument(
        "--web-item-catalog",
        default="web/src/assets/deliverable-items.json",
    )
    catalog_action = parser.add_mutually_exclusive_group()
    catalog_action.add_argument(
        "--generate-web-item-catalog",
        action="store_true",
        help="Generate the frontend allowlist from pinned PalworldSaveTools metadata",
    )
    catalog_action.add_argument(
        "--check-web-item-catalog",
        action="store_true",
        help="Fail when the generated frontend allowlist is missing or stale",
    )
    args = parser.parse_args()

    repo_root = Path(__file__).resolve().parent.parent
    output = (repo_root / args.output).resolve()
    cache = (repo_root / args.cache_dir).resolve()
    web_item_catalog = (repo_root / args.web_item_catalog).resolve()
    if args.generate_web_item_catalog or args.check_web_item_catalog:
        source_root = prepare_source(cache)
        if args.generate_web_item_catalog:
            build_web_item_catalog(source_root, web_item_catalog)
            print(f"Web item catalog generated: {web_item_catalog}")
        else:
            check_web_item_catalog(source_root, web_item_catalog)
            print(f"Web item catalog is current: {web_item_catalog}")
        return
    build(repo_root, output, cache, web_item_catalog)


if __name__ == "__main__":
    main()
