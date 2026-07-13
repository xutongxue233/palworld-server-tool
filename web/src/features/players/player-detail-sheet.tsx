import { useMemo, useState } from "react";
import {
  Ban,
  Box,
  Check,
  Copy,
  Crown,
  Cog,
  Hammer,
  HeartPulse,
  Landmark,
  LoaderCircle,
  Map as MapIcon,
  PackagePlus,
  Pencil,
  ShieldCheck,
  ShieldOff,
  Sparkles,
  UserRoundX,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { ErrorState, LoadingState } from "@/components/common/data-state";
import { StatusDot } from "@/components/common/status-dot";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  queryKeys,
  scopedQueryFn,
  useGuilds,
  usePlayer,
} from "@/hooks/use-server-data";
import { api, getApiErrorMessage } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import {
  copyText,
  formatCoordinate,
  formatDateTime,
  isRecentlyOnline,
} from "@/lib/format";
import {
  getItemImage,
  getItemMetadata,
  getPalImage,
  getPalName,
} from "@/lib/game-data";
import { useI18n } from "@/lib/i18n";
import type {
  InventoryContainer,
  Pal,
  PalHealthMutation,
  PalLevelMutation,
  Player,
  PlayerItem,
  PlayerMapProgress,
  WhitelistPlayer,
} from "@/types/api";

import avatarUrl from "@/assets/avatar.webp";
import { PalDetailDialog } from "@/features/players/pal-detail-dialog";
import { PalHealthDialog } from "@/features/players/pal-health-dialog";
import { PalLevelDialog } from "@/features/players/pal-level-dialog";
import { PalNicknameDialog } from "@/features/players/pal-nickname-dialog";
import { ItemDeliveryDialog } from "@/features/players/item-delivery-dialog";
import { ItemQuantityDialog } from "@/features/players/item-quantity-dialog";
import { PlayerProfileDialog } from "@/features/players/player-profile-dialog";
import { PlayerMapUnlockDialog } from "@/features/players/player-map-unlock-dialog";
import { PlayerStatPointsDialog } from "@/features/players/player-stat-points-dialog";
import { PlayerTechnologyPointsDialog } from "@/features/players/player-technology-points-dialog";

type PlayerAction = "kick" | "ban" | "unban";

const inventoryTabs = [
  ["CommonContainerId", "players.common", "main"],
  ["EssentialContainerId", "players.essential", "key"],
  ["WeaponLoadOutContainerId", "players.weapons", "weapons"],
  ["PlayerEquipArmorContainerId", "players.armor", "armor"],
  ["FoodEquipContainerId", "players.food", "food"],
  ["DropSlotContainerId", "players.drop", "drop"],
] as const;

interface SelectedInventoryItem {
  item: PlayerItem;
  container: InventoryContainer;
  containerLabel: string;
}

function isMapProgressComplete(progress?: PlayerMapProgress) {
  return Boolean(
    progress &&
    progress.fast_travel_total > 0 &&
    progress.areas_total > 0 &&
    progress.world_maps_total > 0 &&
    progress.fast_travel_unlocked >= progress.fast_travel_total &&
    progress.areas_found >= progress.areas_total &&
    progress.world_maps_unlocked >= progress.world_maps_total,
  );
}

function MapProgressMetric({
  label,
  value,
  total,
}: {
  label: string;
  value?: number;
  total?: number;
}) {
  return (
    <div className="min-w-0 bg-card px-3 py-3 sm:px-4">
      <p className="truncate text-xs text-muted-foreground">{label}</p>
      <p className="font-data mt-1 text-base font-semibold">
        {value ?? "--"}
        <span className="ml-1 text-xs font-normal text-muted-foreground">
          / {total ?? "--"}
        </span>
      </p>
    </div>
  );
}

