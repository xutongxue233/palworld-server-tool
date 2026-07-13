export type Locale = "zh" | "en" | "ja";

export interface ServerInfo {
  version: string;
  name: string;
  description: string;
  world_guid: string;
}

export interface ServerMetrics {
  server_fps: number;
  current_player_num: number;
  server_frame_time: number;
  max_player_num: number;
  uptime: number;
  base_camp_num: number;
  days: number;
}

export interface ServerToolInfo {
  version: string;
  latest: string;
}

export interface GameConfigFile {
  configured: boolean;
  path?: string;
  content?: string;
  sha256?: string;
  modified_at?: string;
}

export interface GameConfigWriteResult {
  sha256: string;
  backup_path: string;
  modified_at: string;
  restart_required: boolean;
}

export interface ServerControlStatus {
  configured: boolean;
  mode: string;
  target?: string;
  online: boolean;
  running: boolean;
  state: string;
  detail?: string;
}

export interface Pal {
  instance_id?: string;
  level: number;
  exp: number;
  hp: number;
  max_hp: number;
  type: string;
  gender: string;
  nickname: string;
  is_lucky: boolean;
  is_boss: boolean;
  is_tower: boolean;
  workspeed: number;
  melee: number;
  ranged: number;
  defense: number;
  rank: number;
  rank_attack: number;
  rank_defence: number;
  rank_craftspeed: number;
  skills: string[];
}

export interface PlayerItem {
  SlotIndex: number;
  ItemId: string;
  StackCount: number;
  DynamicId: string;
}

export interface PlayerItems {
  CommonContainerId?: PlayerItem[];
  DropSlotContainerId?: PlayerItem[];
  EssentialContainerId?: PlayerItem[];
  FoodEquipContainerId?: PlayerItem[];
  PlayerEquipArmorContainerId?: PlayerItem[];
  WeaponLoadOutContainerId?: PlayerItem[];
  [key: string]: PlayerItem[] | undefined;
}

export interface PlayerSummary {
  player_uid: string;
  user_id: string;
  steam_id: string;
  nickname: string;
  account_name: string;
  ip: string;
  ping: number;
  location_x: number;
  location_y: number;
  level: number;
  building_count: number;
  last_online: string;
  exp?: number;
  hp?: number;
  max_hp?: number;
  shield_hp?: number;
  shield_max_hp?: number;
  max_status_point?: number;
  unused_status_points?: number;
  technology_points?: number;
  ancient_technology_points?: number;
  status_point?: Record<string, number>;
  full_stomach?: number;
  save_last_online?: string;
}

export interface PlayerMapProgress {
  fast_travel_unlocked: number;
  fast_travel_total: number;
  areas_found: number;
  areas_total: number;
  world_maps_unlocked: number;
  world_maps_total: number;
  progress_digest: string;
  game_version: string;
}

export interface Player extends PlayerSummary {
  pals: Pal[];
  items: PlayerItems | null;
  map_progress?: PlayerMapProgress;
}

export interface GuildPlayer {
  player_uid: string;
  nickname: string;
}

export interface BaseCamp {
  id: string;
  area: number;
  location_x: number;
  location_y: number;
}

export interface Guild {
  name: string;
  base_camp_level: number;
  admin_player_uid: string;
  players: GuildPlayer[];
  base_camp: BaseCamp[];
}

export interface WhitelistPlayer {
  name: string;
  user_id: string;
  steam_id: string;
  player_uid: string;
}

export interface Backup {
  backup_id: string;
  save_time: string;
  path: string;
}

export type InventoryContainer =
  "main" | "key" | "weapons" | "armor" | "food" | "drop";

export interface ItemDelivery {
  player_uid: string;
  item_id: string;
  container: "main" | "key";
  requested: number;
  delivered: number;
  before: number;
  after: number;
  modified_slots: number[];
  dynamic_ids: Record<string, string>;
}

