import type {
  ApiSuccess,
  Backup,
  EditPalHealthResult,
  EditPalLevelResult,
  EditPlayerProfileResult,
  EditPlayerStatPointsResult,
  EditPlayerTechnologyPointsResult,
  Guild,
  GiveItemResult,
  InventoryContainer,
  Player,
  PlayerSummary,
  RenamePalResult,
  SetItemQuantityResult,
  ServerInfo,
  ServerMetrics,
  ServerToolInfo,
  UnlockPlayerMapProgressResult,
  WhitelistPlayer,
  WorldSnapshot,
} from "@/types/api";

export const TOKEN_KEY = "palworld_token";

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
    body,
    headers: suppliedHeaders,
    ...requestOptions
  } = options;
  const headers = new Headers(suppliedHeaders);
  const token = localStorage.getItem(TOKEN_KEY);

  if (auth && token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  let requestBody: BodyInit | undefined;
  if (body instanceof FormData || body instanceof Blob) {
    requestBody = body;
  } else if (body !== undefined) {
    headers.set("Content-Type", "application/json");
    requestBody = JSON.stringify(body);
  }

  const response = await fetch(path, {
    ...requestOptions,
    headers,
    body: requestBody,
  });

  if (response.status === 401 && auth) {
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

export const api = {
  login: (password: string) =>
    request<{ token: string }>("/api/login", {
      method: "POST",
      body: { password },
      auth: false,
    }),
  getServer: () => request<ServerInfo>("/api/server", { auth: false }),
  getServerTool: () =>
    request<ServerToolInfo>("/api/server/tool", { auth: false }),
  getMetrics: () =>
    request<ServerMetrics>("/api/server/metrics", { auth: false }),
  getSettings: () => request<Record<string, unknown>>("/api/server/settings"),
  getWorldSnapshot: () => request<WorldSnapshot>("/api/server/game-data"),
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
  getPlayers: (params: Record<string, unknown> = {}) =>
    request<PlayerSummary[]>(`/api/player${buildQuery(params)}`),
  getOnlinePlayers: () => request<PlayerSummary[]>("/api/online_player"),
  getPlayer: (playerUid: string) =>
    request<Player>(`/api/player/${encodeURIComponent(playerUid)}`),
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
  getGuilds: () => request<Guild[]>("/api/guild", { auth: false }),
  getGuild: (adminPlayerUid: string) =>
    request<Guild>(`/api/guild/${encodeURIComponent(adminPlayerUid)}`, {
      auth: false,
    }),
  getWhitelist: () => request<WhitelistPlayer[]>("/api/whitelist"),
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
  getBackups: (startTime?: number, endTime?: number) =>
    request<Backup[]>(`/api/backup${buildQuery({ startTime, endTime })}`),
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
