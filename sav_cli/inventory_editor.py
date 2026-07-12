from __future__ import annotations

import copy
import json
import math
import re
import sys
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any


ZERO_UUID = "00000000-0000-0000-0000-000000000000"
SLOT_RAW_TYPE = ".worldSaveData.ItemContainerSaveData.Value.Slots.Slots.RawData"
DYNAMIC_RAW_TYPE = ".worldSaveData.DynamicItemSaveData.DynamicItemSaveData.RawData"

CONTAINER_PROPERTIES = {
    "main": "CommonContainerId",
    "key": "EssentialContainerId",
    "weapons": "WeaponLoadOutContainerId",
    "armor": "PlayerEquipArmorContainerId",
    "food": "FoodEquipContainerId",
    "drop": "DropSlotContainerId",
}

SLOT_CUSTOM_VERSION = [
    2,
    0,
    0,
    0,
    126,
    244,
    234,
    18,
    154,
    27,
    90,
    255,
    113,
    170,
    113,
    188,
    223,
    51,
    214,
    14,
    1,
    0,
    0,
    0,
    56,
    11,
    0,
    222,
    73,
    73,
    215,
    206,
    151,
    223,
    45,
    153,
    192,
    193,
    195,
    105,
    1,
    0,
    0,
    0,
]

DYNAMIC_CUSTOM_VERSION = [
    1,
    0,
    0,
    0,
    56,
    11,
    0,
    222,
    73,
    73,
    215,
    206,
    151,
    223,
    45,
    153,
    192,
    193,
    195,
    105,
    1,
    0,
    0,
    0,
]


class InventoryEditError(ValueError):
    pass


class InventoryConflictError(InventoryEditError):
    pass


@dataclass(frozen=True)
class ItemDefinition:
    item_id: str
    max_stack: int
    type_a: str = ""
    type_b: str = ""
    dynamic_type: str = ""
    durability: float = 0.0

    @property
    def is_essential(self) -> bool:
        return self.type_a == "EPalItemTypeA::Essential"

    @property
    def is_egg(self) -> bool:
        return self.type_b == "EPalItemTypeB::MaterialPalEgg"

    @property
    def needs_dynamic_data(self) -> bool:
        return self.dynamic_type in {"weapon", "armor"}


@dataclass(frozen=True)
class DeliveryResult:
    player_uid: str
    item_id: str
    container: str
    requested: int
    delivered: int
    before: int
    after: int
    modified_slots: tuple[int, ...]
    dynamic_ids: dict[int, str]

    def to_dict(self) -> dict[str, Any]:
        return {
            "player_uid": self.player_uid,
            "item_id": self.item_id,
            "container": self.container,
            "requested": self.requested,
            "delivered": self.delivered,
            "before": self.before,
            "after": self.after,
            "modified_slots": list(self.modified_slots),
            "dynamic_ids": {str(index): value for index, value in self.dynamic_ids.items()},
        }


@dataclass(frozen=True)
class InventoryMutationResult:
    player_uid: str
    item_id: str
    container: str
    slot_index: int
    before: int
    after: int
    removed: bool
    dynamic_record_removed: bool
    dynamic_id: str = ZERO_UUID

    def to_dict(self) -> dict[str, Any]:
        return {
            "player_uid": self.player_uid,
            "item_id": self.item_id,
            "container": self.container,
            "slot_index": self.slot_index,
            "before": self.before,
            "after": self.after,
            "removed": self.removed,
            "dynamic_record_removed": self.dynamic_record_removed,
            "dynamic_id": self.dynamic_id,
        }


def default_metadata_path() -> Path:
    bundle_dir = Path(getattr(sys, "_MEIPASS", Path(__file__).resolve().parent))
    return bundle_dir / "item_metadata.json"


