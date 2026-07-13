import { useQuery, type QueryFunctionContext } from "@tanstack/react-query";

import { api, getServerScope } from "@/lib/api";

function scopedQueryKey(...parts: Array<string | number | undefined>) {
  return ["server-scope", getServerScope(), ...parts] as const;
}

function serverScopeFromQueryKey(queryKey: readonly unknown[]) {
  const scope = queryKey[0] === "server-scope" ? queryKey[1] : null;
  if (typeof scope !== "string") {
    throw new Error("Server query is missing its fleet scope");
  }
  return scope;
}

export function scopedQueryFn<T>(load: (scope: string) => Promise<T>) {
  // Bind retries and late refetches to the scope encoded in their cache key.
  return ({ queryKey }: QueryFunctionContext) =>
    load(serverScopeFromQueryKey(queryKey));
}

export const queryKeys = {
  get server() {
    return scopedQueryKey("server");
  },
  get metrics() {
    return scopedQueryKey("server", "metrics");
  },
  get tool() {
    return scopedQueryKey("server", "tool");
  },
  get players() {
    return scopedQueryKey("players");
  },
  get onlinePlayers() {
    return scopedQueryKey("players", "online");
  },
  player: (playerUid: string) => scopedQueryKey("players", playerUid),
  get guilds() {
    return scopedQueryKey("guilds");
  },
  get settings() {
    return scopedQueryKey("server", "settings");
  },
  get snapshot() {
    return scopedQueryKey("server", "snapshot");
  },
  get control() {
    return scopedQueryKey("server", "control");
  },
  get steamcmd() {
    return scopedQueryKey("server", "steamcmd");
  },
  get officialMods() {
    return scopedQueryKey("server", "mods");
  },
  get saveMigration() {
    return scopedQueryKey("server", "migration");
  },
  get whitelist() {
    return scopedQueryKey("whitelist");
  },
  backups: (startTime?: number, endTime?: number) =>
    scopedQueryKey("backups", startTime, endTime),
  get backupsRoot() {
    return scopedQueryKey("backups");
  },
  get nativeBackups() {
    return scopedQueryKey("backups", "native");
  },
  get automationTasks() {
    return scopedQueryKey("automation", "tasks");
  },
  get automationRuns() {
    return scopedQueryKey("automation", "runs");
  },
  get automationSettings() {
    return scopedQueryKey("automation", "settings");
  },
  get automationStatus() {
    return scopedQueryKey("automation", "status");
  },
};

export function useServerInfo() {
  return useQuery({
    queryKey: queryKeys.server,
    queryFn: scopedQueryFn(api.getServer),
    refetchInterval: 30_000,
    retry: 2,
  });
}

export function useServerMetrics() {
  return useQuery({
    queryKey: queryKeys.metrics,
    queryFn: scopedQueryFn(api.getMetrics),
    refetchInterval: 5_000,
    retry: 2,
  });
}

export function useServerToolInfo() {
  return useQuery({
    queryKey: queryKeys.tool,
    queryFn: scopedQueryFn(api.getServerTool),
    staleTime: 15 * 60_000,
    retry: 1,
  });
}

export function usePlayers() {
  return useQuery({
    queryKey: queryKeys.players,
    queryFn: scopedQueryFn((scope) =>
      api.getPlayers({ order_by: "last_online", desc: true }, scope),
    ),
    refetchInterval: 20_000,
  });
}

export function useOnlinePlayers() {
  return useQuery({
    queryKey: queryKeys.onlinePlayers,
    queryFn: scopedQueryFn(api.getOnlinePlayers),
    refetchInterval: 8_000,
  });
}

export function usePlayer(playerUid: string | null) {
  return useQuery({
    queryKey: queryKeys.player(playerUid ?? ""),
    queryFn: scopedQueryFn((scope) => api.getPlayer(playerUid ?? "", scope)),
    enabled: Boolean(playerUid),
  });
}

export function useGuilds() {
  return useQuery({
    queryKey: queryKeys.guilds,
    queryFn: scopedQueryFn(api.getGuilds),
    refetchInterval: 30_000,
  });
}
