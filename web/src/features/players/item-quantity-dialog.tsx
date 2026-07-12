import { useState } from "react";
import { Box, LoaderCircle, Save, Trash2 } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
import { getItemImage, getItemMetadata } from "@/lib/game-data";
import { useI18n } from "@/lib/i18n";
import type { InventoryContainer, PlayerItem } from "@/types/api";

import { OfflineSaveConfirmation } from "@/features/players/offline-save-confirmation";

export function ItemQuantityDialog({
  open,
  onOpenChange,
  playerUid,
  playerName,
  item,
  container,
  containerLabel,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  playerUid: string;
  playerName: string;
  item: PlayerItem | null;
  container: InventoryContainer;
  containerLabel: string;
}) {
  const { locale, t } = useI18n();
  const queryClient = useQueryClient();
  const [quantity, setQuantity] = useState(item?.StackCount ?? 1);
  const [serverStopped, setServerStopped] = useState(false);

  const metadata = item ? getItemMetadata(item.ItemId, locale) : null;
  const image = item ? getItemImage(item.ItemId) : "";
  const mutation = useMutation({
    mutationFn: (nextQuantity: number) => {
      if (!item) throw new Error("Inventory item is unavailable");
      return api.setPlayerItemQuantity(
        playerUid,
        container,
        item.SlotIndex,
        item.ItemId,
        item.StackCount,
        item.DynamicId ?? "00000000-0000-0000-0000-000000000000",
        nextQuantity,
      );
    },
    onSuccess: async (result) => {
      toast.success(
        result.mutation.removed
          ? t("message.itemRemoved", {
              item: metadata?.name ?? result.mutation.item_id,
            })
          : t("message.itemQuantityUpdated", {
              item: metadata?.name ?? result.mutation.item_id,
              quantity: result.mutation.after,
            }),
        {
          description: t("delivery.backupCreated", {
            name: result.backup.path,
          }),
        },
      );
      if (result.sync_error) {
        toast.warning(t("message.itemEditRefreshFailed"));
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
    if (mutation.isPending) return;
    onOpenChange(nextOpen);
  };

  if (!item) return null;

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="sm:max-w-lg">
        <form
          className="grid gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            if (
              quantity >= 1 &&
              quantity <= 999999 &&
              quantity !== item.StackCount &&
              serverStopped
            ) {
              mutation.mutate(quantity);
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("inventoryEdit.title")}</DialogTitle>
            <DialogDescription>
              {t("delivery.target", { name: playerName })}
            </DialogDescription>
          </DialogHeader>

          <div className="flex min-h-16 items-center gap-3 border-y py-3">
            {image ? (
              <img
                src={image}
                alt={metadata?.name ?? item.ItemId}
                className="size-11 shrink-0 rounded-md border bg-muted object-contain p-1"
              />
            ) : (
              <div className="flex size-11 shrink-0 items-center justify-center rounded-md border bg-muted">
                <Box className="size-4 text-muted-foreground" />
              </div>
            )}
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">
                {metadata?.name ?? item.ItemId}
              </p>
              <p className="font-data mt-0.5 truncate text-[10px] text-muted-foreground">
                {item.ItemId}
              </p>
            </div>
            <div className="shrink-0 text-right">
              <p className="text-xs text-muted-foreground">{containerLabel}</p>
              <p className="font-data mt-0.5 text-xs">
                {t("inventoryEdit.slot", { slot: item.SlotIndex })}
              </p>
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="inventory-edit-quantity">
              {t("inventoryEdit.newQuantity")}
            </Label>
            <Input
              id="inventory-edit-quantity"
              type="number"
              min={1}
              max={999999}
              inputMode="numeric"
              value={quantity}
              disabled={mutation.isPending}
              onChange={(event) => setQuantity(Number(event.target.value))}
            />
            <p className="text-xs text-muted-foreground">
              {t("inventoryEdit.currentQuantity", {
                quantity: item.StackCount,
              })}
            </p>
          </div>

          <OfflineSaveConfirmation
            checked={serverStopped}
            disabled={mutation.isPending}
            onCheckedChange={setServerStopped}
          />

          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
            <Button
              type="button"
              variant="destructive"
              disabled={!serverStopped || mutation.isPending}
              onClick={() => mutation.mutate(0)}
            >
              {mutation.isPending && mutation.variables === 0 ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <Trash2 />
              )}
              {t("action.deleteItem")}
            </Button>
            <div className="flex flex-col-reverse gap-2 sm:flex-row">
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
                  quantity < 1 ||
                  quantity > 999999 ||
                  quantity === item.StackCount ||
                  !serverStopped ||
                  mutation.isPending
                }
              >
                {mutation.isPending && mutation.variables !== 0 ? (
                  <LoaderCircle className="animate-spin" />
                ) : (
                  <Save />
                )}
                {t("action.saveChanges")}
              </Button>
            </div>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