def load_item_catalog(path: str = "") -> dict[str, ItemDefinition]:
    metadata_path = Path(path) if path else default_metadata_path()
    if not metadata_path.is_file():
        raise InventoryEditError(f"Item metadata does not exist: {metadata_path}")

    payload = json.loads(metadata_path.read_text(encoding="utf-8"))
    raw_items = payload.get("items", payload)
    if not isinstance(raw_items, dict):
        raise InventoryEditError("Item metadata must contain an object named 'items'")

    catalog: dict[str, ItemDefinition] = {}
    for key, value in raw_items.items():
        if not isinstance(value, dict):
            continue
        item_id = str(value.get("id") or key).strip()
        if not item_id:
            continue
        max_stack = max(1, int(value.get("max_stack") or 1))
        catalog[item_id.lower()] = ItemDefinition(
            item_id=item_id,
            max_stack=max_stack,
            type_a=str(value.get("type_a") or ""),
            type_b=str(value.get("type_b") or ""),
            dynamic_type=str(value.get("dynamic_type") or ""),
            durability=float(value.get("durability") or 0.0),
        )
    if not catalog:
        raise InventoryEditError("Item metadata is empty")
    return catalog


def get_item_definition(
    catalog: dict[str, ItemDefinition], item_id: str
) -> ItemDefinition:
    normalized = item_id.strip().lower()
    if not normalized:
        raise InventoryEditError("Item ID cannot be empty")
    definition = catalog.get(normalized)
    if definition is None:
        raise InventoryEditError(f"Unknown item ID: {item_id}")
    if definition.is_egg:
        raise InventoryEditError(
            "Pal eggs contain a Pal-specific dynamic record and cannot be delivered safely"
        )
    if definition.item_id.startswith("BossDefeatReward_"):
        raise InventoryEditError(
            "Boss reward tokens require player progression updates and are not supported"
        )
    return definition


def resolve_player_save(level_path: Path, player_uid: str) -> Path:
    players_dir = level_path.parent / "Players"
    if not players_dir.is_dir():
        raise InventoryEditError(f"Players directory does not exist: {players_dir}")

    raw_uid = player_uid.strip().replace("-", "")
    if not raw_uid:
        raise InventoryEditError("Player UID cannot be empty")

    exact_name = ""
    prefix = ""
    if raw_uid.isdecimal() and len(raw_uid) <= 10:
        numeric_uid = int(raw_uid, 10)
        if numeric_uid < 0 or numeric_uid > 0xFFFFFFFF:
            raise InventoryEditError("Decimal player UID must fit in 32 bits")
        prefix = f"{numeric_uid:08X}"
        exact_name = prefix + ("0" * 24) + ".SAV"
    elif re.fullmatch(r"[0-9a-fA-F]{8}", raw_uid):
        prefix = raw_uid.upper()
    elif re.fullmatch(r"[0-9a-fA-F]{32}", raw_uid):
        exact_name = raw_uid.upper() + ".SAV"
        prefix = raw_uid[:8].upper()
    else:
        raise InventoryEditError(
            "Player UID must be a decimal ID, 8 hexadecimal digits, or a GUID"
        )

    candidates = sorted(
        (path for path in players_dir.glob("*.sav") if path.is_file()),
        key=lambda path: path.name.upper(),
    )
    if exact_name:
        exact = [path for path in candidates if path.name.upper() == exact_name]
        if len(exact) == 1:
            return exact[0]

    prefixed = [path for path in candidates if path.stem.upper().startswith(prefix)]
    if len(prefixed) == 1:
        return prefixed[0]
    if not prefixed:
        raise InventoryEditError(f"Player save was not found for UID {player_uid}")
    names = ", ".join(path.name for path in prefixed[:5])
    raise InventoryEditError(
        f"Player UID {player_uid} matches multiple save files: {names}"
    )


def _properties(gvas_or_properties: Any) -> dict[str, Any]:
    if hasattr(gvas_or_properties, "properties"):
        return gvas_or_properties.properties
    if isinstance(gvas_or_properties, dict):
        if "properties" in gvas_or_properties:
            return gvas_or_properties["properties"]
        return gvas_or_properties
    raise InventoryEditError("Unsupported GVAS data object")


