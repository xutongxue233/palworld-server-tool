import { useState } from "react";
import { LoaderCircle, Save, Sparkles } from "lucide-react";
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
import { OfflineSaveConfirmation } from "@/features/players/offline-save-confirmation";
import { queryKeys } from "@/hooks/use-server-data";
import {
  api,
  ApiError,
  getApiErrorCode,
  getApiErrorMessage,
} from "@/lib/api";
import { useI18n } from "@/lib/i18n";

const MAX_UNUSED_STATUS_POINTS = 65_535;

export function PlayerStatPointsDialog({
  open,
  onOpenChange,
  playerUid,
  playerName,
  currentPoints,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  playerUid: string;
  playerName: string;
  currentPoints: number;
}) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [points, setPoints] = useState(currentPoints);
  const [serverStopped, setServerStopped] = useState(false);

  const validPoints =
    Number.isInteger(points) &&
    points >= 0 &&
    points <= MAX_UNUSED_STATUS_POINTS;

  const mutation = useMutation({
    mutationFn: () =>
      api.editPlayerStatPoints(playerUid, points, currentPoints),
    onSuccess: async (result) => {
      toast.success(
        t("message.playerStatPointsUpdated", {
          points: result.stat_points.after,
        }),
        {
          description: t("delivery.backupCreated", {
            name: result.backup.path,
          }),
        },
      );
      if (result.sync_error) {
        toast.warning(t("message.playerStatPointsRefreshFailed"));
      }
      onOpenChange(false);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: queryKeys.player(playerUid),
        }),
        queryClient.invalidateQueries({ queryKey: queryKeys.players }),
        queryClient.invalidateQueries({ queryKey: queryKeys.backups() }),
      ]);
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
    if (!mutation.isPending) onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="sm:max-w-md">
        <form
          className="grid gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            if (validPoints && points !== currentPoints && serverStopped) {
              mutation.mutate();
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("statPointsEdit.title")}</DialogTitle>
            <DialogDescription>
              {t("delivery.target", { name: playerName })}
            </DialogDescription>
          </DialogHeader>

          <div className="flex items-center gap-4 border-y py-4">
            <div className="flex size-11 shrink-0 items-center justify-center rounded-md border bg-muted">
              <Sparkles className="size-5 text-[var(--signal)]" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-xs text-muted-foreground">
                {t("statPointsEdit.current")}
              </p>
              <p className="font-data mt-0.5 text-xl font-semibold">
                {currentPoints}
              </p>
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="stat-points-edit-value">
              {t("statPointsEdit.newValue")}
            </Label>
            <Input
              id="stat-points-edit-value"
              type="number"
              min={0}
              max={MAX_UNUSED_STATUS_POINTS}
              inputMode="numeric"
              value={points}
              disabled={mutation.isPending}
              onChange={(event) => setPoints(Number(event.target.value))}
            />
            <p className="text-xs leading-5 text-muted-foreground">
              {t("statPointsEdit.hint")}
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
                !validPoints ||
                points === currentPoints ||
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
      </DialogContent>
    </Dialog>
  );
}
