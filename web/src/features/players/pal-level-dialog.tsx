import { useState } from "react";
import { ArrowRight, CheckCircle2, LoaderCircle, Save } from "lucide-react";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { queryKeys } from "@/hooks/use-server-data";
import { api, ApiError, getApiErrorCode, getApiErrorMessage } from "@/lib/api";
import { getPalImage, getPalName } from "@/lib/game-data";
import { useI18n } from "@/lib/i18n";
import type { Pal, PalLevelMutation } from "@/types/api";

import { OfflineSaveConfirmation } from "@/features/players/offline-save-confirmation";

const MAX_PAL_LEVEL = 80;

function ValueCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-background px-3 py-2.5">
      <p className="text-[11px] text-muted-foreground">{label}</p>
      <p className="font-data mt-1 text-sm font-semibold">{value}</p>
    </div>
  );
}

export function PalLevelDialog({
  open,
  onOpenChange,
  onUpdated,
  playerUid,
  pal,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onUpdated: (mutation: PalLevelMutation) => void;
  playerUid: string;
  pal: Pal;
}) {
  const { locale, t } = useI18n();
  const queryClient = useQueryClient();
  const [level, setLevel] = useState(pal.level);
  const [serverStopped, setServerStopped] = useState(false);
  const [result, setResult] = useState<PalLevelMutation | null>(null);

  const speciesName = getPalName(pal.type, locale);
  const currentName = pal.nickname || speciesName;
  const currentMaxHp = pal.max_hp ?? 0;
  const validLevel =
    Number.isInteger(level) && level >= 1 && level <= MAX_PAL_LEVEL;
  const hasChanges = level !== pal.level;
  const numberFormat = new Intl.NumberFormat(locale);
  const formatNumber = (value: number) => numberFormat.format(value);
  const formatHealth = (value: number) =>
    numberFormat.format(Math.round(value / 1000));

  const mutation = useMutation({
    mutationFn: () => {
      if (!pal.instance_id) throw new Error("Pal instance ID is unavailable");
      return api.editPalLevel(
        playerUid,
        pal.instance_id,
        level,
        pal.nickname,
        pal.level,
        pal.exp,
        pal.hp,
        currentMaxHp,
      );
    },
    onSuccess: async (response) => {
      setResult(response.level);
      onUpdated(response.level);
      toast.success(
        t("message.palLevelUpdated", { level: response.level.level_after }),
        {
          description: t("delivery.backupCreated", {
            name: response.backup.path,
          }),
        },
      );
      if (response.sync_error) {
        toast.warning(t("message.palLevelRefreshFailed"));
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
    if (mutation.isPending) return;
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="sm:max-w-lg">
        {result ? (
          <div className="grid gap-5">
            <DialogHeader>
              <DialogTitle>{t("palLevelEdit.resultTitle")}</DialogTitle>
              <DialogDescription>
                {t("palLevelEdit.target", { name: currentName })}
              </DialogDescription>
            </DialogHeader>

            <div className="flex items-center gap-3 border-y py-4">
              <CheckCircle2 className="size-5 shrink-0 text-[var(--success)]" />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">
                  {t("palLevelEdit.resultSummary")}
                </p>
                <div className="font-data mt-1 flex items-center gap-2 text-lg font-semibold">
                  <span>Lv.{result.level_before}</span>
                  <ArrowRight className="size-4 text-muted-foreground" />
                  <span className="text-primary">Lv.{result.level_after}</span>
                </div>
              </div>
            </div>

            <div>
              <p className="mb-2 text-xs font-medium text-muted-foreground">
                {t("palLevelEdit.appliedValues")}
              </p>
              <div className="grid grid-cols-2 gap-px overflow-hidden rounded-md border bg-border sm:grid-cols-4">
                <ValueCell
                  label={t("players.level")}
                  value={formatNumber(result.level_after)}
                />
                <ValueCell
                  label={t("palLevelEdit.exp")}
                  value={formatNumber(result.exp_after)}
                />
                <ValueCell
                  label={t("pal.hp")}
                  value={formatHealth(result.hp_after)}
                />
                <ValueCell
                  label={t("palLevelEdit.maxHp")}
                  value={formatHealth(result.max_hp_after)}
                />
              </div>
              {result.max_hp_created ? (
                <p className="mt-2 text-xs leading-5 text-muted-foreground">
                  {t("palLevelEdit.maxHpCreated")}
                </p>
              ) : null}
            </div>

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
              if (
                pal.instance_id &&
                validLevel &&
                hasChanges &&
                serverStopped
              ) {
                mutation.mutate();
              }
            }}
          >
            <DialogHeader>
              <DialogTitle>{t("palLevelEdit.title")}</DialogTitle>
              <DialogDescription>
                {t("palLevelEdit.target", { name: currentName })}
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

            <div>
              <p className="mb-2 text-xs font-medium text-muted-foreground">
                {t("palLevelEdit.currentValues")}
              </p>
              <div className="grid grid-cols-2 gap-px overflow-hidden rounded-md border bg-border sm:grid-cols-4">
                <ValueCell
                  label={t("players.level")}
                  value={formatNumber(pal.level)}
                />
                <ValueCell
                  label={t("palLevelEdit.exp")}
                  value={formatNumber(pal.exp)}
                />
                <ValueCell label={t("pal.hp")} value={formatHealth(pal.hp)} />
                <ValueCell
                  label={t("palLevelEdit.maxHp")}
                  value={currentMaxHp > 0 ? formatHealth(currentMaxHp) : "--"}
                />
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="pal-level-edit-level">
                {t("palLevelEdit.newLevel")}
              </Label>
              <Input
                id="pal-level-edit-level"
                type="number"
                min={1}
                max={MAX_PAL_LEVEL}
                inputMode="numeric"
                value={level}
                disabled={mutation.isPending}
                onChange={(event) => setLevel(Number(event.target.value))}
              />
              <p className="text-xs leading-5 text-muted-foreground">
                {t("palLevelEdit.recalculateHint")}
              </p>
            </div>

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
                disabled={
                  !pal.instance_id ||
                  !validLevel ||
                  !hasChanges ||
                  !serverStopped ||
                  mutation.isPending
                }
              >
                {mutation.isPending ? (
                  <LoaderCircle className="animate-spin" />
                ) : (
                  <Save />
                )}
                {t("action.saveChanges")}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
