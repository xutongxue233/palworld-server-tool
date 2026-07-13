import { useState } from "react";
import {
  CalendarRange,
  Download,
  FileArchive,
  LoaderCircle,
  RefreshCw,
  Search,
  Trash2,
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
import { queryKeys } from "@/hooks/use-server-data";
import { api, getApiErrorMessage } from "@/lib/api";
import { downloadBlob, formatDateTime } from "@/lib/format";
import { useI18n } from "@/lib/i18n";

function toInputValue(timestamp: number) {
  const date = new Date(timestamp);
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
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

  const backupsQuery = useQuery({
    queryKey: queryKeys.backups(range.start, range.end),
    queryFn: () => api.getBackups(range.start, range.end),
  });

  const removeMutation = useMutation({
    mutationFn: api.removeBackup,
    onSuccess: async () => {
      toast.success(t("message.deleted"));
      setDeleteId(null);
      await queryClient.invalidateQueries({ queryKey: ["backups"] });
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

  return (
    <Panel
      title={t("operations.backups")}
      description={t("backup.description")}
      actions={
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => void backupsQuery.refetch()}
        >
          <RefreshCw />
          <span className="sr-only">{t("action.refresh")}</span>
        </Button>
      }
    >
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
                        <span className="sr-only">{t("action.download")}</span>
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
        <div className="flex min-h-48 items-center justify-center text-sm text-muted-foreground">
          {t("message.empty")}
        </div>
      )}

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
