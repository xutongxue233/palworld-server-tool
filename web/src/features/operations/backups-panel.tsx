import { useState } from "react";
import {
  ArchiveRestore,
  CalendarRange,
  CircleCheck,
  DatabaseBackup,
  Download,
  FileArchive,
  FolderClock,
  HardDriveDownload,
  LoaderCircle,
  Power,
  RefreshCw,
  RotateCw,
  Search,
  ShieldCheck,
  Trash2,
  TriangleAlert,
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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { queryKeys, scopedQueryFn } from "@/hooks/use-server-data";
import { api, getApiErrorMessage } from "@/lib/api";
import { downloadBlob, formatDateTime } from "@/lib/format";
import { useI18n } from "@/lib/i18n";
import type { NativeBackup } from "@/types/api";

function toInputValue(timestamp: number) {
  const date = new Date(timestamp);
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const unit = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  return `${(bytes / 1024 ** unit).toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

export function BackupsPanel() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [startInput, setStartInput] = useState(() =>
    toInputValue(Date.now() - 24 * 60 * 60 * 1000),
  );
  const [endInput, setEndInput] = useState(() => toInputValue(Date.now()));
  const [range, setRange] = useState(() => ({
    start: Date.now() - 24 * 60 * 60 * 1000,
    end: Date.now(),
  }));
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [restoreBackup, setRestoreBackup] = useState<NativeBackup | null>(null);
  const [restartAfter, setRestartAfter] = useState(false);

  const nativeQuery = useQuery({
    queryKey: queryKeys.nativeBackups,
    queryFn: scopedQueryFn(api.getNativeBackups),
  });
  const backupsQuery = useQuery({
    queryKey: queryKeys.backups(range.start, range.end),
    queryFn: scopedQueryFn((scope) =>
      api.getBackups(range.start, range.end, scope),
    ),
  });

  const removeMutation = useMutation({
    mutationFn: api.removeBackup,
    onSuccess: async () => {
      toast.success(t("message.deleted"));
      setDeleteId(null);
      await queryClient.invalidateQueries({ queryKey: queryKeys.backupsRoot });
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const restoreMutation = useMutation({
    mutationFn: ({
      backup,
      restart,
    }: {
      backup: NativeBackup;
      restart: boolean;
    }) => api.restoreNativeBackup(backup.backup_id, backup.digest, restart),
    onSuccess: async (result) => {
      toast.success(
        t("backup.nativeRestoreSuccess", {
          name: result.restored_backup.backup_id,
        }),
        {
          description: t("backup.safetyBackupCreated", {
            name: result.safety_backup.path,
          }),
        },
      );
      if (result.sync_error) {
        toast.warning(t("backup.restoreSyncWarning"), {
          description: result.sync_error,
        });
      }
      if (result.restart_error) {
        toast.warning(t("backup.restoreRestartWarning"), {
          description: result.restart_error,
        });
      }
      setRestoreBackup(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.nativeBackups }),
        queryClient.invalidateQueries({ queryKey: queryKeys.backupsRoot }),
        queryClient.invalidateQueries({ queryKey: queryKeys.control }),
        queryClient.invalidateQueries({ queryKey: queryKeys.players }),
      ]);
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const download = async (backupId: string, filename: string) => {
    setDownloadingId(backupId);
    try {
      const blob = await api.downloadBackup(backupId);
      downloadBlob(blob, filename || `${backupId}.zip`);
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setDownloadingId(null);
    }
  };

  const nativeCatalog = nativeQuery.data?.native_backups;
  const control = nativeQuery.data?.server_control;
  const restorePipeline = [
    { icon: CircleCheck, number: "01", label: t("backup.pipelineValidate") },
    { icon: Power, number: "02", label: t("backup.pipelineStop") },
    {
      icon: ShieldCheck,
      number: "03",
      label: t("backup.pipelineProtect"),
    },
    {
      icon: HardDriveDownload,
      number: "04",
      label: t("backup.pipelineRestore"),
    },
    { icon: RotateCw, number: "05", label: t("backup.pipelineRestart") },
  ];
  const openRestore = (backup: NativeBackup) => {
    setRestoreBackup(backup);
    setRestartAfter(
      Boolean(control?.configured && (control.online || control.running)),
    );
  };
  const refreshAll = () => {
    void nativeQuery.refetch();
    void backupsQuery.refetch();
  };

  return (
    <Panel
      title={t("operations.backups")}
      description={t("backup.description")}
      actions={
        <Button variant="ghost" size="icon-sm" onClick={refreshAll}>
          <RefreshCw />
          <span className="sr-only">{t("action.refresh")}</span>
        </Button>
      }
    >
      <section className="border-b bg-[linear-gradient(135deg,color-mix(in_oklab,var(--primary)_10%,transparent),transparent_58%)]">
        <div className="flex flex-col gap-3 border-b px-4 py-4 sm:flex-row sm:items-start sm:justify-between sm:px-5">
          <div className="flex min-w-0 items-start gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-md border border-primary/25 bg-primary/10 text-primary">
              <FolderClock className="size-4" />
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="text-sm font-semibold">
                  {t("backup.nativeTitle")}
                </h3>
                <Badge variant="outline">{t("backup.recommended")}</Badge>
              </div>
              <p className="mt-1 max-w-3xl text-xs leading-5 text-muted-foreground">
                {t("backup.nativeDescription")}
              </p>
            </div>
          </div>
          {nativeCatalog?.world_id ? (
            <div className="shrink-0 text-left sm:text-right">
              <div className="text-[10px] font-medium tracking-[0.16em] text-muted-foreground uppercase">
                {t("backup.world")}
              </div>
              <div className="mt-1 max-w-56 truncate font-data text-xs">
                {nativeCatalog.world_id}
              </div>
            </div>
          ) : null}
        </div>

        {nativeQuery.isPending ? (
          <LoadingState />
        ) : nativeQuery.isError ? (
          <ErrorState
            error={nativeQuery.error}
            retry={() => void nativeQuery.refetch()}
          />
        ) : !nativeCatalog?.configured ? (
          <div className="m-4 flex gap-3 rounded-md border border-amber-500/30 bg-amber-500/8 p-4 sm:m-5">
            <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-600" />
            <div>
              <p className="text-sm font-medium">
                {t("backup.nativeNotConfigured")}
              </p>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {t("backup.nativeNotConfiguredHint")}
              </p>
            </div>
          </div>
        ) : !nativeCatalog.available || nativeCatalog.backups.length === 0 ? (
          <div className="m-4 flex gap-3 rounded-md border border-dashed p-4 sm:m-5">
            <DatabaseBackup className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <div>
              <p className="text-sm font-medium">{t("backup.nativeEmpty")}</p>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {t("backup.nativeEmptyHint")}
              </p>
            </div>
          </div>
        ) : (
          <div className="grid gap-3 p-4 sm:p-5 lg:grid-cols-2">
            {nativeCatalog.backups.map((backup) => (
              <article
                key={backup.backup_id}
                className="group rounded-md border bg-card/90 p-4 shadow-xs transition-colors hover:border-primary/30"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-data text-sm font-semibold">
                        {formatDateTime(backup.created_at)}
                      </span>
                      <Badge
                        variant={backup.valid ? "secondary" : "destructive"}
                      >
                        {backup.valid ? t("backup.valid") : t("backup.invalid")}
                      </Badge>
                    </div>
                    <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                      <span>{formatBytes(backup.size_bytes)}</span>
                      <span>
                        {t("backup.fileCount", { count: backup.file_count })}
                      </span>
                      <span>
                        {t("backup.playerCount", {
                          count: backup.player_files,
                        })}
                      </span>
                      {backup.has_world_option ? (
                        <span className="text-amber-600">WorldOption.sav</span>
                      ) : null}
                    </div>
                  </div>
                  {backup.valid ? (
                    <CircleCheck className="size-4 shrink-0 text-emerald-600" />
                  ) : (
                    <TriangleAlert className="size-4 shrink-0 text-destructive" />
                  )}
                </div>
                {backup.issues?.length ? (
                  <div className="mt-3 rounded border border-destructive/20 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                    {backup.issues.join("; ")}
                  </div>
                ) : null}
                <div className="mt-4 flex items-center justify-between gap-3 border-t pt-3">
                  <span className="truncate font-data text-[10px] text-muted-foreground">
                    {backup.backup_id}
                  </span>
                  <Button
                    size="sm"
                    variant={backup.valid ? "outline" : "ghost"}
                    disabled={!backup.valid}
                    onClick={() => openRestore(backup)}
                  >
                    <ArchiveRestore /> {t("backup.restore")}
                  </Button>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>

      <section>
        <div className="flex items-start gap-3 border-b px-4 py-4 sm:px-5">
          <div className="flex size-9 shrink-0 items-center justify-center rounded-md border bg-muted/40 text-muted-foreground">
            <ShieldCheck className="size-4" />
          </div>
          <div>
            <h3 className="text-sm font-semibold">{t("backup.safetyTitle")}</h3>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              {t("backup.safetyDescription")}
            </p>
          </div>
        </div>

        <div className="grid gap-3 border-b p-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-end sm:p-5">
          <div className="grid gap-1.5">
            <Label htmlFor="backup-start">{t("backup.start")}</Label>
            <div className="relative">
              <CalendarRange className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                id="backup-start"
                type="datetime-local"
                value={startInput}
                onChange={(event) => setStartInput(event.target.value)}
                className="pl-9"
              />
            </div>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="backup-end">{t("backup.end")}</Label>
            <div className="relative">
              <CalendarRange className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                id="backup-end"
                type="datetime-local"
                value={endInput}
                onChange={(event) => setEndInput(event.target.value)}
                className="pl-9"
              />
            </div>
          </div>
          <Button
            onClick={() => {
              const start = new Date(startInput).getTime();
              const end = new Date(endInput).getTime();
              if (
                Number.isFinite(start) &&
                Number.isFinite(end) &&
                start <= end
              ) {
                setRange({ start, end });
              }
            }}
          >
            <Search /> {t("action.search")}
          </Button>
        </div>

        {backupsQuery.isPending ? (
          <LoadingState />
        ) : backupsQuery.isError ? (
          <ErrorState
            error={backupsQuery.error}
            retry={() => void backupsQuery.refetch()}
          />
        ) : (backupsQuery.data ?? []).length ? (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("backup.time")}</TableHead>
                  <TableHead>{t("backup.file")}</TableHead>
                  <TableHead className="w-32" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {(backupsQuery.data ?? []).map((backup) => (
                  <TableRow key={backup.backup_id}>
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <FileArchive className="size-4 text-primary" />
                        <span className="font-data text-xs">
                          {formatDateTime(backup.save_time)}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className="font-data text-xs text-muted-foreground">
                      {backup.path}
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          disabled={downloadingId === backup.backup_id}
                          onClick={() =>
                            void download(backup.backup_id, backup.path)
                          }
                        >
                          {downloadingId === backup.backup_id ? (
                            <LoaderCircle className="animate-spin" />
                          ) : (
                            <Download />
                          )}
                          <span className="sr-only">
                            {t("action.download")}
                          </span>
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => setDeleteId(backup.backup_id)}
                        >
                          <Trash2 />
                          <span className="sr-only">{t("action.delete")}</span>
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        ) : (
          <div className="flex min-h-36 items-center justify-center text-sm text-muted-foreground">
            {t("message.empty")}
          </div>
        )}
      </section>

      <AlertDialog
        open={Boolean(restoreBackup)}
        onOpenChange={(open) => !open && setRestoreBackup(null)}
      >
        <AlertDialogContent className="sm:max-w-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("backup.restoreTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("backup.restoreDescription", {
                name: restoreBackup?.backup_id ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <div className="overflow-hidden rounded-md border">
            <div className="grid gap-0 sm:grid-cols-5">
              {restorePipeline.map((step, index) => {
                const PipelineIcon = step.icon;
                return (
                  <div
                    key={step.number}
                    className="relative flex items-center gap-3 border-b p-3 last:border-b-0 sm:block sm:border-r sm:border-b-0 sm:last:border-r-0"
                  >
                    <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                      <PipelineIcon className="size-3.5" />
                    </div>
                    <div className="mt-0 sm:mt-3">
                      <div className="font-data text-[9px] text-muted-foreground">
                        {step.number}
                      </div>
                      <div className="mt-0.5 text-xs font-medium leading-4">
                        {step.label}
                      </div>
                    </div>
                    {index < 4 ? (
                      <div className="absolute top-6 right-[-5px] z-10 hidden size-2 rotate-45 border-t border-r bg-background sm:block" />
                    ) : null}
                  </div>
                );
              })}
            </div>
          </div>

          <div className="grid gap-3 rounded-md bg-muted/35 p-4 text-xs">
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">
                {t("backup.selectedSnapshot")}
              </span>
              <span className="font-data font-medium">
                {restoreBackup?.backup_id}
              </span>
            </div>
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">
                {t("backup.snapshotSize")}
              </span>
              <span className="font-data font-medium">
                {formatBytes(restoreBackup?.size_bytes ?? 0)}
              </span>
            </div>
            <div className="flex items-center justify-between gap-4 border-t pt-3">
              <Label htmlFor="native-backup-restart" className="grid gap-1">
                <span>{t("backup.restartAfter")}</span>
                <span className="font-normal text-muted-foreground">
                  {control?.configured
                    ? t("backup.restartAfterHint")
                    : t("backup.restartUnavailable")}
                </span>
              </Label>
              <Switch
                id="native-backup-restart"
                checked={restartAfter}
                disabled={!control?.configured}
                onCheckedChange={setRestartAfter}
              />
            </div>
          </div>

          {!control?.configured ? (
            <div className="flex gap-3 rounded-md border border-amber-500/30 bg-amber-500/8 p-3 text-xs leading-5">
              <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-600" />
              <span>{t("backup.manualStopWarning")}</span>
            </div>
          ) : null}

          <AlertDialogFooter>
            <AlertDialogCancel disabled={restoreMutation.isPending}>
              {t("action.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={!restoreBackup?.valid || restoreMutation.isPending}
              onClick={(event) => {
                event.preventDefault();
                if (restoreBackup) {
                  restoreMutation.mutate({
                    backup: restoreBackup,
                    restart: restartAfter,
                  });
                }
              }}
            >
              {restoreMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <ArchiveRestore />
              )}
              {restoreMutation.isPending
                ? t("backup.restoring")
                : t("backup.confirmRestore")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={Boolean(deleteId)}
        onOpenChange={(open) => !open && setDeleteId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("action.delete")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("confirm.delete")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("action.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={removeMutation.isPending}
              onClick={(event) => {
                event.preventDefault();
                if (deleteId) removeMutation.mutate(deleteId);
              }}
            >
              {removeMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : null}
              {t("action.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Panel>
  );
}
