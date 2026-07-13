import { useState, type FormEvent, type ReactNode } from "react";
import {
  ArrowRight,
  CircleCheck,
  DatabaseBackup,
  FileCheck2,
  FolderInput,
  FolderLock,
  Import,
  LoaderCircle,
  Power,
  RefreshCw,
  RotateCw,
  Server,
  ShieldCheck,
  TriangleAlert,
} from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { Panel } from "@/components/common/panel";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { queryKeys } from "@/hooks/use-server-data";
import { api, getApiErrorCode, getApiErrorMessage } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type {
  SaveMigrationApplyResult,
  SaveMigrationKind,
  SaveMigrationNotice,
  SaveMigrationPlatform,
  SaveMigrationPreflightResult,
} from "@/types/api";

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const unit = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  return `${(bytes / 1024 ** unit).toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

function SummaryCell({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="rounded-md border bg-background/65 px-3 py-2.5">
      <div className="text-[9px] tracking-[0.16em] text-muted-foreground uppercase">
        {label}
      </div>
      <div className="font-data mt-1 text-sm font-semibold">{children}</div>
    </div>
  );
}

export function SaveMigrationPanel() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [sourcePath, setSourcePath] = useState("");
  const [sourcePlatform, setSourcePlatform] =
    useState<SaveMigrationPlatform>("current");
  const [sourceKind, setSourceKind] = useState<SaveMigrationKind>("dedicated");
  const [preflight, setPreflight] =
    useState<SaveMigrationPreflightResult | null>(null);
  const [manualStopConfirmed, setManualStopConfirmed] = useState(false);
  const [restartAfter, setRestartAfter] = useState(false);
  const [confirmationOpen, setConfirmationOpen] = useState(false);
  const [lastResult, setLastResult] = useState<SaveMigrationApplyResult | null>(
    null,
  );

  const clearPlan = () => {
    setPreflight(null);
    setManualStopConfirmed(false);
    setRestartAfter(false);
  };

  const preflightMutation = useMutation({
    mutationFn: () =>
      api.preflightSaveMigration({
        source_path: sourcePath.trim(),
        source_platform: sourcePlatform,
        source_kind: sourceKind,
      }),
    onSuccess: (result) => {
      setPreflight(result);
      setManualStopConfirmed(false);
      setRestartAfter(
        Boolean(
          result.server_control.configured &&
          (result.server_control.online || result.server_control.running),
        ),
      );
      if (result.plan.can_migrate) {
        toast.success(t("migration.preflightReady"), {
          description: t("migration.preflightReadyHint", {
            players: result.plan.source_player_files,
          }),
        });
      } else {
        toast.warning(t("migration.preflightBlocked"), {
          description: t("migration.resolveBlockers"),
        });
      }
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const plan = preflight?.plan;
  const control = preflight?.server_control;
  const serverStillOnline = Boolean(control?.online || control?.running);
  const managedState = control?.state.trim().toLowerCase() || "";
  const managedStateKnown = Boolean(
    control?.configured &&
    !control.detail &&
    managedState &&
    !["unknown", "invalid", "unconfigured"].includes(managedState),
  );
  const serverReady = Boolean(
    control &&
    !control.busy &&
    (control.configured
      ? managedStateKnown
      : serverStillOnline || manualStopConfirmed),
  );

  const applyMutation = useMutation({
    mutationFn: () => {
      if (!plan || !control) throw new Error(t("migration.planUnavailable"));
      return api.applySaveMigration({
        source_path: plan.source_path || sourcePath.trim(),
        source_platform: sourcePlatform,
        source_kind: sourceKind,
        expected_plan_digest: plan.plan_digest,
        confirm_migration: true,
        confirm_server_stopped: Boolean(
          !control.configured && !serverStillOnline && manualStopConfirmed,
        ),
        restart_after: restartAfter,
        shutdown_seconds: 10,
        shutdown_message: t("migration.shutdownMessage"),
      });
    },
    onSuccess: async (result) => {
      setLastResult(result);
      setPreflight(null);
      setManualStopConfirmed(false);
      setRestartAfter(false);
      setConfirmationOpen(false);
      toast.success(t("migration.success"), {
        description: t("migration.successHint", {
          source: result.migration.plan.source_world_id || "--",
          destination: result.migration.plan.destination_world_id || "--",
        }),
      });
      if (result.sync_error) {
        toast.warning(t("migration.syncFailed"), {
          description: result.sync_error,
        });
      }
      if (result.restart_error) {
        toast.warning(t("migration.restartFailed"), {
          description: result.restart_error,
        });
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.control }),
        queryClient.invalidateQueries({ queryKey: queryKeys.players }),
        queryClient.invalidateQueries({ queryKey: queryKeys.guilds }),
        queryClient.invalidateQueries({ queryKey: queryKeys.settings }),
        queryClient.invalidateQueries({ queryKey: queryKeys.snapshot }),
        queryClient.invalidateQueries({ queryKey: queryKeys.nativeBackups }),
        queryClient.invalidateQueries({ queryKey: queryKeys.backupsRoot }),
        queryClient.invalidateQueries({ queryKey: queryKeys.automationStatus }),
      ]);
    },
    onError: async (error) => {
      toast.error(getApiErrorMessage(error));
      if (getApiErrorCode(error) === "save_migration_plan_changed") {
        setConfirmationOpen(false);
        await preflightMutation.mutateAsync().catch(() => undefined);
      }
    },
  });

  const submitPreflight = (event: FormEvent) => {
    event.preventDefault();
    if (!sourcePath.trim()) return;
    setLastResult(null);
    preflightMutation.mutate();
  };

  const operationPending =
    preflightMutation.isPending || applyMutation.isPending;
  const applyReady = Boolean(
    plan?.can_migrate && serverReady && !operationPending,
  );
  const noticeText = (notice: SaveMigrationNotice) => {
    const key = `migration.notice.${notice.code}`;
    const translated = t(key);
    return translated === key ? notice.message : translated;
  };
  const pipeline = [
    { icon: FileCheck2, label: t("migration.pipelinePlan") },
    { icon: Power, label: t("migration.pipelineStop") },
    { icon: DatabaseBackup, label: t("migration.pipelineBackup") },
    { icon: FolderInput, label: t("migration.pipelineStage") },
    { icon: ShieldCheck, label: t("migration.pipelineSwap") },
    { icon: RotateCw, label: t("migration.pipelineSync") },
  ];

  return (
    <div className="space-y-4">
      <Panel
        title={t("migration.title")}
        description={t("migration.description")}
      >
        <form
          className="border-b bg-[linear-gradient(115deg,color-mix(in_oklab,var(--primary)_8%,transparent),transparent_48%)] p-4 sm:p-5"
          onSubmit={submitPreflight}
        >
          <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_240px_170px_auto] xl:items-end">
            <div className="min-w-0 space-y-2">
              <Label htmlFor="migration-source-path">
                {t("migration.sourcePath")}
              </Label>
              <Input
                id="migration-source-path"
                className="font-data"
                value={sourcePath}
                disabled={operationPending}
                placeholder={t("migration.sourcePathPlaceholder")}
                autoComplete="off"
                onChange={(event) => {
                  setSourcePath(event.target.value);
                  clearPlan();
                }}
              />
              <p className="text-xs leading-5 text-muted-foreground">
                {t("migration.sourcePathHint")}
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="migration-platform">
                {t("migration.sourcePlatform")}
              </Label>
              <Select
                value={sourcePlatform}
                disabled={operationPending}
                onValueChange={(value) => {
                  setSourcePlatform(value as SaveMigrationPlatform);
                  clearPlan();
                }}
              >
                <SelectTrigger id="migration-platform" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="current">
                    {t("migration.platformCurrent")}
                  </SelectItem>
                  <SelectItem value="windows">Windows</SelectItem>
                  <SelectItem value="linux">Linux</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="migration-source-kind">
                {t("migration.sourceKind")}
              </Label>
              <Select
                value={sourceKind}
                disabled={operationPending}
                onValueChange={(value) => {
                  setSourceKind(value as SaveMigrationKind);
                  clearPlan();
                }}
              >
                <SelectTrigger id="migration-source-kind" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="dedicated">
                    {t("migration.kindDedicated")}
                  </SelectItem>
                  <SelectItem value="coop">
                    {t("migration.kindCoop")}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <Button
              type="submit"
              size="lg"
              disabled={!sourcePath.trim() || operationPending}
            >
              {preflightMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : plan ? (
                <RefreshCw />
              ) : (
                <FileCheck2 />
              )}
              {preflightMutation.isPending
                ? t("migration.preflighting")
                : plan
                  ? t("migration.preflightAgain")
                  : t("migration.preflightAction")}
            </Button>
          </div>
        </form>

        {!plan || !control ? (
          <div className="p-4 sm:p-5">
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(260px,0.5fr)]">
              <div className="rounded-md border border-dashed p-5">
                <div className="flex size-10 items-center justify-center rounded-md border bg-muted/40 text-primary">
                  <Import className="size-5" />
                </div>
                <h3 className="mt-4 text-sm font-semibold">
                  {t("migration.emptyTitle")}
                </h3>
                <p className="mt-2 max-w-2xl text-xs leading-5 text-muted-foreground">
                  {t("migration.emptyDescription")}
                </p>
              </div>
              <div
                className={cn(
                  "rounded-md border p-4",
                  sourceKind === "coop"
                    ? "border-destructive/25 bg-destructive/5"
                    : "bg-muted/20",
                )}
              >
                <div className="flex items-center gap-2 text-xs font-semibold">
                  {sourceKind === "coop" ? (
                    <TriangleAlert className="size-4 text-destructive" />
                  ) : (
                    <ShieldCheck className="size-4 text-emerald-600" />
                  )}
                  {sourceKind === "coop"
                    ? t("migration.coopBlockedTitle")
                    : t("migration.safeScopeTitle")}
                </div>
                <p className="mt-2 text-xs leading-5 text-muted-foreground">
                  {sourceKind === "coop"
                    ? t("migration.coopBlockedHint")
                    : t("migration.safeScopeHint")}
                </p>
              </div>
            </div>
          </div>
        ) : (
          <>
            <section className="border-b p-4 sm:p-5">
              <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="outline" className="font-data">
                    PALWORLD {plan.game_version}
                  </Badge>
                  <Badge variant="outline" className="font-data uppercase">
                    {plan.source_platform} → {plan.destination_platform}
                  </Badge>
                  <Badge
                    variant={plan.can_migrate ? "secondary" : "destructive"}
                  >
                    {plan.can_migrate ? <CircleCheck /> : <TriangleAlert />}
                    {plan.can_migrate
                      ? t("migration.preflightReady")
                      : t("migration.preflightBlocked")}
                  </Badge>
                </div>
                <span className="font-data text-[10px] text-muted-foreground">
                  {plan.plan_digest.slice(0, 12)}
                </span>
              </div>

              <div className="grid items-stretch gap-3 lg:grid-cols-[minmax(0,1fr)_44px_minmax(0,1fr)]">
                <article className="overflow-hidden rounded-md border border-sky-500/25 bg-sky-500/5">
                  <div className="flex items-center justify-between border-b border-sky-500/15 px-4 py-3">
                    <div className="flex items-center gap-2 text-xs font-semibold">
                      <FolderInput className="size-4 text-sky-600" />
                      {t("migration.sourceWorld")}
                    </div>
                    <Badge variant="outline">
                      {plan.source_world_id || "--"}
                    </Badge>
                  </div>
                  <div className="p-4">
                    <p className="font-data break-all text-[11px] leading-5 text-muted-foreground">
                      {plan.source_path || plan.source_input}
                    </p>
                    <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4 lg:grid-cols-2 xl:grid-cols-4">
                      <SummaryCell label={t("migration.players")}>
                        {plan.source_player_files}
                      </SummaryCell>
                      <SummaryCell label={t("migration.files")}>
                        {plan.source_file_count}
                      </SummaryCell>
                      <SummaryCell label={t("migration.size")}>
                        {formatBytes(plan.source_size_bytes)}
                      </SummaryCell>
                      <SummaryCell label="WorldOption">
                        {plan.source_has_world_option
                          ? t("common.yes")
                          : t("common.no")}
                      </SummaryCell>
                    </div>
                  </div>
                </article>

                <div className="flex items-center justify-center">
                  <div className="flex size-10 items-center justify-center rounded-full border bg-background text-primary shadow-sm">
                    <ArrowRight className="size-4 lg:rotate-0" />
                  </div>
                </div>

                <article className="overflow-hidden rounded-md border border-violet-500/25 bg-violet-500/5">
                  <div className="flex items-center justify-between border-b border-violet-500/15 px-4 py-3">
                    <div className="flex items-center gap-2 text-xs font-semibold">
                      <FolderLock className="size-4 text-violet-600" />
                      {t("migration.destinationWorld")}
                    </div>
                    <Badge variant="outline">
                      {plan.destination_world_id || "--"}
                    </Badge>
                  </div>
                  <div className="p-4">
                    <p className="font-data break-all text-[11px] leading-5 text-muted-foreground">
                      {plan.destination_path || t("migration.notConfigured")}
                    </p>
                    <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4 lg:grid-cols-2 xl:grid-cols-4">
                      <SummaryCell label={t("migration.players")}>
                        {plan.destination_player_files}
                      </SummaryCell>
                      <SummaryCell label={t("migration.files")}>
                        {plan.destination_file_count}
                      </SummaryCell>
                      <SummaryCell label={t("migration.size")}>
                        {formatBytes(plan.destination_size_bytes)}
                      </SummaryCell>
                      <SummaryCell label="WorldOption">
                        {plan.destination_has_world_option
                          ? t("common.yes")
                          : t("common.no")}
                      </SummaryCell>
                    </div>
                  </div>
                </article>
              </div>

              <div className="mt-3 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(280px,0.55fr)]">
                <div className="rounded-md border bg-muted/20 p-3">
                  <div className="text-[9px] tracking-[0.16em] text-muted-foreground uppercase">
                    {t("migration.replaceScope")}
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {["Level.sav", "LevelMeta.sav", "Players/"].map((entry) => (
                      <Badge
                        key={entry}
                        variant="outline"
                        className="font-data"
                      >
                        {entry}
                      </Badge>
                    ))}
                    <Badge
                      variant="outline"
                      className={cn(
                        "font-data",
                        plan.source_has_world_option
                          ? "text-amber-600"
                          : "text-muted-foreground line-through",
                      )}
                    >
                      WorldOption.sav
                    </Badge>
                  </div>
                </div>
                <div className="rounded-md border border-emerald-500/20 bg-emerald-500/5 p-3 text-xs leading-5 text-muted-foreground">
                  <div className="flex items-center gap-2 font-semibold text-emerald-700 dark:text-emerald-400">
                    <ShieldCheck className="size-4" />
                    {t("migration.preservedTitle")}
                  </div>
                  <p className="mt-1">{t("migration.preservedHint")}</p>
                </div>
              </div>

              {plan.issues?.length ? (
                <div className="mt-3 rounded-md border border-destructive/25 bg-destructive/5 p-3">
                  <div className="flex items-center gap-2 text-xs font-semibold text-destructive">
                    <TriangleAlert className="size-4" />
                    {t("migration.blockers")}
                  </div>
                  <ul className="mt-2 space-y-1 pl-6 text-xs leading-5 text-destructive/90">
                    {plan.issues.map((issue) => (
                      <li
                        key={`${issue.code}:${issue.message}`}
                        className="list-disc break-words"
                      >
                        {noticeText(issue)}
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}
              {plan.warnings?.length ? (
                <div className="mt-3 rounded-md border border-amber-500/25 bg-amber-500/7 p-3">
                  <div className="flex items-center gap-2 text-xs font-semibold text-amber-700 dark:text-amber-400">
                    <TriangleAlert className="size-4" />
                    {t("migration.warnings")}
                  </div>
                  <ul className="mt-2 space-y-1 pl-6 text-xs leading-5 text-muted-foreground">
                    {plan.warnings.map((warning) => (
                      <li
                        key={`${warning.code}:${warning.message}`}
                        className="list-disc break-words"
                      >
                        {noticeText(warning)}
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}
            </section>

            <section className="p-4 sm:p-5">
              <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(300px,0.5fr)]">
                <div>
                  <h3 className="text-sm font-semibold">
                    {t("migration.pipeline")}
                  </h3>
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">
                    {t("migration.pipelineHint")}
                  </p>
                  <div className="mt-4 grid overflow-hidden rounded-md border sm:grid-cols-3 xl:grid-cols-6">
                    {pipeline.map((step, index) => {
                      const StepIcon = step.icon;
                      return (
                        <div
                          key={step.label}
                          className="relative flex items-center gap-3 border-b p-3 last:border-b-0 sm:border-r sm:nth-[3n]:border-r-0 xl:border-b-0 xl:nth-[3n]:border-r xl:last:border-r-0"
                        >
                          <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                            <StepIcon className="size-3.5" />
                          </div>
                          <div>
                            <div className="font-data text-[9px] text-muted-foreground">
                              {String(index + 1).padStart(2, "0")}
                            </div>
                            <div className="mt-0.5 text-xs font-medium leading-4">
                              {step.label}
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>

                <div className="rounded-md border bg-muted/20 p-4">
                  <div className="flex items-start gap-3 border-b pb-4">
                    <Server className="mt-0.5 size-4 shrink-0 text-primary" />
                    <div className="min-w-0">
                      <div className="text-xs font-semibold">
                        {t("migration.serverState")}
                      </div>
                      <p className="mt-1 text-xs leading-5 text-muted-foreground">
                        {control.configured
                          ? t("migration.managedState", {
                              state: control.state,
                            })
                          : serverStillOnline
                            ? t("migration.restStopAvailable")
                            : t("migration.manualState")}
                      </p>
                    </div>
                  </div>

                  {control.configured ? (
                    <div className="flex items-center justify-between gap-4 border-b py-4">
                      <Label htmlFor="migration-restart" className="grid gap-1">
                        <span>{t("migration.restartAfter")}</span>
                        <span className="font-normal text-muted-foreground">
                          {t("migration.restartAfterHint")}
                        </span>
                      </Label>
                      <Switch
                        id="migration-restart"
                        checked={restartAfter}
                        disabled={operationPending}
                        onCheckedChange={setRestartAfter}
                      />
                    </div>
                  ) : !serverStillOnline ? (
                    <div className="flex items-center justify-between gap-4 border-b py-4">
                      <Label
                        htmlFor="migration-manual-stop"
                        className="grid gap-1"
                      >
                        <span>{t("migration.manualStopConfirm")}</span>
                        <span className="font-normal text-muted-foreground">
                          {t("migration.manualStopHint")}
                        </span>
                      </Label>
                      <Switch
                        id="migration-manual-stop"
                        checked={manualStopConfirmed}
                        disabled={operationPending}
                        onCheckedChange={setManualStopConfirmed}
                      />
                    </div>
                  ) : (
                    <div className="border-b py-4 text-xs leading-5 text-amber-700 dark:text-amber-400">
                      {t("migration.restartUnavailable")}
                    </div>
                  )}

                  <Button
                    className="mt-4 w-full"
                    size="lg"
                    disabled={!applyReady}
                    onClick={() => setConfirmationOpen(true)}
                  >
                    {applyMutation.isPending ? (
                      <LoaderCircle className="animate-spin" />
                    ) : (
                      <Import />
                    )}
                    {applyMutation.isPending
                      ? t("migration.running")
                      : t("migration.applyAction")}
                  </Button>
                  {!plan.can_migrate ? (
                    <p className="mt-2 text-center text-xs text-destructive">
                      {t("migration.resolveBlockers")}
                    </p>
                  ) : control.busy ? (
                    <p className="mt-2 text-center text-xs text-amber-700 dark:text-amber-400">
                      {t("migration.serverBusy")}
                    </p>
                  ) : !serverReady ? (
                    <p className="mt-2 text-center text-xs text-amber-700 dark:text-amber-400">
                      {control.configured
                        ? t("migration.serverStatusUnknown")
                        : t("migration.confirmManualStopFirst")}
                    </p>
                  ) : null}
                </div>
              </div>
            </section>
          </>
        )}
      </Panel>

      {lastResult ? (
        <Panel
          title={t("migration.lastResult")}
          description={t("migration.successHint", {
            source: lastResult.migration.plan.source_world_id || "--",
            destination: lastResult.migration.plan.destination_world_id || "--",
          })}
          contentClassName="p-4 sm:p-5"
        >
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="secondary">
                  <CircleCheck /> {t("migration.completed")}
                </Badge>
                <Badge variant="outline">
                  <ShieldCheck /> {t("migration.validated")}
                </Badge>
                {lastResult.restarted ? (
                  <Badge variant="outline">{t("migration.restarted")}</Badge>
                ) : null}
              </div>
              <div className="font-data mt-4 flex flex-wrap items-center gap-3 text-base font-semibold">
                <span>{lastResult.migration.plan.source_world_id || "--"}</span>
                <ArrowRight className="size-4 text-muted-foreground" />
                <span className="text-primary">
                  {lastResult.migration.plan.destination_world_id || "--"}
                </span>
              </div>
              {lastResult.sync_error ? (
                <div className="mt-3 flex gap-2 rounded-md border border-amber-500/25 bg-amber-500/7 p-3 text-xs text-amber-700 dark:text-amber-400">
                  <TriangleAlert className="mt-0.5 size-4 shrink-0" />
                  <span>{lastResult.sync_error}</span>
                </div>
              ) : null}
              {lastResult.restart_error ? (
                <div className="mt-3 flex gap-2 rounded-md border border-amber-500/25 bg-amber-500/7 p-3 text-xs text-amber-700 dark:text-amber-400">
                  <TriangleAlert className="mt-0.5 size-4 shrink-0" />
                  <span>{lastResult.restart_error}</span>
                </div>
              ) : null}
            </div>
            <div className="min-w-64 rounded-md border bg-muted/25 px-4 py-3">
              <div className="flex items-center gap-2 text-[9px] tracking-[0.16em] text-muted-foreground uppercase">
                <DatabaseBackup className="size-3.5" />
                {t("migration.safetyBackup")}
              </div>
              <div className="font-data mt-2 break-all text-xs font-semibold">
                {lastResult.migration.safety_backup.path}
              </div>
            </div>
          </div>
        </Panel>
      ) : null}

      <AlertDialog
        open={confirmationOpen}
        onOpenChange={(open) => {
          if (!applyMutation.isPending) setConfirmationOpen(open);
        }}
      >
        <AlertDialogContent className="data-[size=default]:sm:max-w-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("migration.confirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("migration.confirmDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <div className="space-y-3 text-xs">
            <div className="grid grid-cols-[100px_minmax(0,1fr)] gap-3 rounded-md border bg-muted/20 p-3">
              <span className="text-muted-foreground">
                {t("migration.sourceWorld")}
              </span>
              <span className="font-data break-all text-right font-medium">
                {plan?.source_path || "--"}
              </span>
              <span className="text-muted-foreground">
                {t("migration.destinationWorld")}
              </span>
              <span className="font-data break-all text-right font-medium">
                {plan?.destination_path || "--"}
              </span>
              <span className="text-muted-foreground">
                {t("migration.safetyBackup")}
              </span>
              <span className="text-right font-medium text-emerald-700 dark:text-emerald-400">
                {t("migration.mandatory")}
              </span>
            </div>
            <div className="flex gap-3 rounded-md border border-amber-500/30 bg-amber-500/8 p-3 leading-5">
              <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-600" />
              <span>{t("migration.confirmWarning")}</span>
            </div>
          </div>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={applyMutation.isPending}>
              {t("action.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={!applyReady || applyMutation.isPending}
              onClick={(event) => {
                event.preventDefault();
                applyMutation.mutate();
              }}
            >
              {applyMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <Import />
              )}
              {applyMutation.isPending
                ? t("migration.running")
                : t("migration.confirmRun")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
