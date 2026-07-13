import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  ArrowRight,
  Building2,
  CircleGauge,
  Clock3,
  Copy,
  RefreshCw,
  Server,
  Shield,
  UsersRound,
} from "lucide-react";
import { Link } from "react-router-dom";
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip as ChartTooltip,
  XAxis,
  YAxis,
} from "recharts";
import { toast } from "sonner";

import { ErrorState, LoadingState } from "@/components/common/data-state";
import { Panel } from "@/components/common/panel";
import { SectionHeader } from "@/components/common/section-header";
import { StatusDot } from "@/components/common/status-dot";
import { FleetRail } from "@/components/fleet/fleet-rail";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  useGuilds,
  useOnlinePlayers,
  useServerInfo,
  useServerMetrics,
  useServerToolInfo,
} from "@/hooks/use-server-data";
import { copyText, formatCoordinate, formatDuration } from "@/lib/format";
import { useI18n } from "@/lib/i18n";
import type { ServerMetrics } from "@/types/api";

interface MetricSample {
  timestamp: number;
  at: string;
  fps: number;
  frameTime: number;
}

function MetricCell({
  icon: Icon,
  label,
  value,
  meta,
}: {
  icon: typeof Activity;
  label: string;
  value: string | number;
  meta?: string;
}) {
  return (
    <div className="min-w-0 border-b px-4 py-4 last:border-b-0 sm:border-b-0 sm:border-r sm:last:border-r-0 lg:px-5">
      <div className="flex items-center gap-2 text-muted-foreground">
        <Icon className="size-4" />
        <span className="text-xs font-medium">{label}</span>
      </div>
      <div className="mt-2 flex items-baseline gap-2">
        <span className="font-data text-2xl font-semibold">{value}</span>
        {meta ? (
          <span className="text-xs text-muted-foreground">{meta}</span>
        ) : null}
      </div>
    </div>
  );
}