export interface GiveItemResult {
  delivery: ItemDelivery;
  backup: Backup;
  sync_error?: string;
}

export interface InventoryMutation {
  player_uid: string;
  item_id: string;
  container: InventoryContainer;
  slot_index: number;
  before: number;
  after: number;
  removed: boolean;
  dynamic_record_removed: boolean;
  dynamic_id: string;
}

export interface SetItemQuantityResult {
  mutation: InventoryMutation;
  backup: Backup;
  sync_error?: string;
}

export interface PlayerProfileMutation {
  player_uid: string;
  nickname_before: string;
  nickname_after: string;
  level_before: number;
  level_after: number;
  exp_before: number;
  exp_after: number;
  character_records: number;
  guild_records: number;
}

export interface EditPlayerProfileResult {
  profile: PlayerProfileMutation;
  backup: Backup;
  sync_error?: string;
}

export interface PlayerStatPointsMutation {
  player_uid: string;
  before: number;
  after: number;
  character_records: number;
}

export interface EditPlayerStatPointsResult {
  stat_points: PlayerStatPointsMutation;
  backup: Backup;
  sync_error?: string;
}

export interface PlayerTechnologyPointsMutation {
  player_uid: string;
  technology_before: number;
  technology_after: number;
  ancient_before: number;
  ancient_after: number;
  created_fields: string[];
}

export interface EditPlayerTechnologyPointsResult {
  technology_points: PlayerTechnologyPointsMutation;
  backup: Backup;
  sync_error?: string;
}

export interface PalNicknameMutation {
  player_uid: string;
  instance_id: string;
  pal_type: string;
  nickname_before: string;
  nickname_after: string;
  level: number;
  exp: number;
  nickname_created: boolean;
}

export interface RenamePalResult {
  nickname: PalNicknameMutation;
  backup: Backup;
  sync_error?: string;
}

export interface PalLevelMutation {
  player_uid: string;
  instance_id: string;
  pal_type: string;
  nickname: string;
  level_before: number;
  level_after: number;
  exp_before: number;
  exp_after: number;
  hp_before: number;
  hp_after: number;
  max_hp_before: number;
  max_hp_after: number;
  health_field: string;
  max_hp_created: boolean;
}

export interface EditPalLevelResult {
  level: PalLevelMutation;
  backup: Backup;
  sync_error?: string;
}

export interface PalHealthMutation {
  player_uid: string;
  instance_id: string;
  pal_type: string;
  nickname: string;
  level: number;
  exp: number;
  hp_before: number;
  hp_after: number;
  max_hp: number;
  health_field: string;
}

export interface EditPalHealthResult {
  health: PalHealthMutation;
  backup: Backup;
  sync_error?: string;
}

export interface PlayerMapProgressMutation {
  player_uid: string;
  fast_travel_before: number;
  fast_travel_after: number;
  fast_travel_total: number;
  areas_before: number;
  areas_after: number;
  areas_total: number;
  world_maps_before: number;
  world_maps_after: number;
  world_maps_total: number;
  created_fields: string[];
  progress_digest_before: string;
  progress_digest_after: string;
  game_version: string;
}

export interface UnlockPlayerMapProgressResult {
  map_progress: PlayerMapProgressMutation;
  backup: Backup;
  sync_error?: string;
}

export interface WorldActor {
  Type?: string;
  NickName?: string;
  TrainerNickName?: string;
  GuildName?: string;
  Class?: string;
  level?: number;
  Action?: string;
  AI_Action?: string;
  LocationX?: number;
  LocationY?: number;
  LocationZ?: number;
  [key: string]: unknown;
}

export interface WorldSnapshot {
  Available?: boolean;
  Message?: string;
  Time?: string;
  FPS?: number;
  AverageFPS?: number;
  ActorData?: WorldActor[];
  [key: string]: unknown;
}

export interface ApiSuccess {
  success: boolean;
}