def _world(level_gvas: Any) -> dict[str, Any]:
    try:
        return _properties(level_gvas)["worldSaveData"]["value"]
    except (KeyError, TypeError) as exc:
        raise InventoryEditError("Level.sav does not contain worldSaveData") from exc


def _container_id(player_gvas: Any, container: str) -> str:
    property_name = CONTAINER_PROPERTIES[container]
    try:
        save_data = _properties(player_gvas)["SaveData"]["value"]
        inventory = save_data["InventoryInfo"]["value"]
        value = inventory[property_name]["value"]["ID"]["value"]
    except (KeyError, TypeError) as exc:
        raise InventoryEditError(
            f"Player save does not contain {property_name}"
        ) from exc
    container_id = str(value)
    if not container_id or container_id == ZERO_UUID:
        raise InventoryEditError(f"Player container {property_name} is empty")
    return container_id


def _find_container(world: dict[str, Any], container_id: str) -> dict[str, Any]:
    try:
        containers = world["ItemContainerSaveData"]["value"]
    except (KeyError, TypeError) as exc:
        raise InventoryEditError("Level.sav does not contain item containers") from exc
    for candidate in containers:
        value = candidate.get("key", {}).get("ID", {}).get("value", "")
        if str(value).lower() == container_id.lower():
            return candidate
    raise InventoryEditError(
        f"Player inventory container {container_id} is missing from Level.sav"
    )


def _slot_values(container_data: dict[str, Any]) -> list[dict[str, Any]]:
    try:
        values = container_data["value"]["Slots"]["value"]["values"]
    except (KeyError, TypeError) as exc:
        raise InventoryEditError("Item container does not contain a slot array") from exc
    if not isinstance(values, list):
        raise InventoryEditError("Item container slot array is invalid")
    return values


def _slot_count(container_data: dict[str, Any]) -> int:
    try:
        count = int(container_data["value"]["SlotNum"]["value"])
    except (KeyError, TypeError, ValueError) as exc:
        raise InventoryEditError("Item container does not contain a valid slot count") from exc
    if count <= 0:
        raise InventoryEditError("Item container has no available slots")
    return count


def _raw_slot(slot: dict[str, Any]) -> dict[str, Any]:
    try:
        raw = slot["RawData"]["value"]
    except (KeyError, TypeError) as exc:
        raise InventoryEditError("Item container contains an invalid slot") from exc
    if not isinstance(raw, dict):
        raise InventoryEditError("Item slot RawData is invalid")
    return raw


def _dynamic_values(world: dict[str, Any]) -> list[dict[str, Any]]:
    try:
        values = world["DynamicItemSaveData"]["value"]["values"]
    except (KeyError, TypeError) as exc:
        raise InventoryEditError("Level.sav does not contain DynamicItemSaveData") from exc
    if not isinstance(values, list):
        raise InventoryEditError("DynamicItemSaveData is invalid")
    return values


def _find_slot_template(world: dict[str, Any]) -> dict[str, Any] | None:
    containers = world.get("ItemContainerSaveData", {}).get("value", [])
    for container in containers:
        for slot in _slot_values(container):
            if isinstance(slot, dict) and "RawData" in slot:
                return slot
    return None


def _new_slot(
    template: dict[str, Any] | None,
    slot_index: int,
    item_id: str,
    count: int,
    dynamic_id: str = ZERO_UUID,
) -> dict[str, Any]:
    if template is None:
        slot = {
            "RawData": {
                "array_type": "ByteProperty",
                "id": None,
                "value": {},
                "type": "ArrayProperty",
                "custom_type": SLOT_RAW_TYPE,
            },
            "CustomVersionData": {
                "array_type": "ByteProperty",
                "id": None,
                "value": {"values": list(SLOT_CUSTOM_VERSION)},
                "type": "ArrayProperty",
            },
        }
        trailing_length = 20
    else:
        slot = copy.deepcopy(template)
        trailing = _raw_slot(template).get("trailing_bytes", [])
        trailing_length = len(trailing) if trailing is not None else 20
        trailing_length = trailing_length or 20

    raw_property = slot.setdefault("RawData", {})
    raw_property["array_type"] = "ByteProperty"
    raw_property["id"] = None
    raw_property["type"] = "ArrayProperty"
    raw_property["custom_type"] = SLOT_RAW_TYPE
    raw_property["value"] = {
        "slot_index": slot_index,
        "count": count,
        "item": {
            "static_id": item_id,
            "dynamic_id": {
                "created_world_id": ZERO_UUID,
                "local_id_in_created_world": dynamic_id,
            },
        },
        "trailing_bytes": [0] * trailing_length,
    }
    return slot


