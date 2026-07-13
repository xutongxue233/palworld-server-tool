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
  world_option: WorldOptionOverrideStatus;
}

export interface WorldOptionOverrideStatus {
  supported: boolean;
  present: boolean;
  path?: string;
  size_bytes?: number;
  sha256?: string;
  modified_at?: string;
  message?: string;
}

export interface WorldOptionMutation {
  created: boolean;
  game_version: string;
  updated_keys: string[];
  skipped_keys: string[];
  settings_digest: string;
  sha256: string;
  modified_at: string;
}

export interface WorldOptionSyncResult {
  world_option: WorldOptionMutation;
  safety_backup: Backup;
  maintenance: MaintenanceStopResult;
  restarted: boolean;
  restart_error?: string;
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
  busy: boolean;
}

export interface FleetConfigIssue {
  code: string;
  message: string;
  node_id?: string;
}

export interface FleetNode {
  scope: string;
  id: string;
  name: string;
  management_url?: string;
  local: boolean;
  reachable: boolean;
  selectable: boolean;
  insecure_transport: boolean;
  latency_ms: number;
  protocol_version?: number;
  tool_version?: string;
  server_online: boolean;
  server?: ServerInfo;
  metrics?: ServerMetrics;
  control?: ServerControlStatus;
  server_error?: string;
  error_code?: string;
  error?: string;
  checked_at: string;
}

export interface FleetStatus {
  protocol_version: number;
  local_scope: string;
  nodes: FleetNode[];
  issues?: FleetConfigIssue[];
  checked_at: string;
}

export interface SteamCMDPlan {
  configured: boolean;
  can_execute: boolean;
  app_id: number;
  platform: string;
  executable_path?: string;
  executable_sha256?: string;
  install_dir?: string;
  manifest_path?: string;
  launcher_path?: string;
  installed: boolean;
  partial_installation: boolean;
  build_id?: string;
  existing_worlds: number;
  safety_backup_required: boolean;
  safety_backup_ready: boolean;
  save_path?: string;
  timeout_seconds: number;
  issues?: string[];
  warnings?: string[];
  plan_digest: string;
}

export interface SteamCMDStatus {
  plan: SteamCMDPlan;
  server_control: ServerControlStatus;
}

export interface SteamCMDUpdateRequest {
  expected_plan_digest: string;
  confirm_update: boolean;
  confirm_server_stopped: boolean;
  validate_files: boolean;
  restart_after: boolean;
  shutdown_seconds: number;
  shutdown_message?: string;
}

export interface SteamCMDUpdateExecution {
  before: SteamCMDPlan;
  after: SteamCMDPlan;
  build_id_before?: string;
  build_id_after?: string;
  changed: boolean;
  validated: boolean;
  output_tail?: string;
  started_at: string;
  finished_at: string;
  duration_ms: number;
}

export interface SteamCMDUpdateResult {
  update: SteamCMDUpdateExecution;
  safety_backup?: Backup;
  maintenance: MaintenanceStopResult;
  restarted: boolean;
  restart_error?: string;
}

export interface OfficialModDiagnostic {
  code: string;
  message: string;
  folder_name?: string;
  package_name?: string;
  dependency?: string;
}

export interface OfficialModSettings {
  global_enabled: boolean;
  workshop_root_dir?: string;
  active_mod_list: string[];
}

export interface OfficialModPackage {
  folder_name: string;
  path: string;
  workshop_item_id?: string;
  info_path?: string;
  info_sha256?: string;
  mod_name?: string;
  package_name?: string;
  version?: string;
  author?: string;
  thumbnail?: string;
  min_revision?: number;
  debug_mode: boolean;
  dependencies?: string[];
  tags?: string[];
  install_types?: string[];
  server_install_types?: string[];
  valid: boolean;
  server_compatible: boolean;
  listed: boolean;
  effective_enabled: boolean;
  deployed: boolean;
  pending_restart: boolean;
  pending_removal: boolean;
  issues?: OfficialModDiagnostic[];
  warnings?: OfficialModDiagnostic[];
}

export interface OfficialModInventory {
  workshop_root: string;
  workshop_source: string;
  workshop_available: boolean;
  packages: OfficialModPackage[];
  unknown_active_mods?: string[];
  issues?: OfficialModDiagnostic[];
  warnings?: OfficialModDiagnostic[];
}

