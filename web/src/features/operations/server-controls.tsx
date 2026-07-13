import { useMemo, useState } from "react";
import {
  BellRing,
  DatabaseBackup,
  Download,
  ExternalLink,
  LoaderCircle,
  Power,
  RefreshCw,
  Save,
  Send,
  ServerCog,
  ShieldAlert,
  Terminal,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { ErrorState, LoadingState } from "@/components/common/data-state";
import { Panel } from "@/components/common/panel";
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
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { api, getApiErrorMessage } from "@/lib/api";
import { downloadBlob, formatCoordinate } from "@/lib/format";
import { useI18n } from "@/lib/i18n";
import { queryKeys } from "@/hooks/use-server-data";

export function ServerControls() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [broadcastMessage, setBroadcastMessage] = useState("");
  const [rconCommand, setRconCommand] = useState("Info");
  const [rconResponse, setRconResponse] = useState("");
  const [shutdownSeconds, setShutdownSeconds] = useState(60);
  const [shutdownMessage, setShutdownMessage] = useState("");
  const [confirmAction, setConfirmAction] = useState<"save" | "stop" | null>(
    null,
  );

  const settingsQuery = useQuery({
    queryKey: queryKeys.settings,
    queryFn: api.getSettings,
  });
  const snapshotQuery = useQuery({
    queryKey: queryKeys.snapshot,
    queryFn: api.getWorldSnapshot,
    staleTime: 10_000,
  });

  const settingsRows = useMemo(
    () =>
      Object.entries(settingsQuery.data ?? {})
        .map(([name, value]) => ({
          name,
          value:
            value === null || value === undefined
              ? "--"
              : typeof value === "object"
                ? JSON.stringify(value)
                : String(value),
        }))
        .sort((a, b) => a.name.localeCompare(b.name)),
    [settingsQuery.data],
  );
  const actors = snapshotQuery.data?.ActorData ?? [];

  const saveMutation = useMutation({
    mutationFn: api.saveWorld,
    onSuccess: () => toast.success(t("message.saved")),
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });
  const stopMutation = useMutation({
    mutationFn: api.stopServer,
    onSuccess: () => toast.success(t("message.stopSent")),
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });
  const broadcastMutation = useMutation({
    mutationFn: api.broadcast,
    onSuccess: () => {
      toast.success(t("message.broadcasted"));
      setBroadcastMessage("");
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });
  const rconMutation = useMutation({
    mutationFn: api.runRcon,
    onSuccess: (result) => {
      setRconResponse(result.message || t("operations.rconEmpty"));
      toast.success(t("message.rconExecuted"));
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });
  const shutdownMutation = useMutation({
    mutationFn: ({ seconds, message }: { seconds: number; message: string }) =>
      api.shutdown(seconds, message),
    onSuccess: () => toast.success(t("message.shutdownSent")),
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const refreshDiagnostics = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.settings }),
      queryClient.invalidateQueries({ queryKey: queryKeys.snapshot }),
    ]);
  };

  const downloadSnapshot = () => {
    if (!snapshotQuery.data) return;
    const date = new Date().toISOString().replaceAll(":", "-");
    downloadBlob(
      new Blob([JSON.stringify(snapshotQuery.data, null, 2)], {
        type: "application/json",
      }),
      `palworld-world-snapshot-${date}.json`,
    );
  };

  return (
    <div className="space-y-4">
      <div className="grid gap-4 xl:grid-cols-2">
        <Panel
          title={t("operations.broadcast")}
          description={t("operations.broadcastPlaceholder")}
          contentClassName="p-4 sm:p-5"
        >
          <form
            className="space-y-3"
            onSubmit={(event) => {
              event.preventDefault();
              const message = broadcastMessage.trim();
              if (message) broadcastMutation.mutate(message);
            }}
          >
            <Textarea
              value={broadcastMessage}
              onChange={(event) => setBroadcastMessage(event.target.value)}
              placeholder={t("operations.broadcastPlaceholder")}
              className="min-h-24"
            />
            <div className="flex justify-end">
              <Button
                type="submit"
                disabled={
                  !broadcastMessage.trim() || broadcastMutation.isPending
                }
              >
                {broadcastMutation.isPending ? (
                  <LoaderCircle className="animate-spin" />
                ) : (
                  <Send />
                )}
                {t("action.send")}
              </Button>
            </div>
          </form>
        </Panel>

        <Panel
          title={t("operations.shutdown")}
          description={t("operations.shutdownMessage")}
          contentClassName="p-4 sm:p-5"
        >
          <form
            className="space-y-3"
            onSubmit={(event) => {
              event.preventDefault();
              if (shutdownSeconds > 0) {
                shutdownMutation.mutate({
                  seconds: shutdownSeconds,
                  message: shutdownMessage.trim(),
                });
              }
            }}
          >
            <div className="grid gap-3 sm:grid-cols-[150px_minmax(0,1fr)]">
              <div className="grid gap-1.5">
                <Label htmlFor="shutdown-seconds">
                  {t("operations.shutdownDelay")}
                </Label>
                <Input
                  id="shutdown-seconds"
                  type="number"
                  min={1}
                  max={86400}
                  inputMode="numeric"
                  value={shutdownSeconds}
                  onChange={(event) =>
                    setShutdownSeconds(Number(event.target.value))
                  }
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="shutdown-message">
                  {t("operations.shutdownMessage")}
                </Label>
                <Input
                  id="shutdown-message"
                  value={shutdownMessage}
                  onChange={(event) => setShutdownMessage(event.target.value)}
                  placeholder={t("operations.broadcastPlaceholder")}
                />
              </div>
            </div>
            <div className="flex justify-end">
              <Button
                type="submit"
                variant="secondary"
                disabled={shutdownSeconds < 1 || shutdownMutation.isPending}
              >
                {shutdownMutation.isPending ? (
                  <LoaderCircle className="animate-spin" />
                ) : (
                  <Power />
                )}
                {t("operations.shutdown")}
              </Button>
            </div>
          </form>
        </Panel>
      </div>

      <Panel
        title={t("operations.rcon")}
        description={t("operations.rconDescription")}
        contentClassName="p-4 sm:p-5"
      >
        <form
          className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(280px,0.75fr)]"
          onSubmit={(event) => {
            event.preventDefault();
            const command = rconCommand.trim();
            if (command) rconMutation.mutate(command);
          }}
        >
          <div className="space-y-3">
            <Textarea
              value={rconCommand}
              onChange={(event) => setRconCommand(event.target.value)}
              placeholder={t("operations.rconPlaceholder")}
              className="font-data min-h-24"
              maxLength={4096}
            />
            <div className="flex justify-end">
              <Button
                type="submit"
                disabled={!rconCommand.trim() || rconMutation.isPending}
              >
                {rconMutation.isPending ? (
                  <LoaderCircle className="animate-spin" />
                ) : (
                  <Terminal />
                )}
                {t("action.execute")}
              </Button>
            </div>
          </div>
          <pre className="font-data min-h-24 overflow-auto whitespace-pre-wrap break-words rounded-md border bg-muted/35 p-3 text-xs text-muted-foreground">
            {rconResponse || t("operations.rconEmpty")}
          </pre>
        </form>
      </Panel>

      <Panel contentClassName="grid sm:grid-cols-3">
        <button
          type="button"
          className="flex min-h-28 items-start gap-3 border-b p-4 text-left transition-colors hover:bg-muted/55 sm:border-b-0 sm:border-r sm:p-5"
          onClick={() => setConfirmAction("save")}
        >
          <Save className="mt-0.5 size-5 text-primary" />
          <span>
            <span className="block text-sm font-semibold">
              {t("operations.save")}
            </span>
            <span className="mt-1 block text-xs text-muted-foreground">
              {t("confirm.save")}
            </span>
          </span>
        </button>
        <button
          type="button"
          className="flex min-h-28 items-start gap-3 border-b p-4 text-left transition-colors hover:bg-muted/55 sm:border-b-0 sm:border-r sm:p-5"
          onClick={() =>
            window.open("/#/configuration", "_blank", "noopener,noreferrer")
          }
        >
          <ServerCog className="mt-0.5 size-5 text-[var(--signal)]" />
          <span>
            <span className="flex items-center gap-1 text-sm font-semibold">
              {t("operations.openGenerator")}{" "}
              <ExternalLink className="size-3" />
            </span>
            <span className="mt-1 block text-xs text-muted-foreground">
              PalWorldSettings.ini
            </span>
          </span>
        </button>
        <button
          type="button"
          className="flex min-h-28 items-start gap-3 p-4 text-left transition-colors hover:bg-destructive/8 sm:p-5"
          onClick={() => setConfirmAction("stop")}
        >
          <ShieldAlert className="mt-0.5 size-5 text-destructive" />
          <span>
            <span className="block text-sm font-semibold text-destructive">
              {t("operations.stop")}
            </span>
            <span className="mt-1 block text-xs text-muted-foreground">
              {t("confirm.stop")}
            </span>
          </span>
        </button>
      </Panel>

      <div className="grid gap-4 2xl:grid-cols-2">
        <Panel
          title={t("operations.settings")}
          description={`${settingsRows.length} ${t("operations.settingName")}`}
          actions={
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => void settingsQuery.refetch()}
            >
              <RefreshCw />
              <span className="sr-only">{t("action.refresh")}</span>
            </Button>
          }
        >
          {settingsQuery.isPending ? (
            <LoadingState className="min-h-64" />
          ) : settingsQuery.isError ? (
            <ErrorState
              error={settingsQuery.error}
              retry={() => void settingsQuery.refetch()}
            />
          ) : (
            <div className="max-h-[520px] overflow-auto">
              <Table>
                <TableHeader className="sticky top-0 z-10 bg-card">
                  <TableRow>
                    <TableHead>{t("operations.settingName")}</TableHead>
                    <TableHead>{t("operations.settingValue")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {settingsRows.map((row) => (
                    <TableRow key={row.name}>
                      <TableCell className="font-data text-xs">
                        {row.name}
                      </TableCell>
                      <TableCell className="max-w-[360px] break-words text-xs text-muted-foreground">
                        {row.value}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </Panel>

        <Panel
          title={t("operations.snapshot")}
          description={snapshotQuery.data?.Time || "--"}
          actions={
            <div className="flex gap-1">
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={downloadSnapshot}
                disabled={!snapshotQuery.data}
              >
                <Download />
                <span className="sr-only">
                  {t("operations.downloadSnapshot")}
                </span>
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => void snapshotQuery.refetch()}
              >
                <RefreshCw />
                <span className="sr-only">{t("action.refresh")}</span>
              </Button>
            </div>
          }
        >
          {snapshotQuery.isPending ? (
            <LoadingState className="min-h-64" />
          ) : snapshotQuery.isError ? (
            <ErrorState
              error={snapshotQuery.error}
              retry={() => void snapshotQuery.refetch()}
            />
          ) : (
            <>
              <div className="grid grid-cols-3 border-b">
                <div className="border-r p-4">
                  <BellRing className="size-4 text-[var(--signal)]" />
                  <p className="font-data mt-2 text-xl font-semibold">
                    {snapshotQuery.data?.FPS ?? "--"}
                  </p>
                  <p className="text-xs text-muted-foreground">FPS</p>
                </div>
                <div className="border-r p-4">
                  <DatabaseBackup className="size-4 text-primary" />
                  <p className="font-data mt-2 text-xl font-semibold">
                    {snapshotQuery.data?.AverageFPS ?? "--"}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {t("operations.averageFps")}
                  </p>
                </div>
                <div className="p-4">
                  <ServerCog className="size-4 text-[var(--warning)]" />
                  <p className="font-data mt-2 text-xl font-semibold">
                    {actors.length}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {t("operations.actorCount")}
                  </p>
                </div>
              </div>
              <div className="max-h-[370px] overflow-auto">
                <Table>
                  <TableHeader className="sticky top-0 z-10 bg-card">
                    <TableRow>
                      <TableHead>{t("operations.actorType")}</TableHead>
                      <TableHead>{t("operations.actorName")}</TableHead>
                      <TableHead className="w-20">Lv.</TableHead>
                      <TableHead>{t("operations.actorAction")}</TableHead>
                      <TableHead>{t("operations.actorLocation")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {actors.map((actor, index) => (
                      <TableRow
                        key={`${actor.Type}-${actor.NickName}-${index}`}
                      >
                        <TableCell>{actor.Type || "--"}</TableCell>
                        <TableCell>
                          {actor.NickName ||
                            actor.TrainerNickName ||
                            actor.GuildName ||
                            actor.Class ||
                            "--"}
                        </TableCell>
                        <TableCell className="font-data">
                          {actor.level ?? "--"}
                        </TableCell>
                        <TableCell>
                          {actor.Action || actor.AI_Action || "--"}
                        </TableCell>
                        <TableCell className="font-data text-xs text-muted-foreground">
                          {formatCoordinate(actor.LocationX)},{" "}
                          {formatCoordinate(actor.LocationY)},{" "}
                          {formatCoordinate(actor.LocationZ)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </>
          )}
        </Panel>
      </div>

      <div className="flex justify-end">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => void refreshDiagnostics()}
        >
          <RefreshCw /> {t("action.refresh")}
        </Button>
      </div>

      <AlertDialog
        open={Boolean(confirmAction)}
        onOpenChange={(open) => !open && setConfirmAction(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirmAction === "save"
                ? t("operations.save")
                : t("operations.stop")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirmAction === "save" ? t("confirm.save") : t("confirm.stop")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("action.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant={confirmAction === "stop" ? "destructive" : "default"}
              disabled={saveMutation.isPending || stopMutation.isPending}
              onClick={(event) => {
                event.preventDefault();
                if (confirmAction === "save") {
                  saveMutation.mutate(undefined, {
                    onSuccess: () => setConfirmAction(null),
                  });
                } else if (confirmAction === "stop") {
                  stopMutation.mutate(undefined, {
                    onSuccess: () => setConfirmAction(null),
                  });
                }
              }}
            >
              {saveMutation.isPending || stopMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : null}
              {t("action.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