def _new_dynamic_item(
    world: dict[str, Any], definition: ItemDefinition, dynamic_id: str
) -> dict[str, Any]:
    values = _dynamic_values(world)
    version_template: dict[str, Any] | None = None
    for existing in values:
        raw = existing.get("RawData", {}).get("value", {})
        if raw.get("type") == definition.dynamic_type:
            version_template = existing.get("CustomVersionData")
            break

    if version_template is None:
        custom_version = {
            "array_type": "ByteProperty",
            "id": None,
            "value": {"values": list(DYNAMIC_CUSTOM_VERSION)},
            "type": "ArrayProperty",
        }
    else:
        custom_version = copy.deepcopy(version_template)

    raw_value: dict[str, Any] = {
        "type": definition.dynamic_type,
        "id": {
            "created_world_id": ZERO_UUID,
            "local_id_in_created_world": dynamic_id,
            "static_id": definition.item_id,
        },
        "leading_bytes": [0] * 4,
        "durability": float(definition.durability),
        "trailing_bytes": [0] * 4,
    }
    if definition.dynamic_type == "weapon":
        raw_value["remaining_bullets"] = 0
        raw_value["passive_skill_list"] = []
    elif definition.dynamic_type != "armor":
        raise InventoryEditError(
            f"Unsupported dynamic item type: {definition.dynamic_type}"
        )

    return {
        "RawData": {
            "array_type": "ByteProperty",
            "id": None,
            "value": raw_value,
            "type": "ArrayProperty",
            "custom_type": DYNAMIC_RAW_TYPE,
        },
        "CustomVersionData": custom_version,
    }


def _inventory_total(slots: list[dict[str, Any]], item_id: str) -> int:
    total = 0
    for slot in slots:
        raw = _raw_slot(slot)
        if str(raw.get("item", {}).get("static_id", "")).lower() == item_id.lower():
            total += int(raw.get("count", 0))
    return total


def _slot_dynamic_id(raw: dict[str, Any]) -> str:
    return str(
        raw.get("item", {})
        .get("dynamic_id", {})
        .get("local_id_in_created_world", ZERO_UUID)
    ).lower()


def _remove_dynamic_record_if_unreferenced(
    world: dict[str, Any], dynamic_id: str
) -> bool:
    normalized = dynamic_id.lower()
    if not normalized or normalized == ZERO_UUID:
        return False

    for container in world.get("ItemContainerSaveData", {}).get("value", []):
        for slot in _slot_values(container):
            if _slot_dynamic_id(_raw_slot(slot)) == normalized:
                return False

    values = _dynamic_values(world)
    matches = [
        item
        for item in values
        if str(
            item.get("RawData", {})
            .get("value", {})
            .get("id", {})
            .get("local_id_in_created_world", "")
        ).lower()
        == normalized
    ]
    if len(matches) != 1:
        raise InventoryEditError(
            f"Expected one dynamic item record for {dynamic_id}, found {len(matches)}"
        )
    values.remove(matches[0])
    return True


