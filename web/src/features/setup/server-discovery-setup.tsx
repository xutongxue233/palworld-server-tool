import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  FolderSearch,
  HardDrive,
  LoaderCircle,
  RefreshCw,
  TriangleAlert,
  WandSparkles,
} from "lucide-react";
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
import { ScrollArea } from "@/components/ui/scroll-area";
import { api, getApiErrorMessage } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";

const setupStatusKey = ["local", "setup", "status"] as const;
const discoveryKey = ["local", "setup", "discovery"] as const;
const candidateSourceKeys: Record<string, string> = {
  "running-process": "setup.source.runningProcess",
  "steam-library": "setup.source.steamLibrary",
  "common-path": "setup.source.commonPath",
  "configured-save": "setup.source.database",
  "configured-control": "setup.source.database",
  "configured-steamcmd": "setup.source.database",
  "configured-mods": "setup.source.database",
  "configured-config": "setup.source.database",
};

export function ServerDiscoverySetup({
  isAuthenticated,
  passwordConfigured,
  onLogin,
  open,
  onOpenChange,
}: {
  isAuthenticated: boolean;
  passwordConfigured: boolean | null;
  onLogin: () => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [selectedID, setSelectedID] = useState("");
  const [manualMode, setManualMode] = useState(false);
  const [manualPath, setManualPath] = useState("");

  const statusQuery = useQuery({
    queryKey: setupStatusKey,
    queryFn: api.getDiscoverySetupStatus,
    staleTime: 30_000,
  });
  const needsSetup = Boolean(statusQuery.data?.needs_setup);
  const restartRequired = Boolean(statusQuery.data?.restart_required);

  useEffect(() => {
    if (needsSetup && isAuthenticated) onOpenChange(true);
  }, [isAuthenticated, needsSetup, onOpenChange]);

  const discoveryQuery = useQuery({
    queryKey: discoveryKey,
    queryFn: api.getServerDiscovery,
    enabled: open && isAuthenticated,
  });

  const candidates = discoveryQuery.data?.candidates ?? [];
  const effectiveSelectedID = selectedID || candidates[0]?.id || "";
  const usingManualMode =
    manualMode || (discoveryQuery.isSuccess && candidates.length === 0);

  const selectedCandidate = candidates.find(
    (candidate) => candidate.id === effectiveSelectedID,
  );

  const updateAfterDiscovery = async () => {
    await queryClient.invalidateQueries({ queryKey: setupStatusKey });
    await queryClient.invalidateQueries({ queryKey: discoveryKey });
    await queryClient.invalidateQueries({ queryKey: ["server-scope"] });
  };

  const scanMutation = useMutation({
    mutationFn: api.scanServerDiscovery,
    onSuccess: (result) => {
      queryClient.setQueryData(discoveryKey, result);
      setSelectedID(result.candidates[0]?.id ?? "");
      setManualMode(result.candidates.length === 0);
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const applyMutation = useMutation({
    mutationFn: () =>
      api.applyServerDiscovery(
        usingManualMode
          ? { install_dir: manualPath.trim() }
          : { candidate_id: effectiveSelectedID },
      ),
    onSuccess: async (result) => {
      toast.success(
        result.restart_required ? t("setup.savedRestart") : t("setup.applied"),
      );
      onOpenChange(false);
      await updateAfterDiscovery();
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const openSetup = () => {
    if (!isAuthenticated) {
      onLogin();
      return;
    }
    onOpenChange(true);
  };

  return (
    <>
      {needsSetup ? (
        <div className="flex flex-col gap-3 border-b border-amber-500/25 bg-amber-500/8 px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div className="flex min-w-0 items-start gap-3">
            <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-500" />
            <div className="min-w-0">
              <p className="text-sm font-medium">{t("setup.bannerTitle")}</p>
              <p className="mt-0.5 text-xs text-muted-foreground">
                {isAuthenticated
                  ? restartRequired
                    ? t("setup.restartDescription")
                    : t("setup.bannerDescription")
                  : t(
                      passwordConfigured === false
                        ? "setup.passwordDescription"
                        : "setup.loginDescription",
                    )}
              </p>
            </div>
          </div>
          <Button size="sm" variant="outline" onClick={openSetup}>
            <FolderSearch />
            {isAuthenticated
              ? t("setup.open")
              : t(passwordConfigured === false ? "auth.setup" : "auth.login")}
          </Button>
        </div>
      ) : null}

      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("setup.title")}</DialogTitle>
            <DialogDescription>{t("setup.description")}</DialogDescription>
          </DialogHeader>

          {discoveryQuery.isPending ? (
            <div className="flex min-h-64 items-center justify-center">
              <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : discoveryQuery.isError ? (
            <div className="rounded-md border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
              {getApiErrorMessage(discoveryQuery.error)}
            </div>
          ) : (
            <div className="grid gap-4">
              <div className="flex items-center justify-between gap-3">
                <p className="text-xs text-muted-foreground">
                  {t("setup.scanSummary", {
                    count: discoveryQuery.data?.candidates.length ?? 0,
                  })}
                </p>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={scanMutation.isPending || applyMutation.isPending}
                  onClick={() => scanMutation.mutate()}
                >
                  <RefreshCw
                    className={cn(scanMutation.isPending && "animate-spin")}
                  />
                  {t("setup.rescan")}
                </Button>
              </div>

              {(discoveryQuery.data?.candidates.length ?? 0) > 0 ? (
                <ScrollArea className="max-h-72">
                  <div className="grid gap-2 pr-3">
                    {discoveryQuery.data?.candidates.map((candidate) => (
                      <button
                        key={candidate.id}
                        type="button"
                        className={cn(
                          "grid w-full gap-2 rounded-md border p-4 text-left transition-colors hover:bg-muted/55 focus-visible:ring-2 focus-visible:ring-ring",
                          !usingManualMode &&
                            effectiveSelectedID === candidate.id
                            ? "border-primary bg-primary/5"
                            : "border-border",
                        )}
                        onClick={() => {
                          setManualMode(false);
                          setSelectedID(candidate.id);
                        }}
                      >
                        <span className="flex items-start justify-between gap-4">
                          <span className="flex min-w-0 items-start gap-3">
                            <span className="flex size-9 shrink-0 items-center justify-center rounded-md border bg-muted">
                              <HardDrive className="size-4" />
                            </span>
                            <span className="min-w-0">
                              <span className="block text-sm font-medium">
                                {candidate.install_dir}
                              </span>
                              <span className="mt-1 block break-all font-mono text-[10px] text-muted-foreground">
                                {candidate.launcher_path}
                              </span>
                            </span>
                          </span>
                          <span className="shrink-0 rounded-full border px-2 py-0.5 text-[10px] text-muted-foreground">
                            {t(
                              candidateSourceKeys[candidate.source] ??
                                candidate.source,
                            )}
                          </span>
                        </span>
                        <span className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                          <span>
                            {t("setup.worlds", {
                              count: candidate.worlds?.length ?? 0,
                            })}
                          </span>
                          <span>
                            {candidate.config_exists
                              ? t("setup.configFound")
                              : t("setup.configPending")}
                          </span>
                          <span>REST {candidate.rest_port}</span>
                        </span>
                      </button>
                    ))}
                  </div>
                </ScrollArea>
              ) : (
                <div className="rounded-md border border-dashed p-5 text-sm text-muted-foreground">
                  {t("setup.notFound")}
                </div>
              )}

              <div className="grid gap-2 rounded-md border bg-muted/25 p-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="text-sm font-medium">
                      {t("setup.manualTitle")}
                    </p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {t("setup.manualDescription")}
                    </p>
                  </div>
                  <Button
                    type="button"
                    size="sm"
                    variant={usingManualMode ? "default" : "outline"}
                    onClick={() => setManualMode((current) => !current)}
                  >
                    {t("setup.manual")}
                  </Button>
                </div>
                {usingManualMode ? (
                  <div className="grid gap-2 pt-2">
                    <Label htmlFor="manual-palserver-path">
                      {t("setup.installDir")}
                    </Label>
                    <Input
                      id="manual-palserver-path"
                      value={manualPath}
                      disabled={applyMutation.isPending}
                      placeholder={t("setup.installPlaceholder")}
                      onChange={(event) => setManualPath(event.target.value)}
                    />
                  </div>
                ) : null}
              </div>

              {discoveryQuery.data?.warnings?.map((warning) => (
                <p key={warning} className="text-xs text-amber-600">
                  {warning}
                </p>
              ))}

              <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
                <Button
                  type="button"
                  variant="outline"
                  disabled={applyMutation.isPending}
                  onClick={() => onOpenChange(false)}
                >
                  {t("action.cancel")}
                </Button>
                <Button
                  type="button"
                  disabled={
                    applyMutation.isPending ||
                    (usingManualMode
                      ? manualPath.trim() === ""
                      : !selectedCandidate)
                  }
                  onClick={() => applyMutation.mutate()}
                >
                  {applyMutation.isPending ? (
                    <LoaderCircle className="animate-spin" />
                  ) : (
                    <WandSparkles />
                  )}
                  {t("setup.apply")}
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
