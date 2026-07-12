import { useState } from "react";
import {
  ArrowRight,
  CheckCircle2,
  FileCheck2,
  LoaderCircle,
  Map as MapIcon,
  TriangleAlert,
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
import { OfflineSaveConfirmation } from "@/features/players/offline-save-confirmation";
import { queryKeys } from "@/hooks/use-server-data";
import { api, ApiError, getApiErrorCode, getApiErrorMessage } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import type {
  Player,
  PlayerMapProgress,
  PlayerMapProgressMutation,
  UnlockPlayerMapProgressResult,
} from "@/types/api";

type MapProgressChange = Pick<
  PlayerMapProgressMutation,
  | "fast_travel_before"
  | "fast_travel_after"
  | "fast_travel_total"
  | "areas_before"
  | "areas_after"
  | "areas_total"
  | "world_maps_before"
  | "world_maps_after"
  | "world_maps_total"
>;

function ProgressChange({
  label,
  before,
  after,
  total,
}: {
  label: string;
  before: number;
  after: number;
  total: number;
}) {
  const denominator = Math.max(total, 1);
  const beforeWidth = Math.min(100, Math.max(0, (before / denominator) * 100));
  const afterWidth = Math.min(100, Math.max(0, (after / denominator) * 100));

  return (
    <div className="grid gap-2">
      <div className="flex min-w-0 items-center justify-between gap-3">
        <p className="truncate text-sm text-muted-foreground">{label}</p>
        <div className="font-data flex shrink-0 items-center gap-1.5 text-sm">
          <span>{before}</span>
          <ArrowRight className="size-3.5 text-muted-foreground" />
          <span className="font-semibold text-[var(--success)]">
            {after}/{total}
          </span>
        </div>
      </div>
      <div className="relative h-1.5 overflow-hidden rounded-full bg-muted">
        <div
          className="absolute inset-y-0 left-0 bg-[var(--success)] opacity-25"
          style={{ width: `${afterWidth}%` }}
        />
        <div
          className="absolute inset-y-0 left-0 bg-[var(--success)]"
          style={{ width: `${beforeWidth}%` }}
        />
      </div>
    </div>
  );
}

function ProgressList({ progress }: { progress: MapProgressChange }) {
  const { t } = useI18n();

  return (
    <div className="grid gap-4 border-y py-4">
      <ProgressChange
        label={t("mapUnlock.fastTravel")}
        before={progress.fast_travel_before}
        after={progress.fast_travel_after}
        total={progress.fast_travel_total}
      />
      <ProgressChange
        label={t("mapUnlock.areas")}
        before={progress.areas_before}
        after={progress.areas_after}
        total={progress.areas_total}
      />
      <ProgressChange
        label={t("mapUnlock.worldMaps")}
        before={progress.world_maps_before}
        after={progress.world_maps_after}
        total={progress.world_maps_total}
      />
    </div>
  );
}

function pendingProgress(progress: PlayerMapProgress): MapProgressChange {
  return {
    fast_travel_before: progress.fast_travel_unlocked,
    fast_travel_after: progress.fast_travel_total,
    fast_travel_total: progress.fast_travel_total,
    areas_before: progress.areas_found,
    areas_after: progress.areas_total,
    areas_total: progress.areas_total,
    world_maps_before: progress.world_maps_unlocked,
    world_maps_after: progress.world_maps_total,
    world_maps_total: progress.world_maps_total,
  };
}

export function PlayerMapUnlockDialog({
  open,
  onOpenChange,
  playerUid,
  playerName,
  progress,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  playerUid: string;
  playerName: string;
  progress: PlayerMapProgress;
}) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [serverStopped, setServerStopped] = useState(false);
  const [result, setResult] = useState<UnlockPlayerMapProgressResult | null>(
    null,
  );

  const mutation = useMutation({
    mutationFn: () =>
      api.unlockPlayerMapProgress(playerUid, progress.progress_digest),
    onSuccess: async (response) => {
      const updated = response.map_progress;
      setResult(response);
      queryClient.setQueryData<Player>(
        queryKeys.player(playerUid),
        (current) =>
          current
            ? {
                ...current,
                map_progress: {
                  fast_travel_unlocked: updated.fast_travel_after,
                  fast_travel_total: updated.fast_travel_total,
                  areas_found: updated.areas_after,
                  areas_total: updated.areas_total,
                  world_maps_unlocked: updated.world_maps_after,
                  world_maps_total: updated.world_maps_total,
                  progress_digest: updated.progress_digest_after,
                  game_version: updated.game_version,
                },
              }
            : current,
      );

      toast.success(t("message.mapProgressUnlocked"), {
        description: t("delivery.backupCreated", {
          name: response.backup.path,
        }),
      });
      if (response.sync_error) {
        toast.warning(t("message.mapProgressRefreshFailed"));
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

  const preview = pendingProgress(progress);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-md">
        {result ? (
          <div className="grid gap-5">
            <DialogHeader>
              <DialogTitle>{t("mapUnlock.resultTitle")}</DialogTitle>
              <DialogDescription>
                {t("delivery.target", { name: playerName })}
              </DialogDescription>
            </DialogHeader>

            <div className="flex items-start gap-3">
              <CheckCircle2 className="mt-0.5 size-5 shrink-0 text-[var(--success)]" />
              <div className="min-w-0">
                <p className="text-sm font-medium">
                  {t("mapUnlock.resultSummary")}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("mapUnlock.gameVersion", {
                    version: result.map_progress.game_version,
                  })}
                </p>
              </div>
            </div>

            <ProgressList progress={result.map_progress} />

            <div className="grid gap-3 text-sm">
              <div className="flex items-start gap-2.5">
                <FileCheck2 className="mt-0.5 size-4 shrink-0 text-[var(--success)]" />
                <div className="min-w-0">
                  <p className="font-medium">{t("mapUnlock.backup")}</p>
                  <p className="font-data mt-0.5 break-all text-xs text-muted-foreground">
                    {result.backup.path}
                  </p>
                </div>
              </div>
              {result.map_progress.created_fields.length ? (
                <p className="text-xs text-muted-foreground">
                  {t("mapUnlock.createdFields", {
                    count: result.map_progress.created_fields.length,
                  })}
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
              if (serverStopped && progress.progress_digest) mutation.mutate();
            }}
          >
            <DialogHeader>
              <DialogTitle>{t("mapUnlock.title")}</DialogTitle>
              <DialogDescription>
                {t("delivery.target", { name: playerName })}
              </DialogDescription>
            </DialogHeader>

            <div className="border-l-2 border-destructive pl-3">
              <div className="flex items-center gap-2 text-sm font-semibold text-destructive">
                <TriangleAlert className="size-4" />
                {t("mapUnlock.warningTitle")}
              </div>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {t("mapUnlock.warningDescription")}
              </p>
            </div>

            <div>
              <div className="mb-3 flex items-center justify-between gap-3">
                <p className="text-sm font-medium">
                  {t("mapUnlock.currentProgress")}
                </p>
                <span className="font-data text-xs text-muted-foreground">
                  {progress.game_version}
                </span>
              </div>
              <ProgressList progress={preview} />
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
                variant="destructive"
                disabled={
                  !serverStopped ||
                  !progress.progress_digest ||
                  mutation.isPending
                }
              >
                {mutation.isPending ? (
                  <LoaderCircle className="animate-spin" />
                ) : (
                  <MapIcon />
                )}
                {t("action.unlockFullMap")}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