def _validate_container(container_data: dict[str, Any]) -> None:
    slots = _slot_values(container_data)
    slot_count = _slot_count(container_data)
    seen: set[int] = set()
    for slot in slots:
        raw = _raw_slot(slot)
        index = int(raw.get("slot_index", -1))
        count = int(raw.get("count", 0))
        item_id = str(raw.get("item", {}).get("static_id", ""))
        if index < 0 or index >= slot_count:
            raise InventoryEditError(f"Item slot index {index} is out of range")
        if index in seen:
            raise InventoryEditError(f"Item slot index {index} is duplicated")
        if count <= 0 or not item_id or item_id.lower() == "none":
            raise InventoryEditError(f"Item slot {index} contains an invalid item")
        seen.add(index)


def _validate_dynamic_references(world: dict[str, Any]) -> None:
    dynamic_ids: set[str] = set()
    for item in _dynamic_values(world):
        value = item.get("RawData", {}).get("value", {})
        dynamic_id = str(
            value.get("id", {}).get("local_id_in_created_world", "")
        ).lower()
        if not dynamic_id or dynamic_id == ZERO_UUID:
            raise InventoryEditError("Dynamic item record contains an empty ID")
        if dynamic_id in dynamic_ids:
            raise InventoryEditError(f"Dynamic item ID {dynamic_id} is duplicated")
        dynamic_ids.add(dynamic_id)

    containers = world.get("ItemContainerSaveData", {}).get("value", [])
    for container in containers:
        for slot in _slot_values(container):
            raw = _raw_slot(slot)
            dynamic_id = str(
                raw.get("item", {})
                .get("dynamic_id", {})
                .get("local_id_in_created_world", ZERO_UUID)
            ).lower()
            if dynamic_id != ZERO_UUID and dynamic_id not in dynamic_ids:
                raise InventoryEditError(
                    f"Item slot references missing dynamic item {dynamic_id}"
                )


def deliver_item(
    level_gvas: Any,
    player_gvas: Any,
    player_uid: str,
    definition: ItemDefinition,
    quantity: int,
    container: str = "auto",
) -> DeliveryResult:
    if quantity <= 0 or quantity > 999_999:
        raise InventoryEditError("Quantity must be between 1 and 999999")
    if container not in {"auto", "main", "key"}:
        raise InventoryEditError(f"Unsupported inventory container: {container}")

    selected_container = container
    if selected_container == "auto":
        selected_container = "key" if definition.is_essential else "main"
    elif selected_container == "key" and not definition.is_essential:
        raise InventoryEditError(
            f"Non-essential item {definition.item_id} cannot be delivered to key inventory"
        )
    elif selected_container == "main" and definition.is_essential:
        raise InventoryEditError(
            f"Essential item {definition.item_id} must be delivered to key inventory"
        )

    world = _world(level_gvas)
    container_id = _container_id(player_gvas, selected_container)
    container_data = _find_container(world, container_id)
    _validate_container(container_data)
    _validate_dynamic_references(world)

    slots = _slot_values(container_data)
    slot_count = _slot_count(container_data)
    before = _inventory_total(slots, definition.item_id)
    existing_indexes = {int(_raw_slot(slot)["slot_index"]) for slot in slots}
    free_indexes = [index for index in range(slot_count) if index not in existing_indexes]

    remaining = quantity
    merge_plan: list[tuple[dict[str, Any], int]] = []
    if not definition.needs_dynamic_data:
        for slot in slots:
            raw = _raw_slot(slot)
            if (
                str(raw.get("item", {}).get("static_id", "")).lower()
                != definition.item_id.lower()
            ):
                continue
            dynamic_id = str(
                raw.get("item", {})
                .get("dynamic_id", {})
                .get("local_id_in_created_world", ZERO_UUID)
            )
            if dynamic_id.lower() != ZERO_UUID:
                continue
            current = int(raw.get("count", 0))
            added = min(max(0, definition.max_stack - current), remaining)
            if added:
                merge_plan.append((slot, added))
                remaining -= added
            if remaining == 0:
                break

    if definition.needs_dynamic_data:
        new_slot_count = remaining
    else:
        new_slot_count = math.ceil(remaining / definition.max_stack)
    if new_slot_count > len(free_indexes):
        raise InventoryEditError(
            f"Inventory needs {new_slot_count} empty slots but only {len(free_indexes)} are available"
        )

    modified_slots: list[int] = []
    dynamic_ids: dict[int, str] = {}
    for slot, added in merge_plan:
        raw = _raw_slot(slot)
        raw["count"] = int(raw["count"]) + added
        modified_slots.append(int(raw["slot_index"]))

    template = slots[0] if slots else _find_slot_template(world)
    dynamic_values = _dynamic_values(world)
    for slot_index in free_indexes[:new_slot_count]:
        count = 1 if definition.needs_dynamic_data else min(
            definition.max_stack, remaining
        )
        dynamic_id = ZERO_UUID
        if definition.needs_dynamic_data:
            dynamic_id = str(uuid.uuid4())
            dynamic_values.append(_new_dynamic_item(world, definition, dynamic_id))
            dynamic_ids[slot_index] = dynamic_id
        slots.append(
            _new_slot(
                template,
                slot_index,
                definition.item_id,
                count,
                dynamic_id,
            )
        )
        remaining -= count
        modified_slots.append(slot_index)

    if remaining != 0:
        raise InventoryEditError(
            f"Internal inventory plan left {remaining} items undelivered"
        )

    slots.sort(key=lambda slot: int(_raw_slot(slot)["slot_index"]))
    _validate_container(container_data)
    _validate_dynamic_references(world)
    after = _inventory_total(slots, definition.item_id)
    if after - before != quantity:
        raise InventoryEditError(
            f"Inventory verification expected +{quantity}, observed +{after - before}"
        )

    return DeliveryResult(
        player_uid=player_uid,
        item_id=definition.item_id,
        container=selected_container,
        requested=quantity,
        delivered=quantity,
        before=before,
        after=after,
        modified_slots=tuple(sorted(set(modified_slots))),
        dynamic_ids=dynamic_ids,
    )


