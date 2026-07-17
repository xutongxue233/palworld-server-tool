import type {
  ApiSuccess,
  AuthStatus,
  AuthToken,
  AutomationSettings,
  AutomationSettingsUpdate,
  AutomationStatus,
  Backup,
  EditPalHealthResult,
  EditPalLevelResult,
  EditPlayerProfileResult,
  EditPlayerStatPointsResult,
  EditPlayerTechnologyPointsResult,
  FleetStatus,
  Guild,
  GameConfigFile,
  GameConfigWriteResult,
  GiveItemResult,
  InventoryContainer,
  NativeBackupListResult,
  NativeBackupRestoreResult,
  OfficialModApplyRequest,
  OfficialModApplyResult,
  OfficialModPreflightResult,
  OfficialModSettings,
  OfficialModStatus,
  Player,
  PlayerSummary,
  RenamePalResult,
  DiscoverySetupStatus,
  SaveMigrationApplyRequest,
  SaveMigrationApplyResult,
  SaveMigrationPreflightRequest,
  SaveMigrationPreflightResult,
  SetItemQuantityResult,
  ServerInfo,
  ServerDiscoveryApplyRequest,
  ServerDiscoveryStatus,
  ServerMetrics,
  ServerToolInfo,
  ServerControlStatus,
  SteamCMDStatus,
  SteamCMDUpdateRequest,
  SteamCMDUpdateResult,
  ScheduledTask,
  ScheduledTaskInput,
  TaskRun,
  UnlockPlayerMapProgressResult,
  WhitelistPlayer,
  WorldSnapshot,
  WorldOptionSyncResult,
} from "@/types/api";

export const TOKEN_KEY = "palworld_token";
export const LOCAL_SERVER_SCOPE = "local";

let currentServerScope = LOCAL_SERVER_SCOPE;

export function getServerScope() {
  return currentServerScope;
}

export function setServerScope(scope: string) {
  currentServerScope = normalizeServerScope(scope);
}

function normalizeServerScope(scope: string) {
  return /^[a-z0-9](?:[a-z0-9_-]{0,46}[a-z0-9])?$/.test(scope)
    ? scope
    : LOCAL_SERVER_SCOPE;
}

export class ApiError extends Error {
  status: number;
  payload: unknown;

  constructor(message: string, status: number, payload: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.payload = payload;
  }
}

export function getApiErrorCode(error: unknown) {
  if (!(error instanceof ApiError)) return "";
  const payload = error.payload;
  return typeof payload === "object" && payload && "code" in payload
    ? String((payload as { code: unknown }).code)
    : "";
}

interface RequestOptions extends Omit<RequestInit, "body"> {
  body?: unknown;
  auth?: boolean;
  responseType?: "json" | "blob" | "text";
  scope?: "current" | "local";
  serverScope?: string;
}

function buildQuery(values: Record<string, unknown> = {}) {
  const params = new URLSearchParams();
  Object.entries(values).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== "") {
      params.set(key, String(value));
    }
  });
  const query = params.toString();
  return query ? `?${query}` : "";
}

