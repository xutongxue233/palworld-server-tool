import { useMemo, useState } from "react";
import { Box, LoaderCircle, PackagePlus } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { SearchSelect } from "@/components/common/search-select";
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
import { getItemImage, getItemMetadata, getItemOptions } from "@/lib/game-data";
import { useI18n } from "@/lib/i18n";
import { OfflineSaveConfirmation } from "@/features/players/offline-save-confirmation";

export function ItemDeliveryDialog({
  open,
  onOpenChange,
  playerUid,
  playerName,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  playerUid: string;
  playerName: string;
}) {
  const { locale, t } = useI18n();
  const queryClient = useQueryClient();
  const [itemId, setItemId] = useState("");
  const [quantity, setQuantity] = useState(1);
  const [serverStopped, setServerStopped] = useState(false);

  const options = useMemo(
    () =>
      getItemOptions(locale).map((item) => ({
        value: item.key,
        label: item.name,
        description: item.key,
      })),
    [locale],
  );
  const selected = itemId ? getItemMetadata(itemId, locale) : null;
  const selectedImage = itemId ? getItemImage(itemId) : "";

  const reset = () => {
    setItemId("");
    setQuantity(1);
    setServerStopped(false);
  };

  const deliveryMutation = useMutation({
    mutationFn: () => api.givePlayerItem(playerUid, itemId, quantity),
    onSuccess: async (result) => {
      toast.success(
        t("message.itemDelivered", {
          quantity: result.delivery.delivered,
          item: selected?.name ?? result.delivery.item_id,
        }),
        {
          description: t("delivery.backupCreated", {
            name: result.backup.path,
          }),
        },
      );
      if (result.sync_error) {
        toast.warning(t("message.itemDeliveredRefreshFailed"));
      }
      onOpenChange(false);
      reset();
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
    if (deliveryMutation.isPending) return;
    if (!nextOpen) reset();
    onOpenChange(nextOpen);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="sm:max-w-xl">
        <form
          className="grid gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            if (
              itemId &&
              quantity >= 1 &&
              quantity <= 999999 &&
              serverStopped
            ) {
              deliveryMutation.mutate();
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("delivery.title")}</DialogTitle>
            <DialogDescription>
              {t("delivery.target", { name: playerName })}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_120px]">
            <div className="grid gap-2">
              <Label>{t("item.name")}</Label>
              <SearchSelect
                value={itemId}
                options={options}
                placeholder={t("delivery.itemPlaceholder")}
                searchPlaceholder={t("delivery.itemSearch")}
                emptyText={t("message.empty")}
                onValueChange={setItemId}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="delivery-quantity">{t("item.quantity")}</Label>
              <Input
                id="delivery-quantity"
                type="number"
                min={1}
                max={999999}
                inputMode="numeric"
                value={quantity}
                onChange={(event) => setQuantity(Number(event.target.value))}
              />
            </div>
          </div>

          <div className="flex min-h-16 items-center gap-3 border-y py-3">
            {selectedImage ? (
              <img
                src={selectedImage}
                alt={selected?.name ?? itemId}
                className="size-11 shrink-0 rounded-md border bg-muted object-contain p-1"
              />
            ) : (
              <div className="flex size-11 shrink-0 items-center justify-center rounded-md border bg-muted">
                <Box className="size-4 text-muted-foreground" />
              </div>
            )}
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">
                {selected?.name ?? t("delivery.itemPlaceholder")}
              </p>
              <p className="font-data mt-0.5 truncate text-[10px] text-muted-foreground">
                {itemId || "--"}
              </p>
            </div>
          </div>

          <OfflineSaveConfirmation
            checked={serverStopped}
            disabled={deliveryMutation.isPending}
            onCheckedChange={setServerStopped}
          />

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={deliveryMutation.isPending}
              onClick={() => setOpen(false)}
            >
              {t("action.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={
                !itemId ||
                quantity < 1 ||
                quantity > 999999 ||
                !serverStopped ||
                deliveryMutation.isPending
              }
            >
              {deliveryMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <PackagePlus />
              )}
              {t("action.deliverItem")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