def set_item_quantity(
    level_gvas: Any,
    player_gvas: Any,
    player_uid: str,
    catalog: dict[str, ItemDefinition],
    container: str,
    slot_index: int,
    quantity: int,
    expected_item_id: str,
    expected_quantity: int,
    expected_dynamic_id: str = ZERO_UUID,
) -> InventoryMutationResult:
    if container not in CONTAINER_PROPERTIES:
        raise InventoryEditError(f"Unsupported inventory container: {container}")
    if slot_index < 0:
        raise InventoryEditError("Slot index cannot be negative")
    if quantity < 0 or quantity > 999_999:
        raise InventoryEditError("Quantity must be between 0 and 999999")
    if expected_quantity < 1:
        raise InventoryEditError("Expected quantity must be at least 1")

    world = _world(level_gvas)
    container_id = _container_id(player_gvas, container)
    container_data = _find_container(world, container_id)
    _validate_container(container_data)
    _validate_dynamic_references(world)

    slots = _slot_values(container_data)
    slot = next(
        (
            candidate
            for candidate in slots
            if int(_raw_slot(candidate).get("slot_index", -1)) == slot_index
        ),
        None,
    )
    if slot is None:
        raise InventoryConflictError(
            f"Inventory slot {slot_index} no longer contains an item"
        )

    raw = _raw_slot(slot)
    item_id = str(raw.get("item", {}).get("static_id", ""))
    before = int(raw.get("count", 0))
    if item_id.lower() != expected_item_id.strip().lower():
        raise InventoryConflictError(
            f"Inventory slot {slot_index} changed from {expected_item_id} to {item_id}"
        )
    if before != expected_quantity:
        raise InventoryConflictError(
            f"Inventory slot {slot_index} quantity changed from "
            f"{expected_quantity} to {before}"
        )

    dynamic_id = _slot_dynamic_id(raw)
    normalized_expected_dynamic_id = expected_dynamic_id.strip().lower()
    if not normalized_expected_dynamic_id:
        raise InventoryEditError("Expected dynamic item ID cannot be empty")
    if dynamic_id != normalized_expected_dynamic_id:
        raise InventoryConflictError(
            f"Inventory slot {slot_index} dynamic item changed from "
            f"{expected_dynamic_id} to {dynamic_id}"
        )
    if quantity == before:
        raise InventoryEditError("New quantity is unchanged")

    definition = catalog.get(item_id.lower())
    if quantity > 0:
        if dynamic_id != ZERO_UUID:
            if quantity != 1:
                raise InventoryEditError(
                    "Dynamic inventory items must have a quantity of 1"
                )
        else:
            if definition is None:
                raise InventoryEditError(
                    f"Unknown item metadata for {item_id}; only removal is supported"
                )
            if definition.needs_dynamic_data:
                raise InventoryEditError(
                    f"Dynamic item {item_id} is missing its dynamic record"
                )
            if quantity > definition.max_stack:
                raise InventoryEditError(
                    f"Quantity {quantity} exceeds the maximum stack "
                    f"of {definition.max_stack} for {item_id}"
                )

    dynamic_record_removed = False
    if quantity == 0:
        slots.remove(slot)
        dynamic_record_removed = _remove_dynamic_record_if_unreferenced(
            world, dynamic_id
        )
    else:
        raw["count"] = quantity

    slots.sort(key=lambda candidate: int(_raw_slot(candidate)["slot_index"]))
    _validate_container(container_data)
    _validate_dynamic_references(world)
    return InventoryMutationResult(
        player_uid=player_uid,
        item_id=item_id,
        container=container,
        slot_index=slot_index,
        before=before,
        after=quantity,
        removed=quantity == 0,
        dynamic_record_removed=dynamic_record_removed,
        dynamic_id=dynamic_id,
    )


