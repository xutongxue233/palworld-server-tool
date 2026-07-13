import { useQuery } from "@tanstack/react-query";

import { api } from "@/lib/api";

export const queryKeys = {
  server: ["server"] as const,
  metrics: ["server", "metrics"] as const,
  tool: ["server", "tool"] as const,
  players: ["players"] as const,
  onlinePlayers: ["players", "online"] as const,
  player: (playerUid: string) => ["players", playerUid] as const,
  guilds: ["guilds"] as const,
  settings: ["server", "settings"] as const,
  snapshot: ["server", "snapshot"] as const,
  control: ["server", "control"] as const,
  whitelist: ["whitelist"] as const,
  backups: (startTime?: number, endTime?: number) =>
    ["backups", startTime, endTime] as const,
  nativeBackups: ["backups", "native"] as const,
  automationTasks: ["automation", "tasks"] as const,
  automationRuns: ["automation", "runs"] as const,
  automationSettings: ["automation", "settings"] as const,
  automationStatus: ["automation", "status"] as const,
};

export function useServerInfo() {
  return useQuery({
    queryKey: queryKeys.server,
    queryFn: api.getServer,
    refetchInterval: 30_000,
    retry: 2,
  });
}

export function useServerMetrics() {
  return useQuery({
    queryKey: queryKeys.metrics,
    queryFn: api.getMetrics,
    refetchInterval: 5_000,
    retry: 2,
  });
}

export function useServerToolInfo() {
  return useQuery({
    queryKey: queryKeys.tool,
    queryFn: api.getServerTool,
    staleTime: 15 * 60_000,
    retry: 1,
  });
}

export function usePlayers() {
  return useQuery({
    queryKey: queryKeys.players,
    queryFn: () => api.getPlayers({ order_by: "last_online", desc: true }),
    refetchInterval: 20_000,
  });
}

export function useOnlinePlayers() {
  return useQuery({
    queryKey: queryKeys.onlinePlayers,
    queryFn: api.getOnlinePlayers,
    refetchInterval: 8_000,
  });
}

export function usePlayer(playerUid: string | null) {
  return useQuery({
    queryKey: queryKeys.player(playerUid ?? ""),
    queryFn: () => api.getPlayer(playerUid ?? ""),
    enabled: Boolean(playerUid),
  });
}

export function useGuilds() {
  return useQuery({
    queryKey: queryKeys.guilds,
    queryFn: api.getGuilds,
    refetchInterval: 30_000,
  });
}