function DetailField({
  label,
  value,
  copyable = false,
}: {
  label: string;
  value?: string | number;
  copyable?: boolean;
}) {
  const { t } = useI18n();
  const text = value === undefined || value === "" ? "--" : String(value);
  return (
    <div className="min-w-0 border-b py-3 last:border-b-0">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className="mt-1 flex items-center gap-2">
        <p className="font-data min-w-0 flex-1 truncate text-sm">{text}</p>
        {copyable && text !== "--" ? (
          <Button
            variant="ghost"
            size="icon-xs"
            onClick={() => {
              void copyText(text);
              toast.success(t("message.copied"));
            }}
          >
            <Copy />
            <span className="sr-only">{t("action.copy")}</span>
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function InventoryTable({
  items,
  onEditItem,
}: {
  items: PlayerItem[];
  onEditItem: (item: PlayerItem) => void;
}) {
  const { locale, t } = useI18n();
  if (!items?.length) {
    return (
      <div className="flex min-h-36 items-center justify-center text-sm text-muted-foreground">
        {t("players.noItems")}
      </div>
    );
  }
  return (
    <div className="overflow-x-auto rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("item.name")}</TableHead>
            <TableHead className="w-24 text-right">
              {t("item.quantity")}
            </TableHead>
            <TableHead className="w-20 text-right">{t("item.slot")}</TableHead>
            <TableHead className="w-12">
              <span className="sr-only">{t("action.editItem")}</span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((item) => {
            const metadata = getItemMetadata(item.ItemId, locale);
            return (
              <TableRow key={`${item.SlotIndex}-${item.ItemId}`}>
                <TableCell>
                  <div className="flex items-center gap-3">
                    {getItemImage(item.ItemId) ? (
                      <img
                        src={getItemImage(item.ItemId)}
                        alt={metadata.name}
                        className="size-9 rounded-md border bg-muted object-contain p-1"
                      />
                    ) : (
                      <div className="flex size-9 items-center justify-center rounded-md border bg-muted">
                        <Box className="size-4 text-muted-foreground" />
                      </div>
                    )}
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">
                        {metadata.name}
                      </p>
                      <p className="font-data truncate text-[10px] text-muted-foreground">
                        {item.ItemId}
                      </p>
                    </div>
                  </div>
                </TableCell>
                <TableCell className="font-data text-right">
                  {item.StackCount}
                </TableCell>
                <TableCell className="font-data text-right">
                  {item.SlotIndex}
                </TableCell>
                <TableCell className="text-right">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-xs"
                        onClick={() => onEditItem(item)}
                      >
                        <Pencil />
                        <span className="sr-only">{t("action.editItem")}</span>
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>{t("action.editItem")}</TooltipContent>
                  </Tooltip>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}

export function PlayerDetailSheet({
  playerUid,
  onOpenChange,
  requestLogin,
}: {
  playerUid: string | null;
  onOpenChange: (open: boolean) => void;
  requestLogin: () => void;
}) {
  const { locale, t } = useI18n();
  const { isAuthenticated } = useAuth();
  const queryClient = useQueryClient();
  const playerQuery = usePlayer(playerUid);
  const guildsQuery = useGuilds();
  const [palSearch, setPalSearch] = useState("");
  const [selectedPal, setSelectedPal] = useState<Pal | null>(null);
  const [palNicknameEdit, setPalNicknameEdit] = useState<Pal | null>(null);
  const [palLevelEdit, setPalLevelEdit] = useState<Pal | null>(null);
  const [palHealthEdit, setPalHealthEdit] = useState<Pal | null>(null);
  const [pendingAction, setPendingAction] = useState<PlayerAction | null>(null);
  const [actionMessage, setActionMessage] = useState("");
  const [itemDeliveryOpen, setItemDeliveryOpen] = useState(false);
  const [profileEditOpen, setProfileEditOpen] = useState(false);
  const [statPointsEditOpen, setStatPointsEditOpen] = useState(false);
  const [technologyPointsEditOpen, setTechnologyPointsEditOpen] =
    useState(false);
  const [mapUnlockOpen, setMapUnlockOpen] = useState(false);
  const [selectedInventoryItem, setSelectedInventoryItem] =
    useState<SelectedInventoryItem | null>(null);

  const whitelistQuery = useQuery({
    queryKey: queryKeys.whitelist,
    queryFn: scopedQueryFn(api.getWhitelist),
    enabled: isAuthenticated && Boolean(playerUid),
  });

  const player = playerQuery.data;
  const mapProgressComplete = isMapProgressComplete(player?.map_progress);
  const online = isRecentlyOnline(player?.last_online);
  const guild = useMemo(
    () =>
      guildsQuery.data?.find((candidate) =>
        candidate.players.some((member) => member.player_uid === playerUid),
      ),
    [guildsQuery.data, playerUid],
  );
  const isWhitelisted = useMemo(() => {
    if (!player) return false;
    return (whitelistQuery.data ?? []).some(
      (entry) =>
        (entry.player_uid && entry.player_uid === player.player_uid) ||
        (entry.user_id && entry.user_id === player.user_id) ||
        (entry.steam_id && entry.steam_id === player.steam_id),
    );
  }, [player, whitelistQuery.data]);

  const filteredPals = useMemo(() => {
    const normalized = palSearch.trim().toLowerCase();
    if (!normalized) return player?.pals ?? [];
    return (player?.pals ?? []).filter((pal) => {
      const name =
        `${pal.nickname} ${getPalName(pal.type, locale)} ${pal.skills.join(" ")}`.toLowerCase();
      return name.includes(normalized);
    });
  }, [locale, palSearch, player?.pals]);

  const actionMutation = useMutation({
    mutationFn: ({
      action,
      message,
    }: {
      action: PlayerAction;
      message: string;
    }) => api.playerAction(playerUid ?? "", action, message),
    onSuccess: async () => {
      toast.success(t("message.updated"));
      setPendingAction(null);
      setActionMessage("");
      await queryClient.invalidateQueries({ queryKey: queryKeys.players });
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const whitelistMutation = useMutation({
    mutationFn: (entry: WhitelistPlayer) => api.addWhitelist(entry),
    onSuccess: async () => {
      toast.success(t("message.updated"));
      await queryClient.invalidateQueries({ queryKey: queryKeys.whitelist });
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const requireAdmin = (action: () => void) => {
    if (!isAuthenticated) {
      toast.error(t("message.authRequired"));
      requestLogin();
      return;
    }
    action();
  };

  const applyPalLevelMutation = (mutation: PalLevelMutation) => {
    const updatePal = (candidate: Pal): Pal =>
      candidate.instance_id === mutation.instance_id
        ? {
            ...candidate,
            level: mutation.level_after,
            exp: mutation.exp_after,
            hp: mutation.hp_after,
            max_hp: mutation.max_hp_after,
          }
        : candidate;

    queryClient.setQueryData<Player>(
      queryKeys.player(mutation.player_uid),
      (current) =>
        current
          ? {
              ...current,
              pals: current.pals.map(updatePal),
            }
          : current,
    );
    setSelectedPal((current) => (current ? updatePal(current) : current));
    setPalLevelEdit((current) => (current ? updatePal(current) : current));
  };

  const applyPalHealthMutation = (mutation: PalHealthMutation) => {
    const updatePal = (candidate: Pal): Pal =>
      candidate.instance_id === mutation.instance_id
        ? {
            ...candidate,
            hp: mutation.hp_after,
          }
        : candidate;

    queryClient.setQueryData<Player>(
      queryKeys.player(mutation.player_uid),
      (current) =>
        current
          ? {
              ...current,
              pals: current.pals.map(updatePal),
            }
          : current,
    );
    setSelectedPal((current) => (current ? updatePal(current) : current));
    setPalHealthEdit((current) => (current ? updatePal(current) : current));
  };

  return (
    <>
      <Sheet open={Boolean(playerUid)} onOpenChange={onOpenChange}>
        <SheetContent side="right" className="w-full gap-0 p-0 sm:max-w-3xl">
          <SheetHeader className="border-b px-5 py-5 text-left sm:px-6">
            <div className="flex items-start gap-4 pr-8">
              <img
                src={avatarUrl}
                alt=""
                className="size-14 rounded-md border bg-muted object-cover"
              />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <SheetTitle className="truncate text-xl">
                    {player?.nickname ||
                      player?.account_name ||
                      t("players.details")}
                  </SheetTitle>
                  {player ? (
                    <Badge variant="secondary">Lv.{player.level}</Badge>
                  ) : null}
                </div>
                <SheetDescription className="mt-1 flex items-center gap-2">
                  <StatusDot online={online} />
                  {online ? t("status.online") : t("status.offline")}
                  {guild ? ` · ${guild.name}` : ""}
                </SheetDescription>
              </div>
            </div>
          </SheetHeader>

          {playerQuery.isPending ? (
            <LoadingState className="h-full" />
          ) : playerQuery.isError ? (
            <ErrorState
              error={playerQuery.error}
              retry={() => void playerQuery.refetch()}
            />
          ) : player ? (
            <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
              <Tabs
                defaultValue="profile"
                className="flex min-h-0 flex-1 flex-col"
              >
                <div className="border-b px-5 sm:px-6">
                  <TabsList
                    variant="line"
                    className="h-12 w-full justify-start overflow-x-auto"
                  >
                    <TabsTrigger value="profile">
                      {t("players.profile")}
                    </TabsTrigger>
                    <TabsTrigger value="pals">
                      {t("players.pals")}{" "}
                      <span className="font-data">
                        {player.pals?.length ?? 0}
                      </span>
                    </TabsTrigger>
                    <TabsTrigger value="inventory">
                      {t("players.inventory")}
                    </TabsTrigger>
                  </TabsList>
                </div>

                <ScrollArea className="min-h-0 flex-1">
                  <TabsContent
                    value="profile"
                    className="m-0 space-y-5 p-5 sm:p-6"
                  >
                    <div className="flex justify-end">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() =>
                          requireAdmin(() => setProfileEditOpen(true))
                        }
                      >
                        <Pencil />
                        {t("action.editProfile")}
                      </Button>
                    </div>

                    <div className="grid overflow-hidden rounded-md border sm:grid-cols-4">
                      <div className="border-b p-4 sm:border-b-0 sm:border-r">
                        <HeartPulse className="size-4 text-destructive" />
                        <p className="font-data mt-3 text-xl font-semibold">
                          {Math.round((player.hp ?? 0) / 1000)}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {t("players.hp")}
                        </p>
                      </div>
                      <div className="border-b p-4 sm:border-b-0 sm:border-r">
                        <ShieldCheck className="size-4 text-primary" />
                        <p className="font-data mt-3 text-xl font-semibold">
                          {Math.round((player.shield_hp ?? 0) / 1000)}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {t("players.shield")}
                        </p>
                      </div>
                      <div className="border-b p-4 sm:border-b-0 sm:border-r">
                        <Hammer className="size-4 text-[var(--warning)]" />
                        <p className="font-data mt-3 text-xl font-semibold">
                          {player.building_count ?? 0}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {t("players.buildings")}
                        </p>
                      </div>
                      <div className="p-4">
                        <Crown className="size-4 text-[var(--signal)]" />
                        <p className="font-data mt-3 text-xl font-semibold">
                          {player.level}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {t("players.level")}
                        </p>
                      </div>
                    </div>

                    <div className="flex items-center gap-4 rounded-md border p-4">
                      <div className="flex size-10 shrink-0 items-center justify-center rounded-md border bg-muted">
                        <Sparkles className="size-4 text-[var(--signal)]" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="text-xs text-muted-foreground">
                          {t("players.unusedStatusPoints")}
                        </p>
                        <p className="font-data mt-0.5 text-lg font-semibold">
                          {player.unused_status_points ?? "--"}
                        </p>
                        {player.unused_status_points === undefined ? (
                          <p className="mt-0.5 text-xs text-muted-foreground">
                            {t("players.statPointsSyncRequired")}
                          </p>
                        ) : null}
                      </div>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={player.unused_status_points === undefined}
                        onClick={() =>
                          requireAdmin(() => setStatPointsEditOpen(true))
                        }
                      >
                        <Pencil />
                        {t("action.editStatPoints")}
                      </Button>
                    </div>

                    <div className="grid gap-px overflow-hidden rounded-md border bg-border sm:grid-cols-[1fr_1fr_auto]">
                      <div className="flex items-center gap-3 bg-card p-4">
                        <Cog className="size-4 shrink-0 text-primary" />
                        <div className="min-w-0">
                          <p className="text-xs text-muted-foreground">
                            {t("players.technologyPoints")}
                          </p>
                          <p className="font-data mt-0.5 text-lg font-semibold">
                            {player.technology_points ?? "--"}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-3 bg-card p-4">
                        <Landmark className="size-4 shrink-0 text-[var(--signal)]" />
                        <div className="min-w-0">
                          <p className="text-xs text-muted-foreground">
                            {t("players.ancientTechnologyPoints")}
                          </p>
                          <p className="font-data mt-0.5 text-lg font-semibold">
                            {player.ancient_technology_points ?? "--"}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center bg-card p-4">
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={
                            player.technology_points === undefined ||
                            player.ancient_technology_points === undefined
                          }
                          onClick={() =>
                            requireAdmin(() =>
                              setTechnologyPointsEditOpen(true),
                            )
                          }
                        >
                          <Pencil />
                          {t("action.editTechnologyPoints")}
                        </Button>
                      </div>
                    </div>

                    <div className="overflow-hidden rounded-md border">
                      <div className="flex flex-wrap items-center gap-3 p-4">
                        <div className="flex size-10 shrink-0 items-center justify-center rounded-md border bg-muted">
                          <MapIcon className="size-4 text-[var(--success)]" />
                        </div>
                        <div className="min-w-40 flex-1">
                          <p className="text-sm font-medium">
                            {t("players.mapProgress")}
                          </p>
                          <p className="mt-0.5 text-xs text-muted-foreground">
                            {player.map_progress
                              ? t(
                                  mapProgressComplete
                                    ? "players.mapProgressComplete"
                                    : "players.mapProgressVersion",
                                  {
                                    version: player.map_progress.game_version,
                                  },
                                )
                              : t("players.mapProgressSyncRequired")}
                          </p>
                        </div>
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={
                            !player.map_progress?.progress_digest ||
                            mapProgressComplete
                          }
                          onClick={() =>
                            requireAdmin(() => setMapUnlockOpen(true))
                          }
                        >
                          <MapIcon />
                          {t(
                            mapProgressComplete
                              ? "action.mapFullyUnlocked"
                              : "action.unlockFullMap",
                          )}
                        </Button>
                      </div>
                      <div className="grid grid-cols-3 gap-px border-t bg-border">
                        <MapProgressMetric
                          label={t("mapUnlock.fastTravel")}
                          value={player.map_progress?.fast_travel_unlocked}
                          total={player.map_progress?.fast_travel_total}
                        />
                        <MapProgressMetric
                          label={t("mapUnlock.areas")}
                          value={player.map_progress?.areas_found}
                          total={player.map_progress?.areas_total}
                        />
                        <MapProgressMetric
                          label={t("mapUnlock.worldMaps")}
                          value={player.map_progress?.world_maps_unlocked}
                          total={player.map_progress?.world_maps_total}
                        />
                      </div>
                    </div>

                    <div className="grid gap-x-6 rounded-md border px-4 sm:grid-cols-2">
                      <DetailField
                        label={t("players.playerUid")}
                        value={player.player_uid}
                        copyable
                      />
                      <DetailField
                        label={t("players.userId")}
                        value={player.user_id}
                        copyable
                      />
                      <DetailField
                        label={t("players.steamId")}
                        value={player.steam_id}
                        copyable
                      />
                      <DetailField
                        label={t("players.account")}
                        value={player.account_name}
                      />
                      <DetailField
                        label={t("players.ip")}
                        value={player.ip}
                        copyable
                      />
                      <DetailField
                        label={t("players.ping")}
                        value={
                          player.ping ? `${player.ping.toFixed(1)} ms` : "--"
                        }
                      />
                      <DetailField
                        label={t("players.location")}
                        value={`X ${formatCoordinate(player.location_x)} / Y ${formatCoordinate(player.location_y)}`}
                      />
                      <DetailField
                        label={t("players.lastOnline")}
                        value={formatDateTime(player.last_online)}
                      />
                    </div>
                  </TabsContent>

                  <TabsContent
                    value="pals"
                    className="m-0 space-y-4 p-5 sm:p-6"
                  >
                    <Input
                      value={palSearch}
                      onChange={(event) => setPalSearch(event.target.value)}
                      placeholder={t("players.search")}
                    />
                    {filteredPals.length ? (
                      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                        {filteredPals.map((pal, index) => {
                          const name =
                            pal.nickname || getPalName(pal.type, locale);
                          return (
                            <button
                              key={
                                pal.instance_id ??
                                `${pal.type}-${index}-${pal.nickname}`
                              }
                              type="button"
                              onClick={() => setSelectedPal(pal)}
                              className="flex min-w-0 items-center gap-3 rounded-md border bg-card p-3 text-left transition-colors hover:bg-muted/65 focus-visible:ring-2 focus-visible:ring-ring"
                            >
                              <img
                                src={getPalImage(pal.type, pal.is_boss)}
                                alt={name}
                                className="size-12 shrink-0 rounded-md border bg-muted object-contain p-1"
                              />
                              <div className="min-w-0 flex-1">
                                <p className="truncate text-sm font-medium">
                                  {name}
                                </p>
                                <p className="mt-0.5 truncate text-xs text-muted-foreground">
                                  {getPalName(pal.type, locale)}
                                </p>
                              </div>
                              <span className="font-data text-xs">
                                Lv.{pal.level}
                              </span>
                            </button>
                          );
                        })}
                      </div>
                    ) : (
                      <div className="flex min-h-40 items-center justify-center text-sm text-muted-foreground">
                        {t("players.noPals")}
                      </div>
                    )}
                  </TabsContent>

                  <TabsContent value="inventory" className="m-0 p-5 sm:p-6">
                    <Tabs defaultValue="CommonContainerId">
                      <div className="mb-4 flex min-w-0 items-center gap-2">
                        <TabsList className="min-w-0 flex-1 justify-start overflow-x-auto">
                          {inventoryTabs.map(([key, label]) => (
                            <TabsTrigger key={key} value={key}>
                              {t(label)}
                            </TabsTrigger>
                          ))}
                        </TabsList>
                        <Button
                          size="sm"
                          className="shrink-0"
                          onClick={() =>
                            requireAdmin(() => setItemDeliveryOpen(true))
                          }
                        >
                          <PackagePlus />
                          {t("action.deliverItem")}
                        </Button>
                      </div>
                      {inventoryTabs.map(([key, label, container]) => (
                        <TabsContent key={key} value={key} className="m-0">
                          <InventoryTable
                            items={player.items?.[key] ?? []}
                            onEditItem={(item) =>
                              requireAdmin(() =>
                                setSelectedInventoryItem({
                                  item,
                                  container,
                                  containerLabel: t(label),
                                }),
                              )
                            }
                          />
                        </TabsContent>
                      ))}
                    </Tabs>
                  </TabsContent>
                </ScrollArea>
              </Tabs>

              <div className="flex flex-wrap items-center gap-2 border-t bg-background px-5 py-3 sm:px-6">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={isWhitelisted || whitelistMutation.isPending}
                  onClick={() =>
                    requireAdmin(() =>
                      whitelistMutation.mutate({
                        name: player.nickname,
                        player_uid: player.player_uid,
                        user_id: player.user_id,
                        steam_id: player.steam_id,
                      }),
                    )
                  }
                >
                  {whitelistMutation.isPending ? (
                    <LoaderCircle className="animate-spin" />
                  ) : (
                    <ShieldCheck />
                  )}
                  {isWhitelisted ? <Check /> : null}
                  {t("action.addWhitelist")}
                </Button>
                <div className="ml-auto flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => requireAdmin(() => setPendingAction("kick"))}
                  >
                    <UserRoundX /> {t("action.kick")}
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={() => requireAdmin(() => setPendingAction("ban"))}
                  >
                    <Ban /> {t("action.ban")}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      requireAdmin(() => setPendingAction("unban"))
                    }
                  >
                    <ShieldOff /> {t("action.unban")}
                  </Button>
                </div>
              </div>
            </div>
          ) : null}
        </SheetContent>
      </Sheet>

      <AlertDialog
        open={Boolean(pendingAction)}
        onOpenChange={(open) => {
          if (!open) {
            setPendingAction(null);
            setActionMessage("");
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingAction ? t(`action.${pendingAction}`) : ""} ·{" "}
              {player?.nickname}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingAction === "unban"
                ? t("action.unban")
                : t("operations.broadcastPlaceholder")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {pendingAction !== "unban" ? (
            <Textarea
              value={actionMessage}
              onChange={(event) => setActionMessage(event.target.value)}
              placeholder={t("operations.broadcastPlaceholder")}
            />
          ) : null}
          <AlertDialogFooter>
            <AlertDialogCancel>{t("action.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant={pendingAction === "ban" ? "destructive" : "default"}
              disabled={actionMutation.isPending}
              onClick={(event) => {
                event.preventDefault();
                if (pendingAction) {
                  actionMutation.mutate({
                    action: pendingAction,
                    message: actionMessage,
                  });
                }
              }}
            >
              {actionMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : null}
              {t("action.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <PalDetailDialog
        pal={selectedPal}
        onOpenChange={(open) => !open && setSelectedPal(null)}
        onRename={
          selectedPal?.instance_id
            ? () =>
                requireAdmin(() => {
                  setPalNicknameEdit(selectedPal);
                  setSelectedPal(null);
                })
            : undefined
        }
        onEditLevel={
          selectedPal?.instance_id
            ? () =>
                requireAdmin(() => {
                  setPalLevelEdit(selectedPal);
                  setSelectedPal(null);
                })
            : undefined
        }
        onRestoreHealth={
          selectedPal?.instance_id &&
          selectedPal.max_hp > 0 &&
          selectedPal.hp < selectedPal.max_hp
            ? () =>
                requireAdmin(() => {
                  setPalHealthEdit(selectedPal);
                  setSelectedPal(null);
                })
            : undefined
        }
      />

      {player ? (
        <>
          <ItemDeliveryDialog
            open={itemDeliveryOpen}
            onOpenChange={setItemDeliveryOpen}
            playerUid={player.player_uid}
            playerName={player.nickname || player.account_name}
          />
          {profileEditOpen ? (
            <PlayerProfileDialog
              open
              onOpenChange={setProfileEditOpen}
              playerUid={player.player_uid}
              currentNickname={player.nickname}
              currentLevel={player.level}
            />
          ) : null}
          {statPointsEditOpen && player.unused_status_points !== undefined ? (
            <PlayerStatPointsDialog
              open
              onOpenChange={setStatPointsEditOpen}
              playerUid={player.player_uid}
              playerName={player.nickname || player.account_name}
              currentPoints={player.unused_status_points}
            />
          ) : null}
          {technologyPointsEditOpen &&
          player.technology_points !== undefined &&
          player.ancient_technology_points !== undefined ? (
            <PlayerTechnologyPointsDialog
              open
              onOpenChange={setTechnologyPointsEditOpen}
              playerUid={player.player_uid}
              playerName={player.nickname || player.account_name}
              currentTechnologyPoints={player.technology_points}
              currentAncientTechnologyPoints={player.ancient_technology_points}
            />
          ) : null}
          {mapUnlockOpen && player.map_progress ? (
            <PlayerMapUnlockDialog
              key={player.player_uid}
              open
              onOpenChange={setMapUnlockOpen}
              playerUid={player.player_uid}
              playerName={player.nickname || player.account_name}
              progress={player.map_progress}
            />
          ) : null}
          {palNicknameEdit?.instance_id ? (
            <PalNicknameDialog
              key={`${palNicknameEdit.instance_id}-${palNicknameEdit.nickname}-${palNicknameEdit.level}-${palNicknameEdit.exp}`}
              open
              onOpenChange={(open) => {
                if (!open) setPalNicknameEdit(null);
              }}
              playerUid={player.player_uid}
              pal={palNicknameEdit}
            />
          ) : null}
          {palLevelEdit?.instance_id ? (
            <PalLevelDialog
              key={palLevelEdit.instance_id}
              open
              onOpenChange={(open) => {
                if (!open) setPalLevelEdit(null);
              }}
              onUpdated={applyPalLevelMutation}
              playerUid={player.player_uid}
              pal={palLevelEdit}
            />
          ) : null}
          {palHealthEdit?.instance_id ? (
            <PalHealthDialog
              key={palHealthEdit.instance_id}
              open
              onOpenChange={(open) => {
                if (!open) setPalHealthEdit(null);
              }}
              onUpdated={applyPalHealthMutation}
              playerUid={player.player_uid}
              pal={palHealthEdit}
            />
          ) : null}
          {selectedInventoryItem ? (
            <ItemQuantityDialog
              key={`${selectedInventoryItem.container}-${selectedInventoryItem.item.SlotIndex}-${selectedInventoryItem.item.StackCount}`}
              open
              onOpenChange={(open) => {
                if (!open) setSelectedInventoryItem(null);
              }}
              playerUid={player.player_uid}
              playerName={player.nickname || player.account_name}
              item={selectedInventoryItem.item}
              container={selectedInventoryItem.container}
              containerLabel={selectedInventoryItem.containerLabel}
            />
          ) : null}
        </>
      ) : null}
    </>
  );
}