async function request<T>(path: string, options: RequestOptions = {}) {
  const {
    auth = true,
    responseType = "json",
    scope: requestScope = "current",
    serverScope,
    body,
    headers: suppliedHeaders,
    ...requestOptions
  } = options;
  const headers = new Headers(suppliedHeaders);
  const token = localStorage.getItem(TOKEN_KEY);

  const scope =
    requestScope === "local"
      ? LOCAL_SERVER_SCOPE
      : normalizeServerScope(serverScope ?? currentServerScope);
  const remote = scope !== LOCAL_SERVER_SCOPE;
  const requestPath = remote ? fleetProxyPath(scope, path) : path;
  const requiresCentralAuth = auth || remote;

  if (requiresCentralAuth && token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  let requestBody: BodyInit | undefined;
  if (body instanceof FormData || body instanceof Blob) {
    requestBody = body;
  } else if (body !== undefined) {
    headers.set("Content-Type", "application/json");
    requestBody = JSON.stringify(body);
  }

  const response = await fetch(requestPath, {
    ...requestOptions,
    headers,
    body: requestBody,
  });

  if (response.status === 401 && requiresCentralAuth) {
    localStorage.removeItem(TOKEN_KEY);
    window.dispatchEvent(new CustomEvent("palworld:auth-expired"));
  }

  let payload: unknown = null;
  if (response.status !== 204) {
    if (responseType === "blob") {
      payload = await response.blob();
    } else if (responseType === "text") {
      payload = await response.text();
    } else {
      const contentType = response.headers.get("content-type") ?? "";
      payload = contentType.includes("application/json")
        ? await response.json()
        : await response.text();
    }
  }

  if (!response.ok) {
    const message =
      typeof payload === "object" && payload && "error" in payload
        ? String((payload as { error: unknown }).error)
        : response.statusText || "Request failed";
    throw new ApiError(message, response.status, payload);
  }

  return payload as T;
}

function fleetProxyPath(scope: string, path: string) {
  const queryIndex = path.indexOf("?");
  const pathname = queryIndex >= 0 ? path.slice(0, queryIndex) : path;
  const query = queryIndex >= 0 ? path.slice(queryIndex) : "";
  if (!pathname.startsWith("/api/")) {
    throw new Error("Fleet proxy only accepts /api paths");
  }
  return `/api/fleet/nodes/${encodeURIComponent(scope)}/proxy${pathname.slice(4)}${query}`;
}

export const api = {
  getAuthStatus: () =>
    request<AuthStatus>("/api/auth/status", {
      auth: false,
      scope: "local",
    }),
  initializePassword: (password: string, passwordConfirmation: string) =>
    request<AuthToken>("/api/auth/password", {
      method: "POST",
      body: {
        password,
        password_confirmation: passwordConfirmation,
      },
      auth: false,
      scope: "local",
    }),
  changePassword: (password: string, passwordConfirmation: string) =>
    request<AuthToken>("/api/auth/password", {
      method: "PUT",
      body: {
        password,
        password_confirmation: passwordConfirmation,
      },
      scope: "local",
    }),
  login: (password: string) =>
    request<AuthToken>("/api/login", {
      method: "POST",
      body: { password },
      auth: false,
      scope: "local",
    }),
  getFleetNodes: () =>
    request<FleetStatus>("/api/fleet/nodes", { scope: "local" }),
  getDiscoverySetupStatus: () =>
    request<DiscoverySetupStatus>("/api/setup/status", {
      auth: false,
      scope: "local",
    }),
  getServerDiscovery: () =>
    request<ServerDiscoveryStatus>("/api/setup/discovery", {
      scope: "local",
    }),
  scanServerDiscovery: () =>
    request<ServerDiscoveryStatus>("/api/setup/discovery/scan", {
      method: "POST",
      scope: "local",
    }),
  applyServerDiscovery: (update: ServerDiscoveryApplyRequest) =>
    request<ServerDiscoveryStatus>("/api/setup/discovery/apply", {
      method: "POST",
      body: update,
      scope: "local",
    }),
  getServer: (serverScope?: string) =>
    request<ServerInfo>("/api/server", { auth: false, serverScope }),
  getServerTool: (serverScope?: string) =>
    request<ServerToolInfo>("/api/server/tool", {
      auth: false,
      serverScope,
    }),
  getMetrics: (serverScope?: string) =>
    request<ServerMetrics>("/api/server/metrics", {
      auth: false,
      serverScope,
    }),
  getSettings: (serverScope?: string) =>
    request<Record<string, unknown>>("/api/server/settings", { serverScope }),
  getWorldSnapshot: (serverScope?: string) =>
    request<WorldSnapshot>("/api/server/game-data", { serverScope }),
  getGameConfigFile: (serverScope?: string) =>
    request<GameConfigFile>("/api/server/config-file", { serverScope }),
  putGameConfigFile: (content: string, expectedSha256: string) =>
    request<GameConfigWriteResult>("/api/server/config-file", {
      method: "PUT",
      body: { content, expected_sha256: expectedSha256 },
    }),
  syncWorldOption: (content: string, expectedSha256 = "") =>
    request<WorldOptionSyncResult>("/api/server/world-option", {
      method: "PUT",
      body: {
        content,
        expected_sha256: expectedSha256,
        confirm_sync: true,
        shutdown_seconds: 10,
      },
    }),
  getServerControlStatus: (serverScope?: string) =>
    request<ServerControlStatus>("/api/server/control/status", {
      serverScope,
    }),
  getSteamCMDStatus: (serverScope?: string) =>
    request<SteamCMDStatus>("/api/server/steamcmd", { serverScope }),
  updateServerWithSteamCMD: (update: SteamCMDUpdateRequest) =>
    request<SteamCMDUpdateResult>("/api/server/steamcmd/update", {
      method: "POST",
      body: update,
    }),
  getOfficialMods: (serverScope?: string) =>
    request<OfficialModStatus>("/api/server/mods", { serverScope }),
  preflightOfficialMods: (settings: OfficialModSettings) =>
    request<OfficialModPreflightResult>("/api/server/mods/preflight", {
      method: "POST",
      body: settings,
    }),
  applyOfficialMods: (settings: OfficialModApplyRequest) =>
    request<OfficialModApplyResult>("/api/server/mods/apply", {
      method: "POST",
      body: settings,
    }),
  preflightSaveMigration: (source: SaveMigrationPreflightRequest) =>
    request<SaveMigrationPreflightResult>("/api/server/migration/preflight", {
      method: "POST",
      body: source,
    }),
  applySaveMigration: (migration: SaveMigrationApplyRequest) =>
    request<SaveMigrationApplyResult>("/api/server/migration/apply", {
      method: "POST",
      body: migration,
    }),
  runRcon: (command: string) =>
    request<{ message: string }>("/api/rcon", {
      method: "POST",
      body: { command },
    }),
  broadcast: (message: string) =>
    request<ApiSuccess>("/api/server/broadcast", {
      method: "POST",
      body: { message },
    }),
  saveWorld: () => request<ApiSuccess>("/api/server/save", { method: "POST" }),
  shutdown: (seconds: number, message: string) =>
    request<ApiSuccess>("/api/server/shutdown", {
      method: "POST",
      body: { seconds, message },
    }),
  stopServer: () => request<ApiSuccess>("/api/server/stop", { method: "POST" }),
  startServer: () =>
    request<ApiSuccess>("/api/server/start", { method: "POST" }),
  restartServer: (seconds: number, message: string) =>
    request<ApiSuccess>("/api/server/restart", {
      method: "POST",
      body: { seconds, message },
    }),
  getAutomationTasks: (serverScope?: string) =>
    request<ScheduledTask[]>("/api/automation/tasks", { serverScope }),
  createAutomationTask: (task: ScheduledTaskInput) =>
    request<ScheduledTask>("/api/automation/tasks", {
      method: "POST",
      body: task,
    }),
  updateAutomationTask: (taskId: string, task: ScheduledTaskInput) =>
    request<ScheduledTask>(
      `/api/automation/tasks/${encodeURIComponent(taskId)}`,
      {
        method: "PUT",
        body: task,
      },
    ),
  deleteAutomationTask: (taskId: string) =>
    request<ApiSuccess>(`/api/automation/tasks/${encodeURIComponent(taskId)}`, {
      method: "DELETE",
    }),
  runAutomationTask: (taskId: string) =>
    request<TaskRun>(
      `/api/automation/tasks/${encodeURIComponent(taskId)}/run`,
      { method: "POST" },
    ),
  getAutomationRuns: (taskId = "", limit = 50, serverScope?: string) =>
    request<TaskRun[]>(
      `/api/automation/runs${buildQuery({ task_id: taskId, limit })}`,
      { serverScope },
    ),
  getAutomationSettings: (serverScope?: string) =>
    request<AutomationSettings>("/api/automation/settings", { serverScope }),
  updateAutomationSettings: (settings: AutomationSettingsUpdate) =>
    request<AutomationSettings>("/api/automation/settings", {
      method: "PUT",
      body: settings,
    }),
  getAutomationStatus: (serverScope?: string) =>
    request<AutomationStatus>("/api/automation/status", { serverScope }),
  testAutomationNotification: () =>
    request<ApiSuccess>("/api/automation/notifications/test", {
      method: "POST",
    }),
  resetAutomationWatchdog: () =>
    request<ApiSuccess>("/api/automation/watchdog/reset", {
      method: "POST",
    }),
  getPlayers: (params: Record<string, unknown> = {}, serverScope?: string) =>
    request<PlayerSummary[]>(`/api/player${buildQuery(params)}`, {
      serverScope,
    }),
  getOnlinePlayers: (serverScope?: string) =>
    request<PlayerSummary[]>("/api/online_player", { serverScope }),
  getPlayer: (playerUid: string, serverScope?: string) =>
    request<Player>(`/api/player/${encodeURIComponent(playerUid)}`, {
      serverScope,
    }),
  playerAction: (
    playerUid: string,
    action: "kick" | "ban" | "unban",
    message = "",
  ) =>
    request<ApiSuccess>(
      `/api/player/${encodeURIComponent(playerUid)}/${action}`,
      {
        method: "POST",
        body: action === "unban" ? undefined : { message },
      },
    ),
  givePlayerItem: (playerUid: string, itemId: string, quantity: number) =>
    request<GiveItemResult>(
      `/api/player/${encodeURIComponent(playerUid)}/items`,
      {
        method: "POST",
        body: {
          item_id: itemId,
          quantity,
          container: "auto",
          confirm_server_stopped: true,
        },
      },
    ),
  setPlayerItemQuantity: (
    playerUid: string,
    container: InventoryContainer,
    slotIndex: number,
    itemId: string,
    expectedQuantity: number,
    expectedDynamicId: string,
    quantity: number,
  ) =>
    request<SetItemQuantityResult>(
      `/api/player/${encodeURIComponent(playerUid)}/items/${container}/${slotIndex}`,
      {
        method: "PATCH",
        body: {
          item_id: itemId,
          expected_quantity: expectedQuantity,
          expected_dynamic_id: expectedDynamicId,
          quantity,
          confirm_server_stopped: true,
        },
      },
    ),
  editPlayerProfile: (
    playerUid: string,
    nickname: string,
    level: number,
    expectedNickname: string,
    expectedLevel: number,
  ) =>
    request<EditPlayerProfileResult>(
      `/api/player/${encodeURIComponent(playerUid)}/profile`,
      {
        method: "PATCH",
        body: {
          nickname,
          level,
          expected_nickname: expectedNickname,
          expected_level: expectedLevel,
          confirm_server_stopped: true,
        },
      },
    ),
  editPlayerStatPoints: (
    playerUid: string,
    unusedStatusPoints: number,
    expectedUnusedStatusPoints: number,
  ) =>
    request<EditPlayerStatPointsResult>(
      `/api/player/${encodeURIComponent(playerUid)}/stat-points`,
      {
        method: "PATCH",
        body: {
          unused_status_points: unusedStatusPoints,
          expected_unused_status_points: expectedUnusedStatusPoints,
          confirm_server_stopped: true,
        },
      },
    ),
  editPlayerTechnologyPoints: (
    playerUid: string,
    technologyPoints: number,
    ancientTechnologyPoints: number,
    expectedTechnologyPoints: number,
    expectedAncientTechnologyPoints: number,
  ) =>
    request<EditPlayerTechnologyPointsResult>(
      `/api/player/${encodeURIComponent(playerUid)}/technology-points`,
      {
        method: "PATCH",
        body: {
          technology_points: technologyPoints,
          ancient_technology_points: ancientTechnologyPoints,
          expected_technology_points: expectedTechnologyPoints,
          expected_ancient_technology_points: expectedAncientTechnologyPoints,
          confirm_server_stopped: true,
        },
      },
    ),
  renamePal: (
    playerUid: string,
    instanceId: string,
    nickname: string,
    expectedNickname: string,
    expectedLevel: number,
    expectedExp: number,
  ) =>
    request<RenamePalResult>(
      `/api/player/${encodeURIComponent(playerUid)}/pals/${encodeURIComponent(instanceId)}/nickname`,
      {
        method: "PATCH",
        body: {
          nickname,
          expected_nickname: expectedNickname,
          expected_level: expectedLevel,
          expected_exp: expectedExp,
          confirm_server_stopped: true,
        },
      },
    ),
  editPalLevel: (
    playerUid: string,
    instanceId: string,
    level: number,
    expectedNickname: string,
    expectedLevel: number,
    expectedExp: number,
    expectedHp: number,
    expectedMaxHp: number,
  ) =>
    request<EditPalLevelResult>(
      `/api/player/${encodeURIComponent(playerUid)}/pals/${encodeURIComponent(instanceId)}/level`,
      {
        method: "PATCH",
        body: {
          expected_nickname: expectedNickname,
          expected_level: expectedLevel,
          expected_exp: expectedExp,
          expected_hp: expectedHp,
          expected_max_hp: expectedMaxHp,
          level,
          confirm_server_stopped: true,
        },
      },
    ),
  restorePalHealth: (
    playerUid: string,
    instanceId: string,
    expectedNickname: string,
    expectedLevel: number,
    expectedExp: number,
    expectedHp: number,
    expectedMaxHp: number,
  ) =>
    request<EditPalHealthResult>(
      `/api/player/${encodeURIComponent(playerUid)}/pals/${encodeURIComponent(instanceId)}/health`,
      {
        method: "PATCH",
        body: {
          expected_nickname: expectedNickname,
          expected_level: expectedLevel,
          expected_exp: expectedExp,
          expected_hp: expectedHp,
          expected_max_hp: expectedMaxHp,
          confirm_server_stopped: true,
        },
      },
    ),
  unlockPlayerMapProgress: (
    playerUid: string,
    expectedProgressDigest: string,
  ) =>
    request<UnlockPlayerMapProgressResult>(
      `/api/player/${encodeURIComponent(playerUid)}/map-progress`,
      {
        method: "PATCH",
        body: {
          expected_progress_digest: expectedProgressDigest,
          confirm_server_stopped: true,
        },
      },
    ),
  getGuilds: (serverScope?: string) =>
    request<Guild[]>("/api/guild", { auth: false, serverScope }),
  getGuild: (adminPlayerUid: string, serverScope?: string) =>
    request<Guild>(`/api/guild/${encodeURIComponent(adminPlayerUid)}`, {
      auth: false,
      serverScope,
    }),
  getWhitelist: (serverScope?: string) =>
    request<WhitelistPlayer[]>("/api/whitelist", { serverScope }),
  addWhitelist: (player: WhitelistPlayer) =>
    request<ApiSuccess>("/api/whitelist", {
      method: "POST",
      body: player,
    }),
  removeWhitelist: (player: WhitelistPlayer) =>
    request<ApiSuccess>("/api/whitelist", {
      method: "DELETE",
      body: player,
    }),
  replaceWhitelist: (players: WhitelistPlayer[]) =>
    request<ApiSuccess>("/api/whitelist", {
      method: "PUT",
      body: players,
    }),
  getBackups: (startTime?: number, endTime?: number, serverScope?: string) =>
    request<Backup[]>(`/api/backup${buildQuery({ startTime, endTime })}`, {
      serverScope,
    }),
  getNativeBackups: (serverScope?: string) =>
    request<NativeBackupListResult>("/api/server/backups/native", {
      serverScope,
    }),
  restoreNativeBackup: (
    backupId: string,
    expectedDigest: string,
    restartAfter: boolean,
  ) =>
    request<NativeBackupRestoreResult>(
      `/api/server/backups/native/${encodeURIComponent(backupId)}/restore`,
      {
        method: "POST",
        body: {
          expected_digest: expectedDigest,
          confirm_restore: true,
          restart_after: restartAfter,
          shutdown_seconds: 10,
        },
      },
    ),
  downloadBackup: (backupId: string) =>
    request<Blob>(`/api/backup/${encodeURIComponent(backupId)}`, {
      responseType: "blob",
    }),
  removeBackup: (backupId: string) =>
    request<ApiSuccess>(`/api/backup/${encodeURIComponent(backupId)}`, {
      method: "DELETE",
    }),
};

export function getApiErrorMessage(error: unknown) {
  if (error instanceof ApiError || error instanceof Error) {
    return error.message;
  }
  return String(error);
}
