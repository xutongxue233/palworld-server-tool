import {
  Gauge,
  Network,
  Server,
  TriangleAlert,
  UsersRound,
} from "lucide-react";
import { toast } from "sonner";
import { useIsMutating } from "@tanstack/react-query";

import { Panel } from "@/components/common/panel";
import { Badge } from "@/components/ui/badge";
import { useAuth } from "@/lib/auth";
import { useFleet } from "@/lib/fleet";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { FleetNode } from "@/types/api";

function railColor(node: FleetNode) {
  if (!node.reachable) return "bg-destructive";
  if (!node.server_online) return "bg-amber-500";
  return "bg-emerald-500";
}

export function FleetRail() {
  const { isAuthenticated } = useAuth();
  const { t } = useI18n();
  const { scope, nodes, issues, error, selectNode } = useFleet();
  const activeMutations = useIsMutating();

  if (!isAuthenticated || (nodes.length <= 1 && !issues?.length && !error)) {
    return null;
  }

  const choose = (node: FleetNode) => {
    if (node.scope === scope) return;
    if (selectNode(node.scope)) {
      toast.success(t("fleet.switched", { name: node.name }));
    }
  };

  return (
    <div className="border-b p-4 sm:p-6 xl:px-8">
      <Panel
        title={t("fleet.title")}
        description={t("fleet.description")}
        actions={
          <Badge variant="outline" className="font-data">
            <Network className="size-3" />
            v1 · {nodes.length}
          </Badge>
        }
        contentClassName="p-4 sm:p-5"
      >
        <div className="flex gap-3 overflow-x-auto pb-1">
          {nodes.map((node) => {
            const active = node.scope === scope;
            return (
              <button
                key={node.scope}
                type="button"
                disabled={activeMutations > 0 || (!node.selectable && !active)}
                onClick={() => choose(node)}
                className={cn(
                  "relative min-w-[230px] flex-1 overflow-hidden rounded-md border bg-card p-4 text-left transition-colors hover:border-primary/45 disabled:cursor-not-allowed disabled:opacity-60 sm:min-w-[260px]",
                  active && "border-primary/50 bg-primary/[0.035]",
                )}
              >
                <span
                  className={cn(
                    "absolute inset-y-0 left-0 w-1",
                    railColor(node),
                  )}
                />
                <span className="flex items-start justify-between gap-3 pl-1">
                  <span className="flex min-w-0 items-start gap-3">
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-md border bg-muted/35">
                      <Server className="size-4" />
                    </span>
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-semibold">
                        {node.name}
                      </span>
                      <span className="font-data mt-0.5 block truncate text-[10px] text-muted-foreground">
                        {node.server?.name || node.id}
                      </span>
                    </span>
                  </span>
                  <Badge variant={active ? "default" : "secondary"}>
                    {active
                      ? t("fleet.active")
                      : node.local
                        ? t("fleet.local")
                        : t("fleet.remote")}
                  </Badge>
                </span>

                <span className="mt-4 grid grid-cols-3 gap-2 border-t pt-3 text-[10px] text-muted-foreground">
                  <span>
                    <span className="flex items-center gap-1">
                      <UsersRound className="size-3" /> {t("metric.players")}
                    </span>
                    <strong className="font-data mt-1 block text-xs text-foreground">
                      {node.metrics
                        ? `${node.metrics.current_player_num}/${node.metrics.max_player_num}`
                        : "--"}
                    </strong>
                  </span>
                  <span>
                    <span className="flex items-center gap-1">
                      <Gauge className="size-3" /> FPS
                    </span>
                    <strong className="font-data mt-1 block text-xs text-foreground">
                      {node.metrics?.server_fps ?? "--"}
                    </strong>
                  </span>
                  <span>
                    <span>{t("fleet.latency")}</span>
                    <strong className="font-data mt-1 block text-xs text-foreground">
                      {node.local ? t("fleet.direct") : `${node.latency_ms} ms`}
                    </strong>
                  </span>
                </span>

                <span
                  className={cn(
                    "mt-3 block truncate text-[10px]",
                    node.error ? "text-destructive" : "text-muted-foreground",
                  )}
                >
                  {node.error ||
                    (node.server_online
                      ? t("fleet.nodeReady")
                      : t("fleet.serverStopped"))}
                </span>
              </button>
            );
          })}
        </div>

        {issues?.length ? (
          <div className="mt-4 flex gap-3 rounded-md border border-amber-500/30 bg-amber-500/7 p-3 text-[11px] leading-5 text-amber-800 dark:text-amber-200">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" />
            <div>
              <div className="font-semibold">
                {t("fleet.configIssues", { count: issues.length })}
              </div>
              <div className="mt-0.5 text-muted-foreground">
                {issues[0]?.message}
              </div>
            </div>
          </div>
        ) : null}
        {error ? (
          <div className="mt-4 flex gap-3 rounded-md border border-destructive/30 bg-destructive/7 p-3 text-[11px] leading-5 text-destructive">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" />
            <div>
              <div className="font-semibold">{t("fleet.loadFailed")}</div>
              <div className="mt-0.5 text-muted-foreground">
                {error.message}
              </div>
            </div>
          </div>
        ) : null}
      </Panel>
    </div>
  );
}