def verify_delivery(
    level_gvas: Any,
    player_gvas: Any,
    result: DeliveryResult,
) -> None:
    world = _world(level_gvas)
    container_id = _container_id(player_gvas, result.container)
    container_data = _find_container(world, container_id)
    _validate_container(container_data)
    _validate_dynamic_references(world)
    observed = _inventory_total(_slot_values(container_data), result.item_id)
    if observed != result.after:
        raise InventoryEditError(
            f"Rebuilt save contains {observed} {result.item_id}, expected {result.after}"
        )
    slot_dynamic_ids = {
        int(_raw_slot(slot).get("slot_index", -1)): _slot_dynamic_id(_raw_slot(slot))
        for slot in _slot_values(container_data)
    }
    for slot_index, dynamic_id in result.dynamic_ids.items():
        if slot_dynamic_ids.get(slot_index) != dynamic_id.lower():
            raise InventoryEditError(
                f"Rebuilt save did not persist dynamic item {dynamic_id} "
                f"in slot {slot_index}"
            )


def verify_inventory_mutation(
    level_gvas: Any,
    player_gvas: Any,
    result: InventoryMutationResult,
) -> None:
    world = _world(level_gvas)
    container_id = _container_id(player_gvas, result.container)
    container_data = _find_container(world, container_id)
    _validate_container(container_data)
    _validate_dynamic_references(world)

    matching = [
        _raw_slot(slot)
        for slot in _slot_values(container_data)
        if int(_raw_slot(slot).get("slot_index", -1)) == result.slot_index
    ]
    if result.removed:
        if matching:
            raise InventoryEditError(
                f"Rebuilt save still contains inventory slot {result.slot_index}"
            )
    elif len(matching) != 1 or int(matching[0].get("count", 0)) != result.after:
        raise InventoryEditError(
            f"Rebuilt save did not persist quantity {result.after} "
            f"for slot {result.slot_index}"
        )

    if result.dynamic_record_removed and result.dynamic_id != ZERO_UUID:
        remaining_ids = {
            str(
                item.get("RawData", {})
                .get("value", {})
                .get("id", {})
                .get("local_id_in_created_world", "")
            ).lower()
            for item in _dynamic_values(world)
        }
        if result.dynamic_id.lower() in remaining_ids:
            raise InventoryEditError(
                f"Rebuilt save still contains dynamic item {result.dynamic_id}"
            )
