import { useMemo, useState } from "react";
import { LoaderCircle, Plus, Save, ShieldCheck, Trash2 } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { ErrorState, LoadingState } from "@/components/common/data-state";
import { Panel } from "@/components/common/panel";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
import { useI18n } from "@/lib/i18n";
import type { WhitelistPlayer } from "@/types/api";

function emptyEntry(): WhitelistPlayer {
  return { name: "", player_uid: "", user_id: "", steam_id: "" };
}

export function WhitelistPanel() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const whitelistQuery = useQuery({
    queryKey: queryKeys.whitelist,
    queryFn: scopedQueryFn(api.getWhitelist),
  });
  const [draftRows, setDraftRows] = useState<WhitelistPlayer[] | null>(null);
  const rows = useMemo(
    () => draftRows ?? whitelistQuery.data ?? [],
    [draftRows, whitelistQuery.data],
  );
  const dirty = draftRows !== null;

  const invalidRows = useMemo(
    () =>
      rows.filter((row) => !row.player_uid && !row.user_id && !row.steam_id),
    [rows],
  );

  const saveMutation = useMutation({
    mutationFn: () => api.replaceWhitelist(rows),
    onSuccess: async () => {
      toast.success(t("message.updated"));
      setDraftRows(null);
      await queryClient.invalidateQueries({ queryKey: queryKeys.whitelist });
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const updateRow = (
    index: number,
    key: keyof WhitelistPlayer,
    value: string,
  ) => {
    setDraftRows(
      rows.map((row, rowIndex) =>
        rowIndex === index ? { ...row, [key]: value } : row,
      ),
    );
  };

  if (whitelistQuery.isPending) return <LoadingState />;
  if (whitelistQuery.isError) {
    return (
      <ErrorState
        error={whitelistQuery.error}
        retry={() => void whitelistQuery.refetch()}
      />
    );
  }

  return (
    <Panel
      title={t("operations.whitelist")}
      description={`${rows.length} ${t("whitelist.name")}`}
      actions={
        <div className="flex items-center gap-2">
          {dirty ? (
            <Badge variant="secondary">{t("whitelist.unsaved")}</Badge>
          ) : null}
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setDraftRows([emptyEntry(), ...rows]);
            }}
          >
            <Plus /> {t("whitelist.add")}
          </Button>
          <Button
            size="sm"
            disabled={
              !dirty || invalidRows.length > 0 || saveMutation.isPending
            }
            onClick={() => saveMutation.mutate()}
          >
            {saveMutation.isPending ? (
              <LoaderCircle className="animate-spin" />
            ) : (
              <Save />
            )}
            {t("whitelist.save")}
          </Button>
        </div>
      }
    >
      {invalidRows.length > 0 ? (
        <div
          role="alert"
          className="border-b bg-destructive/8 px-4 py-3 text-sm text-destructive sm:px-5"
        >
          {t("whitelist.identityRequired")}
        </div>
      ) : null}
      <div className="overflow-x-auto">
        <Table className="min-w-[880px]">
          <TableHeader>
            <TableRow>
              <TableHead className="w-[180px]">{t("whitelist.name")}</TableHead>
              <TableHead>{t("whitelist.playerUid")}</TableHead>
              <TableHead>{t("whitelist.userId")}</TableHead>
              <TableHead>{t("whitelist.steamId")}</TableHead>
              <TableHead className="w-16" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row, index) => (
              <TableRow
                key={`${row.player_uid}-${row.user_id}-${row.steam_id}-${index}`}
              >
                <TableCell>
                  <div className="flex items-center gap-2">
                    <ShieldCheck className="size-4 shrink-0 text-primary" />
                    <Input
                      value={row.name}
                      onChange={(event) =>
                        updateRow(index, "name", event.target.value)
                      }
                      className="h-8"
                    />
                  </div>
                </TableCell>
                <TableCell>
                  <Input
                    value={row.player_uid}
                    onChange={(event) =>
                      updateRow(index, "player_uid", event.target.value)
                    }
                    className="font-data h-8 text-xs"
                  />
                </TableCell>
                <TableCell>
                  <Input
                    value={row.user_id}
                    onChange={(event) =>
                      updateRow(index, "user_id", event.target.value)
                    }
                    className="font-data h-8 text-xs"
                  />
                </TableCell>
                <TableCell>
                  <Input
                    value={row.steam_id}
                    onChange={(event) =>
                      updateRow(index, "steam_id", event.target.value)
                    }
                    className="font-data h-8 text-xs"
                  />
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => {
                      setDraftRows(
                        rows.filter((_, rowIndex) => rowIndex !== index),
                      );
                    }}
                  >
                    <Trash2 />
                    <span className="sr-only">{t("action.remove")}</span>
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {rows.length === 0 ? (
        <div className="flex min-h-48 items-center justify-center text-sm text-muted-foreground">
          {t("message.empty")}
        </div>
      ) : null}
    </Panel>
  );
}