export interface OfficialModStatus {
  game_version: string;
  platform: string;
  supported: boolean;
  configured: boolean;
  manageable: boolean;
  install_dir?: string;
  install_dir_source?: string;
  launcher_path?: string;
  settings_path?: string;
  settings_exists: boolean;
  settings_sha256?: string;
  settings: OfficialModSettings;
  forced_disabled: boolean;
  launch_workshop_root?: string;
  inventory: OfficialModInventory;
  existing_worlds: number;
  safety_backup_ready: boolean;
  save_path?: string;
  status_digest: string;
  issues?: OfficialModDiagnostic[];
  warnings?: OfficialModDiagnostic[];
}

export interface OfficialModChangePlan {
  status: OfficialModStatus;
  desired_settings: OfficialModSettings;
  target_inventory: OfficialModInventory;
  changed: boolean;
  changes?: string[];
  safety_backup_required: boolean;
  safety_backup_ready: boolean;
  can_apply: boolean;
  issues?: OfficialModDiagnostic[];
  warnings?: OfficialModDiagnostic[];
  plan_digest: string;
}

export interface OfficialModPreflightResult {
  plan: OfficialModChangePlan;
  server_control: ServerControlStatus;
}

export interface OfficialModApplyRequest extends OfficialModSettings {
  expected_plan_digest: string;
  confirm_apply: boolean;
  confirm_mod_risk: boolean;
  confirm_server_stopped: boolean;
  restart_after: boolean;
  shutdown_seconds: number;
  shutdown_message?: string;
}

export interface OfficialModApplyExecution {
  plan: OfficialModChangePlan;
  status: OfficialModStatus;
  changed: boolean;
  created: boolean;
  recovery_path?: string;
  previous_exists: boolean;
  previous_sha256?: string;
  settings_sha256?: string;
  restart_required: boolean;
  applied_at?: string;
  rolled_back: boolean;
  rollback_at?: string;
}

export interface OfficialModApplyResult {
  apply: OfficialModApplyExecution;
  safety_backup?: Backup;
  maintenance: MaintenanceStopResult;
  restarted: boolean;
  restart_error?: string;
  recovery_restarted: boolean;
  recovery_restart_error?: string;
  rollback_error?: string;
}

export type SaveMigrationPlatform = "current" | "windows" | "linux";
export type SaveMigrationKind = "dedicated" | "coop";

export interface SaveMigrationNotice {
  code: string;
  message: string;
}

export interface SaveMigrationPlan {
  configured: boolean;
  can_migrate: boolean;
  game_version: string;
  destination_platform: string;
  source_input: string;
  source_path?: string;
  source_world_id?: string;
  source_platform: string;
  source_kind: string;
  source_digest?: string;
  source_size_bytes: number;
  source_file_count: number;
  source_player_files: number;
  source_has_world_option: boolean;
  source_has_native_backups: boolean;
  source_ignored_entries?: string[];
  coop_host_detected: boolean;
  validation_passed: boolean;
  destination_path?: string;
  destination_world_id?: string;
  destination_digest?: string;
  destination_size_bytes: number;
  destination_file_count: number;
  destination_player_files: number;
  destination_has_world_option: boolean;
  issues?: SaveMigrationNotice[];
  warnings?: SaveMigrationNotice[];
  plan_digest: string;
}

export interface SaveMigrationPreflightRequest {
  source_path: string;
  source_platform: SaveMigrationPlatform;
  source_kind: SaveMigrationKind;
}

export interface SaveMigrationPreflightResult {
  plan: SaveMigrationPlan;
  server_control: ServerControlStatus;
}

export interface SaveMigrationApplyRequest extends SaveMigrationPreflightRequest {
  expected_plan_digest: string;
  confirm_migration: boolean;
  confirm_server_stopped: boolean;
  restart_after: boolean;
  shutdown_seconds: number;
  shutdown_message?: string;
}

export interface SaveMigrationExecution {
  plan: SaveMigrationPlan;
  safety_backup: Backup;
}

export interface SaveMigrationApplyResult {
  migration: SaveMigrationExecution;
  maintenance: MaintenanceStopResult;
  sync_error?: string;
  restarted: boolean;
  restart_error?: string;
}

export type AutomationAction =
  | "save_world"
  | "broadcast"
  | "start_server"
  | "stop_server"
  | "restart_server"
  | "sync_save"
  | "pst_backup";