function PerformanceChart({
  history,
  metrics,
}: {
  history: MetricSample[];
  metrics?: ServerMetrics;
}) {
  const { t } = useI18n();
  if (history.length < 2) {
    return (
      <div className="flex h-[260px] items-center justify-center text-sm text-muted-foreground">
        {t("overview.noHistory")}
      </div>
    );
  }

  return (
    <div className="h-[260px] w-full px-1 py-4 sm:px-4">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={history}
          margin={{ top: 8, right: 8, left: -18, bottom: 0 }}
        >
          <CartesianGrid
            strokeDasharray="3 3"
            vertical={false}
            stroke="var(--border)"
          />
          <XAxis
            dataKey="at"
            tick={{ fontSize: 10, fill: "var(--muted-foreground)" }}
            axisLine={false}
            tickLine={false}
          />
          <YAxis
            yAxisId="fps"
            domain={[0, Math.max(60, metrics?.server_fps ?? 60)]}
            tick={{ fontSize: 10, fill: "var(--muted-foreground)" }}
            axisLine={false}
            tickLine={false}
          />
          <YAxis
            yAxisId="frame"
            orientation="right"
            tick={{ fontSize: 10, fill: "var(--muted-foreground)" }}
            axisLine={false}
            tickLine={false}
          />
          <ChartTooltip
            contentStyle={{
              background: "var(--popover)",
              color: "var(--popover-foreground)",
              border: "1px solid var(--border)",
              borderRadius: 6,
              fontSize: 12,
            }}
          />
          <Legend wrapperStyle={{ fontSize: 11 }} />
          <Line
            yAxisId="fps"
            type="monotone"
            dataKey="fps"
            name="FPS"
            stroke="var(--signal)"
            strokeWidth={2}
            dot={false}
            isAnimationActive={false}
          />
          <Line
            yAxisId="frame"
            type="monotone"
            dataKey="frameTime"
            name="ms"
            stroke="var(--warning)"
            strokeWidth={1.5}
            dot={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

export default function OverviewView() {
  const { t } = useI18n();
  const serverQuery = useServerInfo();
  const metricsQuery = useServerMetrics();
  const toolQuery = useServerToolInfo();
  const onlinePlayersQuery = useOnlinePlayers();
  const guildsQuery = useGuilds();
  const [history, setHistory] = useState<MetricSample[]>([]);

  useEffect(() => {
    const metrics = metricsQuery.data;
    if (!metrics) return;
    const now = Date.now();
    const timer = window.setTimeout(() => {
      setHistory((current) => {
        const last = current[current.length - 1];
        if (last && now - last.timestamp < 1_000) return current;
        const sample = {
          timestamp: now,
          at: new Date(now).toLocaleTimeString([], {
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
          }),
          fps: metrics.server_fps,
          frameTime: metrics.server_frame_time,
        };
        return [...current.slice(-23), sample];
      });
    }, 0);
    return () => window.clearTimeout(timer);
  }, [metricsQuery.data, metricsQuery.dataUpdatedAt]);

  const guildSummary = useMemo(() => {
    const guilds = guildsQuery.data ?? [];
    return {
      guilds: guilds.length,
      members: guilds.reduce((sum, guild) => sum + guild.players.length, 0),
      camps: guilds.reduce((sum, guild) => sum + guild.base_camp.length, 0),
    };
  }, [guildsQuery.data]);

  if (serverQuery.isPending && metricsQuery.isPending) return <LoadingState />;
  if (serverQuery.isError && metricsQuery.isError) {
    return (
      <ErrorState
        error={serverQuery.error}
        retry={() => {
          void serverQuery.refetch();
          void metricsQuery.refetch();
        }}
      />
    );
  }

  const server = serverQuery.data;
  const metrics = metricsQuery.data;
  const onlinePlayers = onlinePlayersQuery.data ?? [];

  return (
    <div>
      <SectionHeader
        eyebrow="LIVE / WORLD 01"
        title={t("overview.title")}
        description={t("overview.subtitle")}
        actions={
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              void serverQuery.refetch();
              void metricsQuery.refetch();
            }}
          >
            <RefreshCw />
            {t("action.refresh")}
          </Button>
        }
      />

      <FleetRail />

      <div className="grid border-b sm:grid-cols-2 lg:grid-cols-4">
        <MetricCell
          icon={CircleGauge}
          label={t("metric.fps")}
          value={metrics?.server_fps ?? "--"}
          meta={`${metrics?.server_frame_time ?? "--"} ms`}
        />
        <MetricCell
          icon={UsersRound}
          label={t("metric.players")}
          value={metrics?.current_player_num ?? "--"}
          meta={`/ ${metrics?.max_player_num ?? "--"}`}
        />
        <MetricCell
          icon={Clock3}
          label={t("metric.uptime")}
          value={formatDuration(metrics?.uptime)}
          meta={t("overview.dayCount", { count: metrics?.days ?? "--" })}
        />
        <MetricCell
          icon={Building2}
          label={t("metric.camps")}
          value={metrics?.base_camp_num ?? guildSummary.camps}
          meta={t("overview.guildCount", { count: guildSummary.guilds })}
        />
      </div>

      <div className="grid gap-4 p-4 sm:p-6 xl:grid-cols-[minmax(0,1.7fr)_minmax(320px,0.8fr)] xl:p-8">
        <Panel
          title={t("overview.performance")}
          description={t("overview.performanceDescription")}
        >
          <PerformanceChart history={history} metrics={metrics} />
        </Panel>

        <Panel title={t("overview.identity")} contentClassName="divide-y">
          <div className="flex items-start gap-4 p-5">
            <div className="telemetry-grid flex size-12 shrink-0 items-center justify-center rounded-md border bg-muted text-primary">
              <Server className="size-6" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <StatusDot online={serverQuery.isSuccess} />
                <h3 className="truncate font-semibold">
                  {server?.name || "Palworld"}
                </h3>
              </div>
              <p className="mt-1 text-sm text-muted-foreground">
                {server?.description || t("overview.description")}
              </p>
            </div>
          </div>
          <dl className="divide-y text-sm">
            <div className="grid grid-cols-[130px_minmax(0,1fr)] gap-3 px-5 py-3">
              <dt className="text-muted-foreground">{t("overview.version")}</dt>
              <dd className="font-data text-right">
                {server?.version || "--"}
              </dd>
            </div>
            <div className="grid grid-cols-[130px_minmax(0,1fr)] gap-3 px-5 py-3">
              <dt className="text-muted-foreground">
                {t("overview.toolVersion")}
              </dt>
              <dd className="font-data text-right">
                {toolQuery.data?.version || "--"}
              </dd>
            </div>
            <div className="grid grid-cols-[130px_minmax(0,1fr)] gap-3 px-5 py-3">
              <dt className="text-muted-foreground">
                {t("overview.latestVersion")}
              </dt>
              <dd className="font-data text-right">
                {toolQuery.data?.latest || "--"}
              </dd>
            </div>
            <div className="px-5 py-3">
              <dt className="mb-1 text-muted-foreground">
                {t("overview.worldGuid")}
              </dt>
              <dd className="flex items-center gap-2">
                <code className="font-data min-w-0 flex-1 truncate text-xs">
                  {server?.world_guid || "--"}
                </code>
                {server?.world_guid ? (
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    onClick={() => {
                      void copyText(server.world_guid);
                      toast.success(t("message.copied"));
                    }}
                  >
                    <Copy />
                    <span className="sr-only">{t("action.copy")}</span>
                  </Button>
                ) : null}
              </dd>
            </div>
          </dl>
        </Panel>

        <Panel
          title={t("overview.onlinePlayers")}
          actions={
            <Button asChild variant="ghost" size="sm">
              <Link to="/players">
                {t("action.view")} <ArrowRight />
              </Link>
            </Button>
          }
        >
          {onlinePlayersQuery.isPending ? (
            <LoadingState className="min-h-56" />
          ) : onlinePlayers.length === 0 ? (
            <div className="flex min-h-56 items-center justify-center text-sm text-muted-foreground">
              {t("message.empty")}
            </div>
          ) : (
            <div className="divide-y">
              {onlinePlayers.slice(0, 6).map((player) => (
                <Link
                  key={player.player_uid}
                  to={`/players?player=${encodeURIComponent(player.player_uid)}`}
                  className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 px-4 py-3 transition-colors hover:bg-muted/55 sm:px-5"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <StatusDot online />
                      <span className="truncate text-sm font-medium">
                        {player.nickname || player.account_name}
                      </span>
                    </div>
                    <p className="font-data mt-1 truncate text-[11px] text-muted-foreground">
                      X {formatCoordinate(player.location_x)} / Y{" "}
                      {formatCoordinate(player.location_y)}
                    </p>
                  </div>
                  <Badge variant="secondary">Lv.{player.level}</Badge>
                </Link>
              ))}
            </div>
          )}
        </Panel>

        <Panel
          title={t("overview.guildActivity")}
          actions={
            <Button asChild variant="ghost" size="sm">
              <Link to="/guilds">
                {t("action.view")} <ArrowRight />
              </Link>
            </Button>
          }
        >
          {guildsQuery.isPending ? (
            <LoadingState className="min-h-56" />
          ) : (
            <div className="grid min-h-56 sm:grid-cols-3">
              <div className="flex flex-col justify-between border-b p-5 sm:border-b-0 sm:border-r">
                <Shield className="size-5 text-primary" />
                <div>
                  <p className="font-data text-3xl font-semibold">
                    {guildSummary.guilds}
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t("nav.guilds")}
                  </p>
                </div>
              </div>
              <div className="flex flex-col justify-between border-b p-5 sm:border-b-0 sm:border-r">
                <UsersRound className="size-5 text-[var(--signal)]" />
                <div>
                  <p className="font-data text-3xl font-semibold">
                    {guildSummary.members}
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t("guilds.members")}
                  </p>
                </div>
              </div>
              <div className="flex flex-col justify-between p-5">
                <Building2 className="size-5 text-[var(--warning)]" />
                <div>
                  <p className="font-data text-3xl font-semibold">
                    {guildSummary.camps}
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t("guilds.camps")}
                  </p>
                </div>
              </div>
            </div>
          )}
        </Panel>
      </div>
    </div>
  );
}
