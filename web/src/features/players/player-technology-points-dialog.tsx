import { useState } from "react";
import { Cog, Landmark, LoaderCircle, Save } from "lucide-react";
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

const MAX_TECHNOLOGY_POINTS = 999_999;

export function PlayerTechnologyPointsDialog({
  open,
  onOpenChange,
  playerUid,
  playerName,
  currentTechnologyPoints,
  currentAncientTechnologyPoints,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  playerUid: string;
  playerName: string;
  currentTechnologyPoints: number;
  currentAncientTechnologyPoints: number;
}) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [technologyPoints, setTechnologyPoints] = useState(
    currentTechnologyPoints,
  );
  const [ancientTechnologyPoints, setAncientTechnologyPoints] = useState(
    currentAncientTechnologyPoints,
  );
  const [serverStopped, setServerStopped] = useState(false);

  const valid = [technologyPoints, ancientTechnologyPoints].every(
    (value) =>
      Number.isInteger(value) && value >= 0 && value <= MAX_TECHNOLOGY_POINTS,
  );
  const changed =
    technologyPoints !== currentTechnologyPoints ||
    ancientTechnologyPoints !== currentAncientTechnologyPoints;

  const mutation = useMutation({
    mutationFn: () =>
      api.editPlayerTechnologyPoints(
        playerUid,
        technologyPoints,
        ancientTechnologyPoints,
        currentTechnologyPoints,
        currentAncientTechnologyPoints,
      ),
    onSuccess: async (result) => {
      toast.success(
        t("message.playerTechnologyPointsUpdated", {
          technology: result.technology_points.technology_after,
          ancient: result.technology_points.ancient_after,
        }),
        {
          description: t("delivery.backupCreated", {
            name: result.backup.path,
          }),
        },
      );
      if (result.sync_error) {
        toast.warning(t("message.playerTechnologyPointsRefreshFailed"));
      }
      onOpenChange(false);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.player(playerUid) }),
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
      <DialogContent className="sm:max-w-lg">
        <form
          className="grid gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            if (valid && changed && serverStopped) mutation.mutate();
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("technologyPointsEdit.title")}</DialogTitle>
            <DialogDescription>
              {t("delivery.target", { name: playerName })}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-2 rounded-md border p-4">
              <Cog className="size-5 text-primary" />
              <Label htmlFor="technology-points-edit-value">
                {t("players.technologyPoints")}
              </Label>
              <Input
                id="technology-points-edit-value"
                type="number"
                min={0}
                max={MAX_TECHNOLOGY_POINTS}
                inputMode="numeric"
                value={technologyPoints}
                disabled={mutation.isPending}
                onChange={(event) =>
                  setTechnologyPoints(Number(event.target.value))
                }
              />
              <p className="font-data text-xs text-muted-foreground">
                {t("technologyPointsEdit.current", {
                  points: currentTechnologyPoints,
                })}
              </p>
            </div>
            <div className="grid gap-2 rounded-md border p-4">
              <Landmark className="size-5 text-[var(--signal)]" />
              <Label htmlFor="ancient-technology-points-edit-value">
                {t("players.ancientTechnologyPoints")}
              </Label>
              <Input
                id="ancient-technology-points-edit-value"
                type="number"
                min={0}
                max={MAX_TECHNOLOGY_POINTS}
                inputMode="numeric"
                value={ancientTechnologyPoints}
                disabled={mutation.isPending}
                onChange={(event) =>
                  setAncientTechnologyPoints(Number(event.target.value))
                }
              />
              <p className="font-data text-xs text-muted-foreground">
                {t("technologyPointsEdit.current", {
                  points: currentAncientTechnologyPoints,
                })}
              </p>
            </div>
          </div>

          <p className="text-xs leading-5 text-muted-foreground">
            {t("technologyPointsEdit.hint")}
          </p>
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
              disabled={!valid || !changed || !serverStopped || mutation.isPending}
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
