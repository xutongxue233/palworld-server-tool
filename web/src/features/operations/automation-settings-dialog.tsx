import { useMemo, useState } from "react";
import {
  BellRing,
  LoaderCircle,
  Save,
  ShieldCheck,
  Trash2,
} from "lucide-react";

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
import { Separator } from "@/components/ui/separator";
import { useI18n } from "@/lib/i18n";
import type {
  AutomationNotificationEvent,
  AutomationNotificationProvider,
  AutomationSettings,
  AutomationSettingsUpdate,
} from "@/types/api";

const notificationEvents: AutomationNotificationEvent[] = [
  "task.succeeded",
  "task.failed",
  "server.started",
  "server.stopped",
  "server.restarted",
  "watchdog.unhealthy",
  "watchdog.recovered",
  "watchdog.recovery_failed",
];

function createDraft(settings: AutomationSettings): AutomationSettingsUpdate {
  return {
    watchdog: { ...settings.watchdog },
    notification: {
      enabled: settings.notification.enabled,
      provider: settings.notification.provider,
      webhook_url: "",
      clear_webhook: false,
      secret: "",
      clear_secret: false,
      events: [...settings.notification.events],
      timeout_seconds: settings.notification.timeout_seconds,
    },
  };
}

export function AutomationSettingsDialog({
  open,
  settings,
  pending,
  onOpenChange,
  onSubmit,
}: {
  open: boolean;
  settings: AutomationSettings;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (update: AutomationSettingsUpdate) => void;
}) {
  const { t } = useI18n();
  const [draft, setDraft] = useState(() => createDraft(settings));

  const webhookWillExist =
    !draft.notification.clear_webhook &&
    (settings.notification.webhook_configured ||
      Boolean(draft.notification.webhook_url?.trim()));
  const valid = useMemo(
    () => !draft.notification.enabled || webhookWillExist,
    [draft.notification.enabled, webhookWillExist],
  );

  const toggleEvent = (
    event: AutomationNotificationEvent,
    checked: boolean,
  ) => {
    setDraft((current) => {
      const events = new Set(current.notification.events);
      if (checked) events.add(event);
      else events.delete(event);
      return {
        ...current,
        notification: {
          ...current.notification,
          events: notificationEvents.filter((candidate) =>
            events.has(candidate),
          ),
        },
      };
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t("automation.settingsTitle")}</DialogTitle>
          <DialogDescription>
            {t("automation.settingsDescription")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-6 py-2">
          <section className="space-y-4">
            <div className="flex items-start gap-3">
              <div className="flex size-9 shrink-0 items-center justify-center rounded-md border border-primary/25 bg-primary/10 text-primary">
                <ShieldCheck className="size-4" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h3 className="text-sm font-semibold">
                      {t("automation.watchdog")}
                    </h3>
                    <p className="mt-1 text-xs leading-5 text-muted-foreground">
                      {t("automation.watchdogDescription")}
                    </p>
                  </div>
                  <Switch
                    checked={draft.watchdog.enabled}
                    onCheckedChange={(enabled) =>
                      setDraft((current) => ({
                        ...current,
                        watchdog: { ...current.watchdog, enabled },
                      }))
                    }
                  />
                </div>
              </div>
            </div>

            <div className="grid gap-4 rounded-md border bg-muted/25 p-4 sm:grid-cols-2 lg:grid-cols-3">
              <div className="flex items-center justify-between gap-3 sm:col-span-2 lg:col-span-3">
                <div>
                  <Label>{t("automation.desiredRunning")}</Label>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t("automation.desiredRunningHint")}
                  </p>
                </div>
                <Switch
                  checked={draft.watchdog.desired_running}
                  disabled={!draft.watchdog.enabled}
                  onCheckedChange={(desired_running) =>
                    setDraft((current) => ({
                      ...current,
                      watchdog: { ...current.watchdog, desired_running },
                    }))
                  }
                />
              </div>

              <NumberSelect
                label={t("automation.checkInterval")}
                value={draft.watchdog.check_interval_seconds}
                values={[15, 30, 60, 120, 300]}
                suffix={t("automation.seconds")}
                onChange={(check_interval_seconds) =>
                  setDraft((current) => ({
                    ...current,
                    watchdog: {
                      ...current.watchdog,
                      check_interval_seconds,
                    },
                  }))
                }
              />
              <NumberSelect
                label={t("automation.failureThreshold")}
                value={draft.watchdog.failure_threshold}
                values={[2, 3, 4, 5]}
                suffix={t("automation.times")}
                onChange={(failure_threshold) =>
                  setDraft((current) => ({
                    ...current,
                    watchdog: {
                      ...current.watchdog,
                      failure_threshold,
                    },
                  }))
                }
              />
              <NumberSelect
                label={t("automation.maxRecoveryAttempts")}
                value={draft.watchdog.max_recovery_attempts}
                values={[1, 2, 3, 5]}
                suffix={t("automation.times")}
                onChange={(max_recovery_attempts) =>
                  setDraft((current) => ({
                    ...current,
                    watchdog: {
                      ...current.watchdog,
                      max_recovery_attempts,
                    },
                  }))
                }
              />
              <NumberSelect
                label={t("automation.restartCooldown")}
                value={draft.watchdog.restart_cooldown_seconds}
                values={[30, 60, 120, 300, 600]}
                suffix={t("automation.seconds")}
                onChange={(restart_cooldown_seconds) =>
                  setDraft((current) => ({
                    ...current,
                    watchdog: {
                      ...current.watchdog,
                      restart_cooldown_seconds,
                    },
                  }))
                }
              />
              <NumberSelect
                label={t("automation.startupGrace")}
                value={draft.watchdog.startup_grace_seconds}
                values={[30, 60, 90, 180, 300]}
                suffix={t("automation.seconds")}
                onChange={(startup_grace_seconds) =>
                  setDraft((current) => ({
                    ...current,
                    watchdog: {
                      ...current.watchdog,
                      startup_grace_seconds,
                    },
                  }))
                }
              />
            </div>
          </section>

          <Separator />

          <section className="space-y-4">
            <div className="flex items-start gap-3">
              <div className="flex size-9 shrink-0 items-center justify-center rounded-md border border-[var(--signal)]/30 bg-[var(--signal)]/10 text-[var(--signal)]">
                <BellRing className="size-4" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h3 className="text-sm font-semibold">
                      {t("automation.notifications")}
                    </h3>
                    <p className="mt-1 text-xs leading-5 text-muted-foreground">
                      {t("automation.notificationsDescription")}
                    </p>
                  </div>
                  <Switch
                    checked={draft.notification.enabled}
                    onCheckedChange={(enabled) =>
                      setDraft((current) => ({
                        ...current,
                        notification: { ...current.notification, enabled },
                      }))
                    }
                  />
                </div>
              </div>
            </div>

            <div className="grid gap-4 rounded-md border bg-muted/25 p-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label>{t("automation.provider")}</Label>
                <Select
                  value={draft.notification.provider}
                  onValueChange={(value: AutomationNotificationProvider) =>
                    setDraft((current) => ({
                      ...current,
                      notification: {
                        ...current.notification,
                        provider: value,
                      },
                    }))
                  }
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="generic">
                      {t("automation.provider.generic")}
                    </SelectItem>
                    <SelectItem value="discord">
                      {t("automation.provider.discord")}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <NumberSelect
                label={t("automation.notificationTimeout")}
                value={draft.notification.timeout_seconds}
                values={[5, 10, 20, 30]}
                suffix={t("automation.seconds")}
                onChange={(timeout_seconds) =>
                  setDraft((current) => ({
                    ...current,
                    notification: {
                      ...current.notification,
                      timeout_seconds,
                    },
                  }))
                }
              />

              <div className="space-y-2 sm:col-span-2">
                <div className="flex items-center justify-between gap-3">
                  <Label htmlFor="automation-webhook">
                    {t("automation.webhookUrl")}
                  </Label>
                  {settings.notification.webhook_configured &&
                  !draft.notification.clear_webhook ? (
                    <Button
                      type="button"
                      size="xs"
                      variant="ghost"
                      onClick={() =>
                        setDraft((current) => ({
                          ...current,
                          notification: {
                            ...current.notification,
                            webhook_url: "",
                            clear_webhook: true,
                            enabled: false,
                          },
                        }))
                      }
                    >
                      <Trash2 /> {t("automation.clearWebhook")}
                    </Button>
                  ) : null}
                </div>
                <Input
                  id="automation-webhook"
                  type="url"
                  value={draft.notification.webhook_url ?? ""}
                  disabled={draft.notification.clear_webhook}
                  placeholder={
                    settings.notification.webhook_configured
                      ? t("automation.webhookKeepPlaceholder")
                      : "https://…"
                  }
                  onChange={(event) =>
                    setDraft((current) => ({
                      ...current,
                      notification: {
                        ...current.notification,
                        webhook_url: event.target.value,
                        clear_webhook: false,
                      },
                    }))
                  }
                />
                <p className="text-xs text-muted-foreground">
                  {draft.notification.clear_webhook
                    ? t("automation.webhookWillClear")
                    : settings.notification.webhook_preview ||
                      t("automation.webhookSecurityHint")}
                </p>
              </div>

              {draft.notification.provider === "generic" ? (
                <div className="space-y-2 sm:col-span-2">
                  <div className="flex items-center justify-between gap-3">
                    <Label htmlFor="automation-secret">
                      {t("automation.webhookSecret")}
                    </Label>
                    {settings.notification.secret_configured &&
                    !draft.notification.clear_secret ? (
                      <Button
                        type="button"
                        size="xs"
                        variant="ghost"
                        onClick={() =>
                          setDraft((current) => ({
                            ...current,
                            notification: {
                              ...current.notification,
                              secret: "",
                              clear_secret: true,
                            },
                          }))
                        }
                      >
                        <Trash2 /> {t("automation.clearSecret")}
                      </Button>
                    ) : null}
                  </div>
                  <Input
                    id="automation-secret"
                    type="password"
                    value={draft.notification.secret ?? ""}
                    disabled={draft.notification.clear_secret}
                    placeholder={
                      settings.notification.secret_configured
                        ? t("automation.secretKeepPlaceholder")
                        : t("automation.secretOptional")
                    }
                    onChange={(event) =>
                      setDraft((current) => ({
                        ...current,
                        notification: {
                          ...current.notification,
                          secret: event.target.value,
                          clear_secret: false,
                        },
                      }))
                    }
                  />
                </div>
              ) : null}

              <fieldset className="space-y-3 sm:col-span-2">
                <legend className="text-sm font-medium">
                  {t("automation.notificationEvents")}
                </legend>
                <div className="grid gap-2 sm:grid-cols-2">
                  {notificationEvents.map((event) => (
                    <label
                      key={event}
                      className="flex items-center gap-2.5 rounded-md border bg-background/70 px-3 py-2.5 text-xs"
                    >
                      <Checkbox
                        checked={draft.notification.events.includes(event)}
                        onCheckedChange={(value) =>
                          toggleEvent(event, value === true)
                        }
                      />
                      {t(`automation.event.${event}`)}
                    </label>
                  ))}
                </div>
              </fieldset>
            </div>
          </section>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("action.cancel")}
          </Button>
          <Button disabled={!valid || pending} onClick={() => onSubmit(draft)}>
            {pending ? <LoaderCircle className="animate-spin" /> : <Save />}
            {t("action.saveChanges")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function NumberSelect({
  label,
  value,
  values,
  suffix,
  onChange,
}: {
  label: string;
  value: number;
  values: number[];
  suffix: string;
  onChange: (value: number) => void;
}) {
  const options = values.includes(value)
    ? values
    : [...values, value].sort((left, right) => left - right);
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      <Select
        value={String(value)}
        onValueChange={(next) => onChange(Number(next))}
      >
        <SelectTrigger className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option} value={String(option)}>
              {option} {suffix}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
