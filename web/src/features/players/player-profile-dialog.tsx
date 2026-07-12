import { useState } from "react";
import { LoaderCircle, Save, UserRound } from "lucide-react";
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
import {
  api,
  ApiError,
  getApiErrorCode,
  getApiErrorMessage,
} from "@/lib/api";
import { useI18n } from "@/lib/i18n";

import { OfflineSaveConfirmation } from "@/features/players/offline-save-confirmation";

const MAX_PLAYER_LEVEL = 80;
const MAX_NICKNAME_LENGTH = 32;

export function PlayerProfileDialog({
  open,
  onOpenChange,
  playerUid,
  currentNickname,
  currentLevel,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  playerUid: string;
  currentNickname: string;
  currentLevel: number;
}) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [nickname, setNickname] = useState(currentNickname);
  const [level, setLevel] = useState(currentLevel);
  const [serverStopped, setServerStopped] = useState(false);

  const normalizedNickname = nickname.trim();
  const nicknameLength = Array.from(normalizedNickname).length;
  const validNickname =
    nicknameLength >= 1 && nicknameLength <= MAX_NICKNAME_LENGTH;
  const validLevel =
    Number.isInteger(level) && level >= 1 && level <= MAX_PLAYER_LEVEL;
  const hasChanges =
    normalizedNickname !== currentNickname || level !== currentLevel;

  const mutation = useMutation({
    mutationFn: () =>
      api.editPlayerProfile(
        playerUid,
        normalizedNickname,
        level,
        currentNickname,
        currentLevel,
      ),
    onSuccess: async (result) => {
      toast.success(
        t("message.playerProfileUpdated", {
          name: result.profile.nickname_after,
          level: result.profile.level_after,
        }),
        {
          description: t("delivery.backupCreated", {
            name: result.backup.path,
          }),
        },
      );
      if (result.sync_error) {
        toast.warning(t("message.playerProfileRefreshFailed"));
      }
      onOpenChange(false);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: queryKeys.player(playerUid),
        }),
        queryClient.invalidateQueries({ queryKey: queryKeys.players }),
        queryClient.invalidateQueries({ queryKey: queryKeys.guilds }),
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
    if (mutation.isPending) return;
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="sm:max-w-lg">
        <form
          className="grid gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            if (validNickname && validLevel && hasChanges && serverStopped) {
              mutation.mutate();
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("profileEdit.title")}</DialogTitle>
            <DialogDescription>
              {t("delivery.target", { name: currentNickname })}
            </DialogDescription>
          </DialogHeader>

          <div className="flex min-h-16 items-center gap-3 border-y py-3">
            <div className="flex size-11 shrink-0 items-center justify-center rounded-md border bg-muted">
              <UserRound className="size-5 text-muted-foreground" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-xs text-muted-foreground">
                {t("profileEdit.currentProfile")}
              </p>
              <p className="mt-0.5 truncate text-sm font-medium">
                {currentNickname}
              </p>
            </div>
            <span className="font-data shrink-0 text-sm font-semibold">
              Lv.{currentLevel}
            </span>
          </div>

          <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_112px]">
            <div className="grid gap-2">
              <Label htmlFor="profile-edit-nickname">
                {t("profileEdit.nickname")}
              </Label>
              <Input
                id="profile-edit-nickname"
                value={nickname}
                maxLength={MAX_NICKNAME_LENGTH}
                autoComplete="off"
                disabled={mutation.isPending}
                onChange={(event) => setNickname(event.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                {t("profileEdit.nicknameLimit")}
              </p>
            </div>
            <div className="grid content-start gap-2">
              <Label htmlFor="profile-edit-level">{t("players.level")}</Label>
              <Input
                id="profile-edit-level"
                type="number"
                min={1}
                max={MAX_PLAYER_LEVEL}
                inputMode="numeric"
                value={level}
                disabled={mutation.isPending}
                onChange={(event) => setLevel(Number(event.target.value))}
              />
              <p className="text-xs text-muted-foreground">
                {t("profileEdit.levelRange")}
              </p>
            </div>
          </div>

          <p className="text-xs leading-5 text-muted-foreground">
            {t("profileEdit.expHint")}
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
              disabled={
                !validNickname ||
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
      </DialogContent>
    </Dialog>
  );
}
