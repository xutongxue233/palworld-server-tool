import { useState } from "react";
import { LoaderCircle, Save } from "lucide-react";
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
import { getPalImage, getPalName } from "@/lib/game-data";
import { useI18n } from "@/lib/i18n";
import type { Pal } from "@/types/api";

import { OfflineSaveConfirmation } from "@/features/players/offline-save-confirmation";

const MAX_NICKNAME_LENGTH = 32;

export function PalNicknameDialog({
  open,
  onOpenChange,
  playerUid,
  pal,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  playerUid: string;
  pal: Pal;
}) {
  const { locale, t } = useI18n();
  const queryClient = useQueryClient();
  const [nickname, setNickname] = useState(pal.nickname);
  const [serverStopped, setServerStopped] = useState(false);

  const normalizedNickname = nickname.trim();
  const nicknameLength = Array.from(normalizedNickname).length;
  const validNickname = nicknameLength <= MAX_NICKNAME_LENGTH;
  const hasChanges = normalizedNickname !== pal.nickname;
  const speciesName = getPalName(pal.type, locale);
  const currentName = pal.nickname || speciesName;

  const mutation = useMutation({
    mutationFn: () => {
      if (!pal.instance_id) throw new Error("Pal instance ID is unavailable");
      return api.renamePal(
        playerUid,
        pal.instance_id,
        normalizedNickname,
        pal.nickname,
        pal.level,
        pal.exp,
      );
    },
    onSuccess: async (result) => {
      toast.success(t("message.palNicknameUpdated"), {
        description: t("delivery.backupCreated", {
          name: result.backup.path,
        }),
      });
      if (result.sync_error) {
        toast.warning(t("message.palNicknameRefreshFailed"));
      }
      onOpenChange(false);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: queryKeys.player(playerUid),
        }),
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
            if (
              pal.instance_id &&
              validNickname &&
              hasChanges &&
              serverStopped
            ) {
              mutation.mutate();
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("palNicknameEdit.title")}</DialogTitle>
            <DialogDescription>
              {t("palNicknameEdit.target", { name: currentName })}
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

          <div className="grid gap-2">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="pal-nickname-edit-name">
                {t("palNicknameEdit.nickname")}
              </Label>
              <span className="font-data text-xs text-muted-foreground">
                {nicknameLength}/{MAX_NICKNAME_LENGTH}
              </span>
            </div>
            <Input
              id="pal-nickname-edit-name"
              value={nickname}
              maxLength={MAX_NICKNAME_LENGTH}
              autoComplete="off"
              disabled={mutation.isPending}
              onChange={(event) => setNickname(event.target.value)}
            />
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
                !validNickname ||
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
