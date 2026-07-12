import { useState } from "react";
import {
  ArrowRight,
  CheckCircle2,
  HeartPulse,
  LoaderCircle,
} from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { queryKeys } from "@/hooks/use-server-data";
import { api, ApiError, getApiErrorCode, getApiErrorMessage } from "@/lib/api";
import { getPalImage, getPalName } from "@/lib/game-data";
import { useI18n } from "@/lib/i18n";
import type { Pal, PalHealthMutation } from "@/types/api";

import { OfflineSaveConfirmation } from "@/features/players/offline-save-confirmation";

function HealthChange({ before, after }: { before: string; after: string }) {
  const { t } = useI18n();

  return (
    <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-3 border-y py-4">
      <div className="min-w-0">
        <p className="text-xs text-muted-foreground">
          {t("palHealthRestore.currentHp")}
        </p>
        <p className="font-data mt-1 text-xl font-semibold">{before}</p>
      </div>
      <ArrowRight className="size-4 text-muted-foreground" />
      <div className="min-w-0 text-right">
        <p className="text-xs text-muted-foreground">
          {t("palHealthRestore.maxHp")}
        </p>
        <p className="font-data mt-1 text-xl font-semibold text-[var(--success)]">
          {after}
        </p>
      </div>
    </div>
  );
}

export function PalHealthDialog({
  open,
  onOpenChange,
  onUpdated,
  playerUid,
  pal,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onUpdated: (mutation: PalHealthMutation) => void;
  playerUid: string;
  pal: Pal;
}) {
  const { locale, t } = useI18n();
  const queryClient = useQueryClient();
  const [serverStopped, setServerStopped] = useState(false);
  const [result, setResult] = useState<PalHealthMutation | null>(null);

  const speciesName = getPalName(pal.type, locale);
  const currentName = pal.nickname || speciesName;
  const canRestore =
    Boolean(pal.instance_id) && pal.max_hp > 0 && pal.hp < pal.max_hp;
  const numberFormat = new Intl.NumberFormat(locale);
  const formatHealth = (value: number) =>
    numberFormat.format(Math.round(value / 1000));

  const mutation = useMutation({
    mutationFn: () => {
      if (!pal.instance_id) throw new Error("Pal instance ID is unavailable");
      return api.restorePalHealth(
        playerUid,
        pal.instance_id,
        pal.nickname,
        pal.level,
        pal.exp,
        pal.hp,
        pal.max_hp,
      );
    },
    onSuccess: async (response) => {
      setResult(response.health);
      onUpdated(response.health);
      toast.success(t("message.palHealthRestored"), {
        description: t("delivery.backupCreated", {
          name: response.backup.path,
        }),
      });
      if (response.sync_error) {
        toast.warning(t("message.palHealthRefreshFailed"));
      }

      const refreshes = [
        queryClient.invalidateQueries({ queryKey: queryKeys.backups() }),
      ];
      if (!response.sync_error) {
        refreshes.push(
          queryClient.invalidateQueries({
            queryKey: queryKeys.player(playerUid),
          }),
        );
      }
      await Promise.all(refreshes);
    },
    onError: (error) => {
      const code = getApiErrorCode(error);
      if (code === "game_server_running") {
        toast.error(t("delivery.serverRunning"));
        return;
      }
      if (code === "save_source_changed") {
        toast.error(t("delivery.saveChanged"));
        return;
      }
      if (code === "game_server_status_unknown") {
        toast.error(t("delivery.serverStatusUnknown"));
        return;
      }
      if (
        code === "unsupported_save_source" ||
        (error instanceof ApiError && error.status === 422)
      ) {
        toast.error(t("delivery.localOnly"));
        return;
      }
      toast.error(getApiErrorMessage(error));
    },
  });

  const setOpen = (nextOpen: boolean) => {
    if (mutation.isPending && !result) return;
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-md">
        {result ? (
          <div className="grid gap-5">
            <DialogHeader>
              <DialogTitle>{t("palHealthRestore.resultTitle")}</DialogTitle>
              <DialogDescription>
                {t("palHealthRestore.target", { name: currentName })}
              </DialogDescription>
            </DialogHeader>

            <div className="flex items-center gap-3">
              <CheckCircle2 className="size-5 shrink-0 text-[var(--success)]" />
              <p className="text-sm font-medium">
                {t("palHealthRestore.resultSummary")}
              </p>
            </div>

            <HealthChange
              before={formatHealth(result.hp_before)}
              after={formatHealth(result.hp_after)}
            />

            <DialogFooter>
              <Button type="button" onClick={() => setOpen(false)}>
                {t("action.close")}
              </Button>
            </DialogFooter>
          </div>
        ) : (
          <form
            className="grid gap-5"
            onSubmit={(event) => {
              event.preventDefault();
              if (canRestore && serverStopped) mutation.mutate();
            }}
          >
            <DialogHeader>
              <DialogTitle>{t("palHealthRestore.title")}</DialogTitle>
              <DialogDescription>
                {t("palHealthRestore.target", { name: currentName })}
              </DialogDescription>
            </DialogHeader>

            <div className="flex min-h-16 items-center gap-3 border-y py-3">
              <img
                src={getPalImage(pal.type, pal.is_boss)}
                alt={currentName}
                className="size-12 shrink-0 rounded-md border bg-muted object-contain p-1"
              />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{currentName}</p>
                <p className="mt-0.5 truncate text-xs text-muted-foreground">
                  {speciesName}
                </p>
              </div>
              <span className="font-data shrink-0 text-sm font-semibold">
                Lv.{pal.level}
              </span>
            </div>

            <HealthChange
              before={formatHealth(pal.hp)}
              after={formatHealth(pal.max_hp)}
            />

            <OfflineSaveConfirmation
              checked={serverStopped}
              disabled={mutation.isPending}
              onCheckedChange={setServerStopped}
            />

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                disabled={mutation.isPending}
                onClick={() => setOpen(false)}
              >
                {t("action.cancel")}
              </Button>
              <Button
                type="submit"
                disabled={!canRestore || !serverStopped || mutation.isPending}
              >
                {mutation.isPending ? (
                  <LoaderCircle className="animate-spin" />
                ) : (
                  <HeartPulse />
                )}
                {t("action.restorePalHealth")}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
