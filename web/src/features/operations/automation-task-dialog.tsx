import { useMemo, useState } from "react";
import { CalendarClock, LoaderCircle, Save } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { Textarea } from "@/components/ui/textarea";
import { useI18n } from "@/lib/i18n";
import type {
  AutomationAction,
  AutomationScheduleKind,
  ScheduledTaskInput,
} from "@/types/api";

const actions: AutomationAction[] = [
  "save_world",
  "broadcast",
  "start_server",
  "stop_server",
  "restart_server",
  "sync_save",
  "pst_backup",
];

const weekdays = [0, 1, 2, 3, 4, 5, 6];

export function emptyAutomationTask(): ScheduledTaskInput {
  return {
    name: "",
    enabled: true,
    action: "save_world",
    schedule: { kind: "interval", interval_minutes: 60 },
    parameters: {},
  };
}

function cloneTask(task: ScheduledTaskInput): ScheduledTaskInput {
  return {
    ...task,
    schedule: {
      ...task.schedule,
      weekdays: [...(task.schedule.weekdays ?? [])],
    },
    parameters: { ...task.parameters },
  };
}

export function AutomationTaskDialog({
  open,
  task,
  editing,
  pending,
  onOpenChange,
  onSubmit,
}: {
  open: boolean;
  task: ScheduledTaskInput;
  editing: boolean;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (task: ScheduledTaskInput) => void;
}) {
  const { t } = useI18n();
  const [draft, setDraft] = useState(() => cloneTask(task));

  const needsMessage =
    draft.action === "broadcast" ||
    draft.action === "restart_server" ||
    draft.action === "stop_server";
  const needsDelay =
    draft.action === "restart_server" || draft.action === "stop_server";
  const valid = useMemo(() => {
    if (!draft.name.trim()) return false;
    if (draft.schedule.kind === "interval") {
      const interval = draft.schedule.interval_minutes ?? 0;
      if (interval < 5 || interval > 10080) return false;
    }
    if (
      (draft.schedule.kind === "daily" || draft.schedule.kind === "weekly") &&
      !/^\d{2}:\d{2}$/.test(draft.schedule.time_of_day ?? "")
    ) {
      return false;
    }
    if (
      draft.schedule.kind === "weekly" &&
      (draft.schedule.weekdays?.length ?? 0) === 0
    ) {
      return false;
    }
    if (draft.action === "broadcast" && !draft.parameters.message?.trim()) {
      return false;
    }
    if (needsDelay) {
      const delay = draft.parameters.delay_seconds ?? 10;
      if (delay < 1 || delay > 300) return false;
    }
    return true;
  }, [draft, needsDelay]);

  const changeScheduleKind = (kind: AutomationScheduleKind) => {
    setDraft((current) => ({
      ...current,
      schedule:
        kind === "interval"
          ? { kind, interval_minutes: current.schedule.interval_minutes ?? 60 }
          : kind === "daily"
            ? { kind, time_of_day: current.schedule.time_of_day ?? "04:00" }
            : {
                kind,
                time_of_day: current.schedule.time_of_day ?? "04:00",
                weekdays: current.schedule.weekdays?.length
                  ? current.schedule.weekdays
                  : [0],
              },
    }));
  };

  const toggleWeekday = (weekday: number, checked: boolean) => {
    setDraft((current) => {
      const selected = new Set(current.schedule.weekdays ?? []);
      if (checked) selected.add(weekday);
      else selected.delete(weekday);
      return {
        ...current,
        schedule: {
          ...current.schedule,
          weekdays: [...selected].sort((left, right) => left - right),
        },
      };
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <div className="flex items-center gap-3">
            <div className="flex size-9 items-center justify-center rounded-md border border-primary/25 bg-primary/10 text-primary">
              <CalendarClock className="size-4" />
            </div>
            <div>
              <DialogTitle>
                {editing
                  ? t("automation.taskEdit")
                  : t("automation.taskCreate")}
              </DialogTitle>
              <DialogDescription className="mt-1">
                {t("automation.taskDialogDescription")}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="grid gap-5 py-2 sm:grid-cols-2">
          <div className="space-y-2 sm:col-span-2">
            <Label htmlFor="automation-task-name">
              {t("automation.taskName")}
            </Label>
            <Input
              id="automation-task-name"
              value={draft.name}
              maxLength={80}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  name: event.target.value,
                }))
              }
              placeholder={t("automation.taskNamePlaceholder")}
            />
          </div>

          <div className="space-y-2">
            <Label>{t("automation.action")}</Label>
            <Select
              value={draft.action}
              onValueChange={(value: AutomationAction) =>
                setDraft((current) => ({
                  ...current,
                  action: value,
                  parameters:
                    value === "broadcast"
                      ? { message: current.parameters.message ?? "" }
                      : value === "restart_server" || value === "stop_server"
                        ? {
                            message: current.parameters.message ?? "",
                            delay_seconds:
                              current.parameters.delay_seconds ?? 10,
                          }
                        : {},
                }))
              }
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {actions.map((action) => (
                  <SelectItem key={action} value={action}>
                    {t(`automation.action.${action}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>{t("automation.schedule")}</Label>
            <Select
              value={draft.schedule.kind}
              onValueChange={(value: AutomationScheduleKind) =>
                changeScheduleKind(value)
              }
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="interval">
                  {t("automation.schedule.interval")}
                </SelectItem>
                <SelectItem value="daily">
                  {t("automation.schedule.daily")}
                </SelectItem>
                <SelectItem value="weekly">
                  {t("automation.schedule.weekly")}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          {draft.schedule.kind === "interval" ? (
            <div className="space-y-2 sm:col-span-2">
              <Label htmlFor="automation-interval">
                {t("automation.intervalMinutes")}
              </Label>
              <Input
                id="automation-interval"
                type="number"
                min={5}
                max={10080}
                value={draft.schedule.interval_minutes ?? 60}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    schedule: {
                      kind: "interval",
                      interval_minutes: Number(event.target.value),
                    },
                  }))
                }
              />
              <p className="text-xs text-muted-foreground">
                {t("automation.intervalHint")}
              </p>
            </div>
          ) : (
            <div className="space-y-2 sm:col-span-2">
              <Label htmlFor="automation-time">
                {t("automation.timeOfDay")}
              </Label>
              <Input
                id="automation-time"
                type="time"
                value={draft.schedule.time_of_day ?? "04:00"}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    schedule: {
                      ...current.schedule,
                      time_of_day: event.target.value,
                    },
                  }))
                }
              />
            </div>
          )}

          {draft.schedule.kind === "weekly" ? (
            <fieldset className="space-y-3 sm:col-span-2">
              <legend className="text-sm font-medium">
                {t("automation.weekdays")}
              </legend>
              <div className="grid grid-cols-4 gap-2 sm:grid-cols-7">
                {weekdays.map((weekday) => {
                  const checked = draft.schedule.weekdays?.includes(weekday);
                  return (
                    <label
                      key={weekday}
                      className="flex items-center gap-2 rounded-md border px-2.5 py-2 text-xs"
                    >
                      <Checkbox
                        checked={checked}
                        onCheckedChange={(value) =>
                          toggleWeekday(weekday, value === true)
                        }
                      />
                      {t(`automation.weekday.${weekday}`)}
                    </label>
                  );
                })}
              </div>
            </fieldset>
          ) : null}

          {needsMessage ? (
            <div className="space-y-2 sm:col-span-2">
              <Label htmlFor="automation-message">
                {draft.action === "broadcast"
                  ? t("automation.broadcastMessage")
                  : t("automation.shutdownMessage")}
              </Label>
              <Textarea
                id="automation-message"
                rows={3}
                maxLength={500}
                value={draft.parameters.message ?? ""}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    parameters: {
                      ...current.parameters,
                      message: event.target.value,
                    },
                  }))
                }
              />
            </div>
          ) : null}

          {needsDelay ? (
            <div className="space-y-2">
              <Label htmlFor="automation-delay">
                {t("automation.shutdownDelay")}
              </Label>
              <Input
                id="automation-delay"
                type="number"
                min={1}
                max={300}
                value={draft.parameters.delay_seconds ?? 10}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    parameters: {
                      ...current.parameters,
                      delay_seconds: Number(event.target.value),
                    },
                  }))
                }
              />
            </div>
          ) : null}

          <div className="flex items-center justify-between gap-4 rounded-md border bg-muted/35 px-4 py-3 sm:col-span-2">
            <div>
              <p className="text-sm font-medium">{t("automation.enabled")}</p>
              <p className="mt-0.5 text-xs text-muted-foreground">
                {t("automation.enabledHint")}
              </p>
            </div>
            <Switch
              checked={draft.enabled}
              onCheckedChange={(enabled) =>
                setDraft((current) => ({ ...current, enabled }))
              }
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("action.cancel")}
          </Button>
          <Button
            disabled={!valid || pending}
            onClick={() => onSubmit(cloneTask(draft))}
          >
            {pending ? <LoaderCircle className="animate-spin" /> : <Save />}
            {editing ? t("action.saveChanges") : t("automation.taskCreate")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
