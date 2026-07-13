import {
  Check,
  ChevronDown,
  LoaderCircle,
  Network,
  Server,
  TriangleAlert,
} from "lucide-react";
import { toast } from "sonner";
import { useIsMutating } from "@tanstack/react-query";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { LOCAL_SERVER_SCOPE } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useFleet } from "@/lib/fleet";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { FleetNode } from "@/types/api";

function nodeSignal(node: FleetNode) {
  if (!node.reachable) return "bg-destructive";
  if (!node.server_online) return "bg-amber-500";
  return "bg-emerald-500";
}

export function ServerSelector() {
  const { isAuthenticated } = useAuth();
  const { t } = useI18n();
  const { scope, activeNode, nodes, issues, isFetching, error, selectNode } =
    useFleet();
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
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          disabled={activeMutations > 0}
          aria-label={t("fleet.currentNode", {
            name: activeNode?.name ?? t("fleet.selector"),
          })}
          className="max-w-[190px] gap-2 px-2.5 sm:max-w-[240px]"
        >
          {isFetching ? <LoaderCircle className="animate-spin" /> : <Network />}
          <span
            className={cn(
              "size-1.5 shrink-0 rounded-full",
              error
                ? "bg-destructive"
                : activeNode
                  ? nodeSignal(activeNode)
                  : "bg-muted-foreground",
            )}
          />
          <span className="hidden min-w-0 truncate sm:inline">
            {activeNode?.name ?? t("fleet.selector")}
          </span>
          <ChevronDown className="size-3.5 text-muted-foreground" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="w-[min(22rem,calc(100vw-1rem))]"
      >
        <DropdownMenuLabel className="flex items-center justify-between gap-3">
          <span>{t("fleet.selector")}</span>
          <Badge variant="outline" className="font-data text-[9px]">
            {t("fleet.nodeCount", { count: nodes.length })}
          </Badge>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <div className="max-h-[min(26rem,70vh)] overflow-y-auto p-1">
          {nodes.map((node) => {
            const active = node.scope === scope;
            return (
              <DropdownMenuItem
                key={node.scope}
                disabled={activeMutations > 0 || (!node.selectable && !active)}
                className="items-start gap-3 py-3"
                onSelect={() => choose(node)}
              >
                <span className="relative mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md border bg-muted/40">
                  <Server className="size-4" />
                  <span
                    className={cn(
                      "absolute -right-0.5 -bottom-0.5 size-2 rounded-full ring-2 ring-popover",
                      nodeSignal(node),
                    )}
                  />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-2">
                    <span className="truncate text-xs font-semibold">
                      {node.name}
                    </span>
                    <Badge variant="secondary" className="text-[8px]">
                      {node.local ? t("fleet.local") : t("fleet.remote")}
                    </Badge>
                    {node.insecure_transport ? (
                      <Badge
                        variant="outline"
                        className="border-amber-500/30 text-[8px] text-amber-700 dark:text-amber-300"
                      >
                        HTTP
                      </Badge>
                    ) : null}
                  </span>
                  <span className="font-data mt-1 block truncate text-[10px] text-muted-foreground">
                    {node.server?.name || node.id}
                  </span>
                  <span className="mt-1 block text-[10px] text-muted-foreground">
                    {node.reachable
                      ? node.server_online
                        ? t("fleet.nodeOnline", {
                            players: node.metrics?.current_player_num ?? 0,
                            max: node.metrics?.max_player_num ?? "--",
                            latency: node.latency_ms,
                          })
                        : t("fleet.serverStopped")
                      : node.error || t("fleet.nodeUnreachable")}
                  </span>
                </span>
                {active ? (
                  <Check className="mt-1 size-4 shrink-0 text-primary" />
                ) : null}
              </DropdownMenuItem>
            );
          })}
        </div>
        {issues?.length ? (
          <>
            <DropdownMenuSeparator />
            <div className="space-y-1.5 px-3 py-2 text-[10px] leading-4 text-amber-700 dark:text-amber-300">
              <div className="flex items-center gap-1.5 font-semibold">
                <TriangleAlert className="size-3.5" />
                {t("fleet.configIssues", { count: issues.length })}
              </div>
              {issues.slice(0, 3).map((issue, index) => (
                <p key={`${issue.code}-${issue.node_id ?? index}`}>
                  {issue.message}
                </p>
              ))}
            </div>
          </>
        ) : null}
        {error ? (
          <>
            <DropdownMenuSeparator />
            <div className="px-3 py-2 text-[10px] leading-4 text-destructive">
              {t("fleet.loadFailed")}: {error.message}
            </div>
          </>
        ) : null}
        {activeNode?.scope !== LOCAL_SERVER_SCOPE ? (
          <>
            <DropdownMenuSeparator />
            <div className="px-3 py-2 text-[10px] leading-4 text-muted-foreground">
              {t("fleet.remoteBoundary")}
            </div>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
