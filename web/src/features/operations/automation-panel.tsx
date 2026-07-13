import { useMemo, useState, type ComponentType } from "react";
import {
  BellRing,
  CalendarClock,
  CheckCircle2,
  Clock3,
  DatabaseBackup,
  History,
  LoaderCircle,
  Megaphone,
  Pencil,
  Play,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
  RotateCw,
  Save,
  Send,
  Settings2,
  ShieldCheck,
  Trash2,
  TriangleAlert,
  XCircle,
  type LucideProps,
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
import { Switch } from "@/components/ui/switch";
import { queryKeys } from "@/hooks/use-server-data";
import { api, getApiErrorCode, getApiErrorMessage } from "@/lib/api";
import { formatDateTime, formatRelativeTime } from "@/lib/format";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type {
  AutomationAction,
  AutomationSettingsUpdate,
  ScheduledTask,
  ScheduledTaskInput,
  TaskRun,
  TaskRunStatus,
  WatchdogRuntimeStatus,
} from "@/types/api";

import {
  AutomationTaskDialog,
  emptyAutomationTask,
} from "@/features/operations/automation-task-dialog";
import { AutomationSettingsDialog } from "@/features/operations/automation-settings-dialog";

const actionIcons: Record<AutomationAction, ComponentType<LucideProps>> = {
  save_world: Save,
  broadcast: Megaphone,
  start_server: Power,
  stop_server: PowerOff,
  restart_server: RotateCw,
  sync_save: RefreshCw,
  pst_backup: DatabaseBackup,
};

function toTaskInput(task: ScheduledTask): ScheduledTaskInput {
  return {
    name: task.name,
    enabled: task.enabled,
    action: task.action,
    schedule: {
      ...task.schedule,
      weekdays: [...(task.schedule.weekdays ?? [])],
    },
    parameters: { ...task.parameters },
  };
}

function runTone(status?: TaskRunStatus) {
  switch (status) {
    case "succeeded":
      return "text-[var(--success)] border-[var(--success)]/30 bg-[var(--success)]/8";
    case "failed":
      return "text-destructive border-destructive/30 bg-destructive/8";
    case "running":
      return "text-[var(--signal)] border-[var(--signal)]/30 bg-[var(--signal)]/8";
    case "skipped":
      return "text-[var(--warning)] border-[var(--warning)]/30 bg-[var(--warning)]/8";
    default:
      return "text-muted-foreground border-border bg-muted/30";
  }
}

export function AutomationPanel() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editorTask, setEditorTask] = useState<ScheduledTask | null>(null);
  const [editorDraft, setEditorDraft] = useState<ScheduledTaskInput>(() =>
    emptyAutomationTask(),
  );
  const [deleteTask, setDeleteTask] = useState<ScheduledTask | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);

  const tasksQuery = useQuery({
    queryKey: queryKeys.automationTasks,
    queryFn: api.getAutomationTasks,
    refetchInterval: 15_000,
  });
  const runsQuery = useQuery({
    queryKey: queryKeys.automationRuns,
    queryFn: () => api.getAutomationRuns("", 40),
    refetchInterval: 15_000,
  });
  const settingsQuery = useQuery({
    queryKey: queryKeys.automationSettings,
    queryFn: api.getAutomationSettings,
  });
  const statusQuery = useQuery({
    queryKey: queryKeys.automationStatus,
    queryFn: api.getAutomationStatus,
    refetchInterval: 8_000,
  });

  const refreshAutomation = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.automationTasks }),
      queryClient.invalidateQueries({ queryKey: queryKeys.automationRuns }),
      queryClient.invalidateQueries({ queryKey: queryKeys.automationSettings }),
      queryClient.invalidateQueries({ queryKey: queryKeys.automationStatus }),
      queryClient.invalidateQueries({ queryKey: queryKeys.control }),
    ]);
  };

  const saveTaskMutation = useMutation({
    mutationFn: ({
      taskId,
      input,
    }: {
      taskId?: string;
      input: ScheduledTaskInput;
    }) =>
      taskId
        ? api.updateAutomationTask(taskId, input)
        : api.createAutomationTask(input),
    onSuccess: async () => {
      toast.success(
        editorTask ? t("automation.taskUpdated") : t("automation.taskCreated"),
      );
      setEditorOpen(false);
      setEditorTask(null);
      await refreshAutomation();
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const toggleTaskMutation = useMutation({
    mutationFn: ({
      task,
      enabled,
    }: {
      task: ScheduledTask;
      enabled: boolean;
    }) =>
      api.updateAutomationTask(task.id, {
        ...toTaskInput(task),
        enabled,
      }),
    onSuccess: refreshAutomation,
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const deleteTaskMutation = useMutation({
    mutationFn: api.deleteAutomationTask,
    onSuccess: async () => {
      toast.success(t("automation.taskDeleted"));
      setDeleteTask(null);
      await refreshAutomation();
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const runTaskMutation = useMutation({
    mutationFn: api.runAutomationTask,
    onSuccess: async (run) => {
      toast.success(t("automation.taskRunCompleted"), {
        description: run.summary,
      });
      await refreshAutomation();
    },
    onError: async (error) => {
      toast.error(getApiErrorMessage(error));
      await refreshAutomation();
    },
  });

  const settingsMutation = useMutation({
    mutationFn: (settings: AutomationSettingsUpdate) =>
      api.updateAutomationSettings(settings),
    onSuccess: async () => {
      toast.success(t("automation.settingsSaved"));
      setSettingsOpen(false);
      await refreshAutomation();
    },
    onError: (error) => {
      toast.error(
        getApiErrorCode(error) === "watchdog_control_required"
          ? t("automation.watchdogControlRequired")
          : getApiErrorMessage(error),
      );
    },
  });

  const testNotificationMutation = useMutation({
    mutationFn: api.testAutomationNotification,
    onSuccess: async () => {
      toast.success(t("automation.testSent"));
      await refreshAutomation();
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const resetWatchdogMutation = useMutation({
    mutationFn: api.resetAutomationWatchdog,
    onSuccess: async () => {
      toast.success(t("automation.watchdogReset"));
      await refreshAutomation();
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const tasks = useMemo(() => tasksQuery.data ?? [], [tasksQuery.data]);
  const runs = runsQuery.data ?? [];
  const nextTask = useMemo(
    () =>
      tasks
        .filter((task) => task.next_run_at)
        .sort(
          (left, right) =>
            new Date(left.next_run_at ?? 0).getTime() -
            new Date(right.next_run_at ?? 0).getTime(),
        )[0],
    [tasks],
  );
  const lastRun = runs[0];
  const templates = useMemo(
    () => [
      {
        key: "save",
        icon: Save,
        draft: {
          name: t("automation.template.saveName"),
          enabled: true,
          action: "save_world",
          schedule: { kind: "interval", interval_minutes: 60 },
          parameters: {},
        } satisfies ScheduledTaskInput,
      },
      {
        key: "restart",
        icon: RotateCw,
        draft: {
          name: t("automation.template.restartName"),
          enabled: true,
          action: "restart_server",
          schedule: { kind: "daily", time_of_day: "04:00" },
          parameters: {
            delay_seconds: 60,
            message: t("automation.template.restartMessage"),
          },
        } satisfies ScheduledTaskInput,
      },
      {
        key: "backup",
        icon: DatabaseBackup,
        draft: {
          name: t("automation.template.backupName"),
          enabled: true,
          action: "pst_backup",
          schedule: { kind: "weekly", time_of_day: "03:30", weekdays: [0] },
          parameters: {},
        } satisfies ScheduledTaskInput,
      },
      {
        key: "sync",
        icon: RefreshCw,
        draft: {
          name: t("automation.template.syncName"),
          enabled: true,
          action: "sync_save",
          schedule: { kind: "interval", interval_minutes: 120 },
          parameters: {},
        } satisfies ScheduledTaskInput,
      },
    ],
    [t],
  );

  const openNewTask = (draft = emptyAutomationTask()) => {
    setEditorTask(null);
    setEditorDraft(draft);
    setEditorOpen(true);
  };

  const openEditTask = (task: ScheduledTask) => {
    setEditorTask(task);
    setEditorDraft(toTaskInput(task));
    setEditorOpen(true);
  };

  if (tasksQuery.isPending && statusQuery.isPending) return <LoadingState />;
  if (tasksQuery.isError) {
    return (
      <ErrorState
        error={tasksQuery.error}
        retry={() => void tasksQuery.refetch()}
      />
    );
  }

  return (
    <div className="space-y-5">
      <MaintenanceRail
        watchdogState={statusQuery.data?.watchdog.state}
        watchdogDetail={statusQuery.data?.watchdog.last_error}
        nextTask={nextTask}
        lastRun={lastRun}
      />

      <div className="grid gap-5 xl:grid-cols-[minmax(0,1.65fr)_minmax(320px,0.75fr)]">
        <Panel
          title={t("automation.tasks")}
          description={t("automation.tasksDescription")}
          actions={
            <Button size="sm" onClick={() => openNewTask()}>
              <Plus /> {t("automation.taskCreate")}
            </Button>
          }
        >
          <div className="border-b bg-[linear-gradient(135deg,color-mix(in_oklab,var(--primary)_9%,transparent),transparent_60%)] p-4 sm:p-5">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="font-display text-[10px] tracking-[0.18em] text-muted-foreground uppercase">
                  {t("automation.quickTemplates")}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("automation.quickTemplatesHint")}
                </p>
              </div>
              <Badge variant="outline">{t("automation.noCron")}</Badge>
            </div>
            <div className="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
              {templates.map(({ key, icon: Icon, draft }) => (
                <button
                  key={key}
                  type="button"
                  onClick={() => openNewTask(draft)}
                  className="group flex min-h-20 items-start gap-3 rounded-md border bg-card/85 p-3 text-left transition-colors hover:border-primary/35 hover:bg-card focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-md border bg-muted text-primary transition-colors group-hover:bg-primary/10">
                    <Icon className="size-4" />
                  </div>
                  <div className="min-w-0">
                    <p className="text-xs font-semibold">{draft.name}</p>
                    <p className="mt-1 text-[11px] leading-4 text-muted-foreground">
                      {scheduleLabel(draft, t)}
                    </p>
                  </div>
                </button>
              ))}
            </div>
          </div>

          {tasks.length === 0 ? (
            <div className="flex min-h-60 flex-col items-center justify-center px-6 text-center">
              <CalendarClock className="size-8 text-muted-foreground/60" />
              <p className="mt-4 text-sm font-medium">
                {t("automation.emptyTasks")}
              </p>
              <p className="mt-1 max-w-sm text-xs leading-5 text-muted-foreground">
                {t("automation.emptyTasksHint")}
              </p>
            </div>
          ) : (
            <div className="divide-y">
              {tasks.map((task) => (
                <TaskRow
                  key={task.id}
                  task={task}
                  toggling={
                    toggleTaskMutation.isPending &&
                    toggleTaskMutation.variables?.task.id === task.id
                  }
                  running={
                    runTaskMutation.isPending &&
                    runTaskMutation.variables === task.id
                  }
                  onToggle={(enabled) =>
                    toggleTaskMutation.mutate({ task, enabled })
                  }
                  onRun={() => runTaskMutation.mutate(task.id)}
                  onEdit={() => openEditTask(task)}
                  onDelete={() => setDeleteTask(task)}
                />
              ))}
            </div>
          )}
        </Panel>

        <div className="space-y-5">
          <WatchdogCard
            status={statusQuery.data?.watchdog}
            busy={statusQuery.data?.busy ?? false}
            onSettings={() => setSettingsOpen(true)}
            onReset={() => resetWatchdogMutation.mutate()}
            resetting={resetWatchdogMutation.isPending}
          />
          <NotificationCard
            status={statusQuery.data?.notification}
            onSettings={() => setSettingsOpen(true)}
            onTest={() => testNotificationMutation.mutate()}
            testing={testNotificationMutation.isPending}
          />
        </div>
      </div>

      <RunHistory runs={runs} loading={runsQuery.isPending} />

      {editorOpen ? (
        <AutomationTaskDialog
          key={editorTask?.id ?? `new-${editorDraft.action}`}
          open
          task={editorDraft}
          editing={Boolean(editorTask)}
          pending={saveTaskMutation.isPending}
          onOpenChange={setEditorOpen}
          onSubmit={(input) =>
            saveTaskMutation.mutate({ taskId: editorTask?.id, input })
          }
        />
      ) : null}

      {settingsQuery.data && settingsOpen ? (
        <AutomationSettingsDialog
          open
          settings={settingsQuery.data}
          pending={settingsMutation.isPending}
          onOpenChange={setSettingsOpen}
          onSubmit={(update) => settingsMutation.mutate(update)}
        />
      ) : null}

      <AlertDialog
        open={Boolean(deleteTask)}
        onOpenChange={(open) => {
          if (!open) setDeleteTask(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("automation.deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("automation.deleteDescription", {
                name: deleteTask?.name ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("action.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleteTaskMutation.isPending}
              onClick={() => {
                if (deleteTask) deleteTaskMutation.mutate(deleteTask.id);
              }}
            >
              {deleteTaskMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <Trash2 />
              )}
              {t("action.remove")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function MaintenanceRail({
  watchdogState,
  watchdogDetail,
  nextTask,
  lastRun,
}: {
  watchdogState?: string;
  watchdogDetail?: string;
  nextTask?: ScheduledTask;
  lastRun?: TaskRun;
}) {
  const { t } = useI18n();
  return (
    <section className="telemetry-grid relative overflow-hidden rounded-md border bg-card">
      <div className="pointer-events-none absolute top-[2.3rem] right-[16.6%] left-[16.6%] hidden h-px bg-border lg:block" />
      <div className="grid lg:grid-cols-3">
        <RailNode
          icon={ShieldCheck}
          label={t("automation.railWatchdog")}
          value={t(`automation.watchdogState.${watchdogState ?? "unknown"}`)}
          detail={watchdogDetail || t("automation.watchdogStateHint")}
          tone={watchdogTone(watchdogState)}
        />
        <RailNode
          icon={Clock3}
          label={t("automation.railNext")}
          value={nextTask?.name ?? t("automation.noNextTask")}
          detail={
            nextTask?.next_run_at
              ? `${formatRelativeTime(nextTask.next_run_at)} · ${formatDateTime(nextTask.next_run_at)}`
              : t("automation.noNextTaskHint")
          }
          tone="signal"
        />
        <RailNode
          icon={lastRun?.status === "failed" ? XCircle : CheckCircle2}
          label={t("automation.railLast")}
          value={lastRun?.task_name ?? t("automation.noRuns")}
          detail={
            lastRun
              ? `${t(`automation.runStatus.${lastRun.status}`)} · ${formatRelativeTime(lastRun.started_at)}`
              : t("automation.noRunsHint")
          }
          tone={lastRun?.status === "failed" ? "danger" : "success"}
        />
      </div>
    </section>
  );
}

function RailNode({
  icon: Icon,
  label,
  value,
  detail,
  tone,
}: {
  icon: ComponentType<LucideProps>;
  label: string;
  value: string;
  detail: string;
  tone: "success" | "warning" | "danger" | "signal" | "muted";
}) {
  const toneClasses = {
    success: "bg-[var(--success)] text-white",
    warning: "bg-[var(--warning)] text-white",
    danger: "bg-destructive text-white",
    signal: "bg-[var(--signal)] text-white",
    muted: "bg-muted text-muted-foreground",
  }[tone];
  return (
    <div className="relative flex min-h-28 items-start gap-3 border-b p-4 last:border-b-0 lg:border-r lg:border-b-0 lg:last:border-r-0 sm:p-5">
      <div
        className={cn(
          "relative z-10 flex size-9 shrink-0 items-center justify-center rounded-full border-4 border-card shadow-sm",
          toneClasses,
        )}
      >
        <Icon className="size-4" />
      </div>
      <div className="min-w-0 pt-0.5">
        <p className="font-display text-[10px] tracking-[0.16em] text-muted-foreground uppercase">
          {label}
        </p>
        <p className="mt-1 truncate text-sm font-semibold">{value}</p>
        <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">
          {detail}
        </p>
      </div>
    </div>
  );
}

function TaskRow({
  task,
  toggling,
  running,
  onToggle,
  onRun,
  onEdit,
  onDelete,
}: {
  task: ScheduledTask;
  toggling: boolean;
  running: boolean;
  onToggle: (enabled: boolean) => void;
  onRun: () => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const { t } = useI18n();
  const Icon = actionIcons[task.action];
  return (
    <article className="grid gap-4 p-4 transition-colors hover:bg-muted/20 sm:p-5 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
      <div className="flex min-w-0 items-start gap-3">
        <div className="flex size-9 shrink-0 items-center justify-center rounded-md border bg-muted text-primary">
          <Icon className="size-4" />
        </div>
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="truncate text-sm font-semibold">{task.name}</h3>
            <Badge variant="outline" className="font-normal">
              {t(`automation.action.${task.action}`)}
            </Badge>
            {task.last_run ? (
              <Badge
                variant="outline"
                className={cn("font-normal", runTone(task.last_run.status))}
              >
                {t(`automation.runStatus.${task.last_run.status}`)}
              </Badge>
            ) : null}
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            {scheduleLabel(task, t)}
          </p>
          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 font-data text-[11px] text-muted-foreground">
            <span>
              {t("automation.nextRun")}: {formatDateTime(task.next_run_at)}
            </span>
            <span>
              {t("automation.lastRun")}:{" "}
              {formatDateTime(task.last_run?.started_at)}
            </span>
          </div>
        </div>
      </div>
      <div className="flex items-center justify-between gap-2 pl-12 lg:justify-end lg:pl-0">
        <div className="mr-2 flex items-center gap-2">
          {toggling ? <LoaderCircle className="size-4 animate-spin" /> : null}
          <Switch
            size="sm"
            checked={task.enabled}
            disabled={toggling}
            onCheckedChange={onToggle}
            aria-label={t("automation.enabled")}
          />
        </div>
        <Button
          size="icon-sm"
          variant="outline"
          disabled={running || task.running}
          onClick={onRun}
        >
          {running || task.running ? (
            <LoaderCircle className="animate-spin" />
          ) : (
            <Play />
          )}
          <span className="sr-only">{t("automation.runNow")}</span>
        </Button>
        <Button size="icon-sm" variant="ghost" onClick={onEdit}>
          <Pencil />
          <span className="sr-only">{t("automation.taskEdit")}</span>
        </Button>
        <Button size="icon-sm" variant="ghost" onClick={onDelete}>
          <Trash2 />
          <span className="sr-only">{t("action.remove")}</span>
        </Button>
      </div>
    </article>
  );
}

function WatchdogCard({
  status,
  busy,
  onSettings,
  onReset,
  resetting,
}: {
  status?: WatchdogRuntimeStatus;
  busy: boolean;
  onSettings: () => void;
  onReset: () => void;
  resetting: boolean;
}) {
  const { t } = useI18n();
  return (
    <Panel
      title={t("automation.watchdog")}
      description={t("automation.watchdogCardDescription")}
      actions={
        <Button size="icon-sm" variant="ghost" onClick={onSettings}>
          <Settings2 />
          <span className="sr-only">{t("automation.settingsTitle")}</span>
        </Button>
      }
    >
      <div className="space-y-4 p-4 sm:p-5">
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <span
              className={cn(
                "size-2.5 rounded-full shadow-[0_0_0_4px_color-mix(in_oklab,currentColor_14%,transparent)]",
                watchdogDot(status?.state),
              )}
            />
            <div>
              <p className="text-sm font-semibold">
                {t(`automation.watchdogState.${status?.state ?? "unknown"}`)}
              </p>
              <p className="mt-0.5 text-xs text-muted-foreground">
                {busy
                  ? t("automation.operationBusy")
                  : status?.desired_running
                    ? t("automation.protectRunning")
                    : t("automation.intentionalStop")}
              </p>
            </div>
          </div>
          <Badge variant="outline">
            {t("automation.failureRecoveryCount", {
              failures: status?.consecutive_failures ?? 0,
              recoveries: status?.recovery_attempts ?? 0,
            })}
          </Badge>
        </div>
        {status?.last_error ? (
          <div className="flex gap-2 rounded-md border border-amber-500/25 bg-amber-500/8 p-3 text-xs leading-5 text-muted-foreground">
            <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-600" />
            <span>{status.last_error}</span>
          </div>
        ) : null}
        <dl className="grid grid-cols-2 gap-3 text-xs">
          <div>
            <dt className="text-muted-foreground">
              {t("automation.lastHealthy")}
            </dt>
            <dd className="mt-1 font-data">
              {formatRelativeTime(status?.last_healthy_at)}
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">
              {t("automation.nextCheck")}
            </dt>
            <dd className="mt-1 font-data">
              {formatRelativeTime(status?.next_check_at)}
            </dd>
          </div>
        </dl>
        <Button
          variant="outline"
          size="sm"
          className="w-full"
          disabled={resetting}
          onClick={onReset}
        >
          {resetting ? (
            <LoaderCircle className="animate-spin" />
          ) : (
            <RefreshCw />
          )}
          {t("automation.resetWatchdog")}
        </Button>
      </div>
    </Panel>
  );
}

function NotificationCard({
  status,
  onSettings,
  onTest,
  testing,
}: {
  status?: Awaited<ReturnType<typeof api.getAutomationStatus>>["notification"];
  onSettings: () => void;
  onTest: () => void;
  testing: boolean;
}) {
  const { t } = useI18n();
  return (
    <Panel
      title={t("automation.notifications")}
      description={t("automation.notificationCardDescription")}
      actions={
        <Button size="icon-sm" variant="ghost" onClick={onSettings}>
          <Settings2 />
          <span className="sr-only">{t("automation.settingsTitle")}</span>
        </Button>
      }
    >
      <div className="space-y-4 p-4 sm:p-5">
        <div className="flex items-center gap-3">
          <div
            className={cn(
              "flex size-9 items-center justify-center rounded-md border",
              status?.enabled && status.configured
                ? "border-[var(--signal)]/30 bg-[var(--signal)]/10 text-[var(--signal)]"
                : "bg-muted text-muted-foreground",
            )}
          >
            <BellRing className="size-4" />
          </div>
          <div className="min-w-0">
            <p className="text-sm font-semibold">
              {status?.enabled && status.configured
                ? t("automation.notificationReady")
                : t("automation.notificationDisabled")}
            </p>
            <p className="mt-0.5 truncate font-data text-[11px] text-muted-foreground">
              {status?.webhook_preview || t("automation.noWebhook")}
            </p>
          </div>
        </div>
        {status?.last_error ? (
          <p className="rounded-md border border-destructive/25 bg-destructive/8 p-3 text-xs leading-5 text-destructive">
            {status.last_error}
          </p>
        ) : null}
        <div className="flex items-center justify-between text-xs">
          <span className="text-muted-foreground">
            {t("automation.lastDelivery")}
          </span>
          <span className="font-data">
            {formatRelativeTime(status?.last_success_at)}
          </span>
        </div>
        <Button
          variant="outline"
          size="sm"
          className="w-full"
          disabled={!status?.configured || testing}
          onClick={onTest}
        >
          {testing ? <LoaderCircle className="animate-spin" /> : <Send />}
          {t("automation.sendTest")}
        </Button>
      </div>
    </Panel>
  );
}

function RunHistory({ runs, loading }: { runs: TaskRun[]; loading: boolean }) {
  const { t } = useI18n();
  return (
    <Panel
      title={t("automation.history")}
      description={t("automation.historyDescription")}
      actions={<History className="size-4 text-muted-foreground" />}
    >
      {loading ? (
        <LoadingState />
      ) : runs.length === 0 ? (
        <div className="flex min-h-32 items-center justify-center text-sm text-muted-foreground">
          {t("automation.noRuns")}
        </div>
      ) : (
        <div className="grid divide-y lg:grid-cols-2 lg:divide-x lg:divide-y-0">
          {runs.slice(0, 12).map((run, index) => (
            <div
              key={run.id}
              className={cn(
                "flex gap-3 px-4 py-3.5 sm:px-5",
                index >= 2 && "lg:border-t",
              )}
            >
              <span
                className={cn(
                  "mt-1.5 size-2 shrink-0 rounded-full",
                  run.status === "succeeded" && "bg-[var(--success)]",
                  run.status === "failed" && "bg-destructive",
                  run.status === "running" && "bg-[var(--signal)]",
                  run.status === "skipped" && "bg-[var(--warning)]",
                )}
              />
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-3">
                  <p className="truncate text-xs font-semibold">
                    {run.task_name}
                  </p>
                  <span className="shrink-0 font-data text-[10px] text-muted-foreground">
                    {formatRelativeTime(run.started_at)}
                  </span>
                </div>
                <p className="mt-1 line-clamp-2 text-[11px] leading-4 text-muted-foreground">
                  {run.error ||
                    run.summary ||
                    t(`automation.action.${run.action}`)}
                </p>
                <div className="mt-2 flex items-center gap-2">
                  <Badge
                    variant="outline"
                    className={cn("text-[10px]", runTone(run.status))}
                  >
                    {t(`automation.runStatus.${run.status}`)}
                  </Badge>
                  <span className="text-[10px] text-muted-foreground">
                    {t(`automation.trigger.${run.trigger}`)}
                  </span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </Panel>
  );
}

function scheduleLabel(
  task: ScheduledTaskInput,
  t: (key: string, values?: Record<string, string | number>) => string,
) {
  if (task.schedule.kind === "interval") {
    return t("automation.scheduleEvery", {
      minutes: task.schedule.interval_minutes ?? 0,
    });
  }
  if (task.schedule.kind === "daily") {
    return t("automation.scheduleDailyAt", {
      time: task.schedule.time_of_day ?? "--:--",
    });
  }
  const days = (task.schedule.weekdays ?? [])
    .map((weekday) => t(`automation.weekday.${weekday}`))
    .join(" / ");
  return t("automation.scheduleWeeklyAt", {
    days,
    time: task.schedule.time_of_day ?? "--:--",
  });
}

function watchdogTone(state?: string): Parameters<typeof RailNode>[0]["tone"] {
  if (state === "healthy") return "success";
  if (state === "degraded" || state === "cooldown" || state === "grace") {
    return "warning";
  }
  if (state === "unhealthy" || state === "exhausted") return "danger";
  if (state === "recovering") return "signal";
  return "muted";
}

function watchdogDot(state?: string) {
  if (state === "healthy") return "bg-[var(--success)] text-[var(--success)]";
  if (state === "recovering") return "bg-[var(--signal)] text-[var(--signal)]";
  if (state === "degraded" || state === "cooldown" || state === "grace") {
    return "bg-[var(--warning)] text-[var(--warning)]";
  }
  if (state === "unhealthy" || state === "exhausted") {
    return "bg-destructive text-destructive";
  }
  return "bg-muted-foreground text-muted-foreground";
}
