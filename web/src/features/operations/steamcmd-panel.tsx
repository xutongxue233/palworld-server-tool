import { useState, type ReactNode } from "react";
import {
  ArrowRight,
  CircleCheck,
  Download,
  FileCheck2,
  FolderCog,
  HardDrive,
  LoaderCircle,
  PackageCheck,
  Power,
  RefreshCw,
  RotateCw,
  Server,
  ShieldCheck,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { ErrorState, LoadingState } from "@/components/common/data-state";
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
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { queryKeys, scopedQueryFn } from "@/hooks/use-server-data";
import { api, getApiErrorCode, getApiErrorMessage } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { SteamCMDUpdateResult } from "@/types/api";

function CheckCard({
  icon: Icon,
  title,
  state,
  children,
}: {
  icon: LucideIcon;
  title: string;
  state: "ready" | "warning" | "blocked";
  children: ReactNode;
}) {
  return (
    <article
      className={cn(
        "rounded-md border bg-card/80 p-4",
        state === "ready" && "border-emerald-500/25",
        state === "warning" && "border-amber-500/30",
        state === "blocked" && "border-destructive/30",
      )}
    >
      <div className="flex items-start gap-3">
        <div
          className={cn(
            "flex size-9 shrink-0 items-center justify-center rounded-md border bg-muted/35 text-muted-foreground",
            state === "ready" &&
              "border-emerald-500/25 bg-emerald-500/8 text-emerald-600",
            state === "warning" &&
              "border-amber-500/25 bg-amber-500/8 text-amber-600",
            state === "blocked" &&
              "border-destructive/25 bg-destructive/8 text-destructive",
          )}
        >
          <Icon className="size-4" />
        </div>
        <div className="min-w-0">
          <h3 className="text-xs font-semibold">{title}</h3>
          <div className="mt-1 text-xs leading-5 text-muted-foreground">
            {children}
          </div>
        </div>
      </div>
    </article>
  );
}

export function SteamCMDPanel() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [validateFiles, setValidateFiles] = useState(true);
  const [restartPreference, setRestartPreference] = useState<boolean | null>(
    null,
  );
  const [manualStopConfirmed, setManualStopConfirmed] = useState(false);
  const [confirmationOpen, setConfirmationOpen] = useState(false);
  const [lastResult, setLastResult] = useState<SteamCMDUpdateResult | null>(
    null,
  );

  const statusQuery = useQuery({
    queryKey: queryKeys.steamcmd,
    queryFn: scopedQueryFn(api.getSteamCMDStatus),
    refetchInterval: 15_000,
  });
  const plan = statusQuery.data?.plan;
  const control = statusQuery.data?.server_control;
  const restartAfter = Boolean(
    control?.configured &&
    (restartPreference ?? Boolean(control.online || control.running)),
  );

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!plan) throw new Error(t("steamcmd.planUnavailable"));
      return api.updateServerWithSteamCMD({
        expected_plan_digest: plan.plan_digest,
        confirm_update: true,
        confirm_server_stopped: Boolean(
          !control?.configured && manualStopConfirmed,
        ),
        validate_files: validateFiles,
        restart_after: restartAfter,
        shutdown_seconds: 10,
        shutdown_message: t("steamcmd.shutdownMessage"),
      });
    },
    onSuccess: async (result) => {
      setLastResult(result);
      setConfirmationOpen(false);
      const wasInstalled = result.update.before.installed;
      const messageKey = !wasInstalled
        ? "steamcmd.installSuccess"
        : result.update.changed
          ? "steamcmd.updateSuccess"
          : "steamcmd.alreadyCurrent";
      toast.success(t(messageKey), {
        description: t("steamcmd.resultBuild", {
          build: result.update.build_id_after || "--",
        }),
      });
      if (result.restart_error) {
        toast.warning(t("steamcmd.restartFailed"), {
          description: result.restart_error,
        });
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.steamcmd }),
        queryClient.invalidateQueries({ queryKey: queryKeys.control }),
        queryClient.invalidateQueries({ queryKey: queryKeys.automationStatus }),
        queryClient.invalidateQueries({ queryKey: queryKeys.backupsRoot }),
      ]);
    },
    onError: async (error) => {
      toast.error(getApiErrorMessage(error));
      if (getApiErrorCode(error) === "steamcmd_plan_changed") {
        setConfirmationOpen(false);
        await statusQuery.refetch();
      }
    },
  });

  if (statusQuery.isPending) {
    return (
      <Panel
        title={t("steamcmd.title")}
        description={t("steamcmd.description")}
      >
        <LoadingState />
      </Panel>
    );
  }
  if (statusQuery.isError || !plan || !control) {
    return (
      <Panel
        title={t("steamcmd.title")}
        description={t("steamcmd.description")}
      >
        <ErrorState
          error={statusQuery.error ?? new Error(t("steamcmd.planUnavailable"))}
          retry={() => void statusQuery.refetch()}
        />
      </Panel>
    );
  }

  const manuallyManaged = !control.configured;
  const serverStillOnline = control.online || control.running;
  const serverReady = control.configured
    ? !control.busy
    : !serverStillOnline && manualStopConfirmed;
  const canRun =
    plan.can_execute &&
    serverReady &&
    !updateMutation.isPending &&
    (!manuallyManaged || manualStopConfirmed);
  const operationLabel = plan.installed
    ? t("steamcmd.updateAction")
    : t("steamcmd.installAction");
  const pipeline = [
    { number: "01", icon: FileCheck2, label: t("steamcmd.pipelinePlan") },
    { number: "02", icon: Power, label: t("steamcmd.pipelineStop") },
    { number: "03", icon: ShieldCheck, label: t("steamcmd.pipelineBackup") },
    { number: "04", icon: Download, label: t("steamcmd.pipelineUpdate") },
    { number: "05", icon: PackageCheck, label: t("steamcmd.pipelineVerify") },
  ];

  return (
    <div className="space-y-4">
      <Panel
        title={t("steamcmd.title")}
        description={t("steamcmd.description")}
        actions={
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={statusQuery.isFetching || updateMutation.isPending}
            onClick={() => void statusQuery.refetch()}
          >
            <RefreshCw
              className={cn(statusQuery.isFetching && "animate-spin")}
            />
            <span className="sr-only">{t("action.refresh")}</span>
          </Button>
        }
      >
        <section className="border-b bg-[radial-gradient(circle_at_top_right,color-mix(in_oklab,var(--primary)_14%,transparent),transparent_42%),linear-gradient(135deg,color-mix(in_oklab,var(--primary)_7%,transparent),transparent_56%)] p-4 sm:p-5">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="outline" className="font-data">
                  APP {plan.app_id}
                </Badge>
                <Badge variant={plan.installed ? "secondary" : "outline"}>
                  {plan.installed
                    ? t("steamcmd.installed")
                    : t("steamcmd.notInstalled")}
                </Badge>
                {plan.partial_installation ? (
                  <Badge variant="outline" className="text-amber-600">
                    {t("steamcmd.partial")}
                  </Badge>
                ) : null}
                <Badge variant={plan.can_execute ? "secondary" : "destructive"}>
                  {plan.can_execute
                    ? t("steamcmd.preflightReady")
                    : t("steamcmd.preflightBlocked")}
                </Badge>
              </div>
              <div className="mt-4 flex items-baseline gap-3">
                <span className="font-data text-[10px] tracking-[0.18em] text-muted-foreground uppercase">
                  {t("steamcmd.build")}
                </span>
                <span className="font-data text-2xl font-semibold tracking-tight">
                  {plan.build_id || "--"}
                </span>
              </div>
              <p className="mt-2 max-w-3xl text-xs leading-5 text-muted-foreground">
                {t("steamcmd.fixedCommandHint")}
              </p>
            </div>
            <div className="grid shrink-0 grid-cols-2 gap-4 rounded-md border bg-background/70 px-4 py-3 text-xs backdrop-blur-sm">
              <div>
                <div className="text-[9px] tracking-[0.16em] text-muted-foreground uppercase">
                  {t("steamcmd.platform")}
                </div>
                <div className="font-data mt-1 font-medium uppercase">
                  {plan.platform}
                </div>
              </div>
              <div>
                <div className="text-[9px] tracking-[0.16em] text-muted-foreground uppercase">
                  {t("steamcmd.timeout")}
                </div>
                <div className="font-data mt-1 font-medium">
                  {plan.timeout_seconds}s
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="border-b p-4 sm:p-5">
          <div className="mb-3 flex items-center justify-between gap-3">
            <div>
              <h3 className="text-sm font-semibold">
                {t("steamcmd.preflight")}
              </h3>
              <p className="mt-1 text-xs text-muted-foreground">
                {t("steamcmd.preflightHint")}
              </p>
            </div>
            {plan.can_execute ? (
              <CircleCheck className="size-5 text-emerald-600" />
            ) : (
              <TriangleAlert className="size-5 text-destructive" />
            )}
          </div>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <CheckCard
              icon={PackageCheck}
              title={t("steamcmd.executable")}
              state={plan.executable_sha256 ? "ready" : "blocked"}
            >
              <span className="font-data block break-all">
                {plan.executable_path || t("steamcmd.notConfigured")}
              </span>
            </CheckCard>
            <CheckCard
              icon={FolderCog}
              title={t("steamcmd.installDir")}
              state={plan.install_dir ? "ready" : "blocked"}
            >
              <span className="font-data block break-all">
                {plan.install_dir || t("steamcmd.notConfigured")}
              </span>
            </CheckCard>
            <CheckCard
              icon={HardDrive}
              title={t("steamcmd.safetyBackup")}
              state={
                !plan.safety_backup_required || plan.safety_backup_ready
                  ? "ready"
                  : "blocked"
              }
            >
              {plan.safety_backup_required
                ? plan.safety_backup_ready
                  ? t("steamcmd.backupReady", {
                      count: plan.existing_worlds,
                    })
                  : t("steamcmd.backupMissing", {
                      count: plan.existing_worlds,
                    })
                : t("steamcmd.backupNotRequired")}
            </CheckCard>
            <CheckCard
              icon={Server}
              title={t("steamcmd.serverState")}
              state={
                control.configured
                  ? control.busy
                    ? "blocked"
                    : serverStillOnline
                      ? "warning"
                      : "ready"
                  : serverStillOnline
                    ? "blocked"
                    : manualStopConfirmed
                      ? "ready"
                      : "warning"
              }
            >
              {control.configured
                ? t("steamcmd.managedState", { state: control.state })
                : serverStillOnline
                  ? t("steamcmd.manualOnlineBlocked")
                  : t("steamcmd.manualState")}
            </CheckCard>
          </div>

          {plan.issues?.length ? (
            <div className="mt-3 rounded-md border border-destructive/25 bg-destructive/5 p-3">
              <div className="flex items-center gap-2 text-xs font-semibold text-destructive">
                <TriangleAlert className="size-4" />
                {t("steamcmd.blockers")}
              </div>
              <ul className="mt-2 space-y-1 pl-6 text-xs leading-5 text-destructive/90">
                {plan.issues.map((issue) => (
                  <li key={issue} className="list-disc break-words">
                    {issue}
                  </li>
                ))}
              </ul>
              <p className="mt-2 text-xs text-muted-foreground">
                {t("steamcmd.configHint")}
              </p>
            </div>
          ) : null}
          {plan.warnings?.length ? (
            <div className="mt-3 rounded-md border border-amber-500/25 bg-amber-500/7 p-3">
              <div className="flex items-center gap-2 text-xs font-semibold text-amber-700 dark:text-amber-400">
                <TriangleAlert className="size-4" />
                {t("steamcmd.warnings")}
              </div>
              <ul className="mt-2 space-y-1 pl-6 text-xs leading-5 text-muted-foreground">
                {plan.warnings.map((warning) => (
                  <li key={warning} className="list-disc break-words">
                    {warning}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </section>

        <section className="p-4 sm:p-5">
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(300px,0.58fr)]">
            <div>
              <h3 className="text-sm font-semibold">
                {t("steamcmd.pipeline")}
              </h3>
              <p className="mt-1 text-xs text-muted-foreground">
                {t("steamcmd.pipelineHint")}
              </p>
              <div className="mt-4 overflow-hidden rounded-md border">
                <div className="grid sm:grid-cols-5">
                  {pipeline.map((step, index) => {
                    const StepIcon = step.icon;
                    return (
                      <div
                        key={step.number}
                        className="relative flex items-center gap-3 border-b p-3 last:border-b-0 sm:block sm:border-r sm:border-b-0 sm:last:border-r-0"
                      >
                        <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                          <StepIcon className="size-3.5" />
                        </div>
                        <div className="sm:mt-3">
                          <div className="font-data text-[9px] text-muted-foreground">
                            {step.number}
                          </div>
                          <div className="mt-0.5 text-xs font-medium leading-4">
                            {step.label}
                          </div>
                        </div>
                        {index < pipeline.length - 1 ? (
                          <div className="absolute top-6 right-[-5px] z-10 hidden size-2 rotate-45 border-t border-r bg-background sm:block" />
                        ) : null}
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>

            <div className="rounded-md border bg-muted/20 p-4">
              <div className="space-y-4">
                <div className="flex items-center justify-between gap-4">
                  <Label htmlFor="steamcmd-validate" className="grid gap-1">
                    <span>{t("steamcmd.validateFiles")}</span>
                    <span className="font-normal text-muted-foreground">
                      {t("steamcmd.validateFilesHint")}
                    </span>
                  </Label>
                  <Switch
                    id="steamcmd-validate"
                    checked={validateFiles}
                    disabled={updateMutation.isPending}
                    onCheckedChange={setValidateFiles}
                  />
                </div>
                <div className="flex items-center justify-between gap-4 border-t pt-4">
                  <Label htmlFor="steamcmd-restart" className="grid gap-1">
                    <span>{t("steamcmd.restartAfter")}</span>
                    <span className="font-normal text-muted-foreground">
                      {control.configured
                        ? t("steamcmd.restartAfterHint")
                        : t("steamcmd.restartUnavailable")}
                    </span>
                  </Label>
                  <Switch
                    id="steamcmd-restart"
                    checked={restartAfter}
                    disabled={!control.configured || updateMutation.isPending}
                    onCheckedChange={setRestartPreference}
                  />
                </div>
                {!control.configured ? (
                  <div className="flex items-center justify-between gap-4 border-t pt-4">
                    <Label
                      htmlFor="steamcmd-manual-stop"
                      className="grid gap-1"
                    >
                      <span>{t("steamcmd.manualStopConfirm")}</span>
                      <span className="font-normal text-muted-foreground">
                        {t("steamcmd.manualStopHint")}
                      </span>
                    </Label>
                    <Switch
                      id="steamcmd-manual-stop"
                      checked={manualStopConfirmed}
                      disabled={serverStillOnline || updateMutation.isPending}
                      onCheckedChange={setManualStopConfirmed}
                    />
                  </div>
                ) : null}
              </div>

              <Button
                className="mt-5 w-full"
                size="lg"
                disabled={!canRun}
                onClick={() => setConfirmationOpen(true)}
              >
                {updateMutation.isPending ? (
                  <LoaderCircle className="animate-spin" />
                ) : plan.installed ? (
                  <RotateCw />
                ) : (
                  <Download />
                )}
                {updateMutation.isPending
                  ? t("steamcmd.running")
                  : operationLabel}
              </Button>
              {!plan.can_execute ? (
                <p className="mt-2 text-center text-xs text-destructive">
                  {t("steamcmd.resolveBlockers")}
                </p>
              ) : !serverReady ? (
                <p className="mt-2 text-center text-xs text-amber-700 dark:text-amber-400">
                  {serverStillOnline && !control.configured
                    ? t("steamcmd.stopManuallyFirst")
                    : t("steamcmd.confirmManualStopFirst")}
                </p>
              ) : null}
            </div>
          </div>
        </section>
      </Panel>

      {lastResult ? (
        <Panel
          title={t("steamcmd.lastResult")}
          description={formatDateTime(lastResult.update.finished_at)}
          contentClassName="p-4 sm:p-5"
        >
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="secondary">
                  <CircleCheck /> {t("steamcmd.completed")}
                </Badge>
                {lastResult.update.validated ? (
                  <Badge variant="outline">
                    {t("steamcmd.filesValidated")}
                  </Badge>
                ) : null}
                {lastResult.restarted ? (
                  <Badge variant="outline">{t("steamcmd.restarted")}</Badge>
                ) : null}
              </div>
              <div className="mt-4 flex flex-wrap items-center gap-3 font-data text-lg font-semibold">
                <span>{lastResult.update.build_id_before || "--"}</span>
                <ArrowRight className="size-4 text-muted-foreground" />
                <span className="text-primary">
                  {lastResult.update.build_id_after || "--"}
                </span>
              </div>
              {lastResult.safety_backup ? (
                <p className="mt-3 text-xs text-muted-foreground">
                  {t("steamcmd.safetyBackupCreated", {
                    name: lastResult.safety_backup.path,
                  })}
                </p>
              ) : null}
              {lastResult.restart_error ? (
                <div className="mt-3 flex gap-2 rounded-md border border-amber-500/25 bg-amber-500/7 p-3 text-xs text-amber-700 dark:text-amber-400">
                  <TriangleAlert className="mt-0.5 size-4 shrink-0" />
                  <span>{lastResult.restart_error}</span>
                </div>
              ) : null}
            </div>
            <div className="rounded-md border bg-muted/25 px-4 py-3 text-right">
              <div className="text-[9px] tracking-[0.16em] text-muted-foreground uppercase">
                {t("steamcmd.duration")}
              </div>
              <div className="font-data mt-1 text-lg font-semibold">
                {(lastResult.update.duration_ms / 1000).toFixed(1)}s
              </div>
            </div>
          </div>
          {lastResult.update.output_tail ? (
            <details className="mt-4 overflow-hidden rounded-md border">
              <summary className="cursor-pointer bg-muted/25 px-4 py-3 text-xs font-medium">
                {t("steamcmd.outputTail")}
              </summary>
              <pre className="font-data max-h-72 overflow-auto whitespace-pre-wrap break-words border-t p-4 text-[11px] leading-5 text-muted-foreground">
                {lastResult.update.output_tail}
              </pre>
            </details>
          ) : null}
        </Panel>
      ) : null}

      <AlertDialog
        open={confirmationOpen}
        onOpenChange={(open) =>
          !updateMutation.isPending && setConfirmationOpen(open)
        }
      >
        <AlertDialogContent className="sm:max-w-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("steamcmd.confirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("steamcmd.confirmDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <div className="overflow-hidden rounded-md border">
            <div className="grid sm:grid-cols-5">
              {pipeline.map((step) => {
                const StepIcon = step.icon;
                return (
                  <div
                    key={step.number}
                    className="flex items-center gap-3 border-b p-3 last:border-b-0 sm:block sm:border-r sm:border-b-0 sm:last:border-r-0"
                  >
                    <StepIcon className="size-4 text-primary" />
                    <div className="mt-0 sm:mt-2">
                      <div className="font-data text-[9px] text-muted-foreground">
                        {step.number}
                      </div>
                      <div className="text-xs font-medium">{step.label}</div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          <div className="grid gap-3 rounded-md bg-muted/35 p-4 text-xs">
            <div className="flex items-start justify-between gap-4">
              <span className="text-muted-foreground">
                {t("steamcmd.installDir")}
              </span>
              <span className="font-data max-w-[70%] break-all text-right font-medium">
                {plan.install_dir}
              </span>
            </div>
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">
                {t("steamcmd.safetyBackup")}
              </span>
              <span className="font-medium">
                {plan.safety_backup_required
                  ? t("steamcmd.required")
                  : t("steamcmd.notRequired")}
              </span>
            </div>
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">
                {t("steamcmd.restartAfter")}
              </span>
              <span className="font-medium">
                {restartAfter ? t("common.yes") : t("common.no")}
              </span>
            </div>
          </div>

          <div className="flex gap-3 rounded-md border border-amber-500/30 bg-amber-500/8 p-3 text-xs leading-5">
            <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-600" />
            <span>{t("steamcmd.confirmWarning")}</span>
          </div>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={updateMutation.isPending}>
              {t("action.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={!canRun || updateMutation.isPending}
              onClick={(event) => {
                event.preventDefault();
                updateMutation.mutate();
              }}
            >
              {updateMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : plan.installed ? (
                <RotateCw />
              ) : (
                <Download />
              )}
              {updateMutation.isPending
                ? t("steamcmd.running")
                : t("steamcmd.confirmRun")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