export type AutomationScheduleKind = "interval" | "daily" | "weekly";

export interface AutomationTaskSchedule {
  kind: AutomationScheduleKind;
  interval_minutes?: number;
  time_of_day?: string;
  weekdays?: number[];
}

export interface AutomationActionParameters {
  message?: string;
  delay_seconds?: number;
}

export interface ScheduledTaskInput {
  name: string;
  enabled: boolean;
  action: AutomationAction;
  schedule: AutomationTaskSchedule;
  parameters: AutomationActionParameters;
}

export type TaskRunStatus = "running" | "succeeded" | "failed" | "skipped";
export type TaskRunTrigger = "scheduled" | "manual";

export interface TaskRun {
  id: string;
  task_id: string;
  task_name: string;
  action: AutomationAction;
  trigger: TaskRunTrigger;
  status: TaskRunStatus;
  started_at: string;
  finished_at?: string;
  summary?: string;
  error?: string;
}

export interface ScheduledTask extends ScheduledTaskInput {
  id: string;
  created_at: string;
  updated_at: string;
  next_run_at?: string;
  running: boolean;
  last_run?: TaskRun;
}

export interface WatchdogSettings {
  enabled: boolean;
  desired_running: boolean;
  check_interval_seconds: number;
  failure_threshold: number;
  restart_cooldown_seconds: number;
  max_recovery_attempts: number;
  startup_grace_seconds: number;
}

export type AutomationNotificationProvider = "generic" | "discord";

export type AutomationNotificationEvent =
  | "task.succeeded"
  | "task.failed"
  | "server.started"
  | "server.stopped"
  | "server.restarted"
  | "watchdog.unhealthy"
  | "watchdog.recovered"
  | "watchdog.recovery_failed";

export interface AutomationNotificationSettings {
  enabled: boolean;
  provider: AutomationNotificationProvider;
  webhook_configured: boolean;
  webhook_preview?: string;
  secret_configured: boolean;
  events: AutomationNotificationEvent[];
  timeout_seconds: number;
}

export interface AutomationSettings {
  watchdog: WatchdogSettings;
  notification: AutomationNotificationSettings;
}

export interface AutomationNotificationSettingsUpdate {
  enabled: boolean;
  provider: AutomationNotificationProvider;
  webhook_url?: string;
  clear_webhook?: boolean;
  secret?: string;
  clear_secret?: boolean;
  events: AutomationNotificationEvent[];
  timeout_seconds: number;
}

export interface AutomationSettingsUpdate {
  watchdog: WatchdogSettings;
  notification: AutomationNotificationSettingsUpdate;
}

export interface WatchdogRuntimeStatus {
  enabled: boolean;
  desired_running: boolean;
  state: string;
  consecutive_failures: number;
  recovery_attempts: number;
  last_check_at?: string;
  last_healthy_at?: string;
  last_recovery_at?: string;
  next_check_at?: string;
  last_error?: string;
}

export interface AutomationNotificationRuntimeStatus {
  configured: boolean;
  enabled: boolean;
  provider?: string;
  webhook_preview?: string;
  secret_configured: boolean;
  last_attempt_at?: string;
  last_success_at?: string;
  last_error?: string;
}

export interface AutomationStatus {
  location: string;
  busy: boolean;
  active_task_id?: string;
  watchdog: WatchdogRuntimeStatus;
  notification: AutomationNotificationRuntimeStatus;
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

export interface NativeBackup {
  backup_id: string;
  created_at: string;
  modified_at: string;
  size_bytes: number;
  file_count: number;
  player_files: number;
  has_world_option: boolean;
  digest: string;
  valid: boolean;
  issues?: string[];
}

export interface NativeBackupCatalog {
  configured: boolean;
  available: boolean;
  world_id?: string;
  backups: NativeBackup[];
}

export interface NativeBackupListResult {
  native_backups: NativeBackupCatalog;
  server_control: ServerControlStatus;
}

export interface MaintenanceStopResult {
  was_running: boolean;
  can_restart: boolean;
}

export interface NativeBackupRestoreResult {
  restored_backup: NativeBackup;
  safety_backup: Backup;
  maintenance: MaintenanceStopResult;
  sync_error?: string;
  restarted: boolean;
  restart_error?: string;
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
