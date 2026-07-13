import { useMemo, useState, type ReactNode } from "react";
import {
  ArrowRight,
  Boxes,
  CircleCheck,
  CircleDashed,
  ExternalLink,
  FileJson,
  Folder,
  Link2,
  LoaderCircle,
  Package,
  PackageCheck,
  Power,
  RefreshCw,
  RotateCw,
  Search,
  ShieldCheck,
  TriangleAlert,
  Undo2,
  X,
  type LucideIcon,
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
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { queryKeys } from "@/hooks/use-server-data";
import { api, getApiErrorCode, getApiErrorMessage } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type {
  OfficialModApplyResult,
  OfficialModChangePlan,
  OfficialModDiagnostic,
  OfficialModInventory,
  OfficialModPackage,
  OfficialModSettings,
} from "@/types/api";

const OFFICIAL_MOD_DOCS =
  "https://docs.palworldgame.com/settings-and-operation/mod";

function DeploymentStage({
  icon: Icon,
  label,
  value,
  state,
  arrow = true,
}: {
  icon: LucideIcon;
  label: string;
  value: string | number;
  state: "source" | "selected" | "deployed";
  arrow?: boolean;
}) {
  return (
    <div className="contents">
      <div
        className={cn(
          "relative min-w-0 rounded-md border bg-card p-4",
          state === "source" && "border-accent/30",
          state === "selected" && "border-primary/30",
          state === "deployed" && "border-emerald-500/30",
        )}
      >
        <div className="flex items-center justify-between gap-3">
          <div
            className={cn(
              "flex size-9 items-center justify-center rounded-md border bg-muted/35",
              state === "source" && "text-accent",
              state === "selected" && "text-primary",
              state === "deployed" && "text-emerald-600 dark:text-emerald-400",
            )}
          >
            <Icon className="size-4" />
          </div>
          <span className="font-data text-2xl font-semibold tabular-nums">
            {value}
          </span>
        </div>
        <div className="mt-3 text-[10px] font-semibold tracking-[0.14em] text-muted-foreground uppercase">
          {label}
        </div>
      </div>
      {arrow ? (
        <div className="hidden items-center justify-center lg:flex">
          <ArrowRight className="size-4 text-muted-foreground/60" />
        </div>
      ) : null}
    </div>
  );
}

function DiagnosticList({
  title,
  diagnostics,
  tone,
  renderDiagnostic,
}: {
  title: string;
  diagnostics?: OfficialModDiagnostic[];
  tone: "warning" | "danger";
  renderDiagnostic: (diagnostic: OfficialModDiagnostic) => string;
}) {
  if (!diagnostics?.length) return null;
  return (
    <section
      className={cn(
        "rounded-md border p-4",
        tone === "danger"
          ? "border-destructive/30 bg-destructive/6"
          : "border-amber-500/30 bg-amber-500/7",
      )}
    >
      <div className="flex items-center gap-2 text-xs font-semibold">
        <TriangleAlert
          className={cn(
            "size-4",
            tone === "danger"
              ? "text-destructive"
              : "text-amber-600 dark:text-amber-400",
          )}
        />
        {title}
      </div>
      <ul className="mt-2 space-y-1.5 pl-6 text-xs leading-5 text-muted-foreground">
        {diagnostics.map((diagnostic, index) => (
          <li key={`${diagnostic.code}-${diagnostic.package_name}-${index}`}>
            {renderDiagnostic(diagnostic)}
          </li>
        ))}
      </ul>
    </section>
  );
}

function PackageStatusBadge({
  pkg,
  listed,
  effective,
  t,
}: {
  pkg: OfficialModPackage;
  listed: boolean;
  effective: boolean;
  t: (key: string, variables?: Record<string, string | number>) => string;
}) {
  if (!pkg.valid) {
    return <Badge variant="destructive">{t("mods.packageInvalid")}</Badge>;
  }
  if (!pkg.server_compatible) {
    return <Badge variant="secondary">{t("mods.clientOnly")}</Badge>;
  }
  if (effective && pkg.deployed) {
    return (
      <Badge className="border-emerald-500/25 bg-emerald-500/12 text-emerald-700 dark:text-emerald-300">
        <CircleCheck /> {t("mods.deployed")}
      </Badge>
    );
  }
  if (effective && !pkg.deployed) {
    return (
      <Badge className="border-amber-500/25 bg-amber-500/12 text-amber-700 dark:text-amber-300">
        <RotateCw /> {t("mods.pendingRestart")}
      </Badge>
    );
  }
  if (!listed && pkg.deployed) {
    return (
      <Badge
        variant="outline"
        className="border-amber-500/30 text-amber-700 dark:text-amber-300"
      >
        <Undo2 /> {t("mods.pendingRemoval")}
      </Badge>
    );
  }
  if (listed) {
    return <Badge variant="outline">{t("mods.selectedInactive")}</Badge>;
  }
  return <Badge variant="secondary">{t("mods.available")}</Badge>;
}

function PackageCard({
  pkg,
  listed,
  effective,
  disabled,
  onToggle,
  renderDiagnostic,
  t,
}: {
  pkg: OfficialModPackage;
  listed: boolean;
  effective: boolean;
  disabled: boolean;
  onToggle: (next: boolean) => void;
  renderDiagnostic: (diagnostic: OfficialModDiagnostic) => string;
  t: (key: string, variables?: Record<string, string | number>) => string;
}) {
  const diagnostics = [...(pkg.issues ?? []), ...(pkg.warnings ?? [])];
  return (
    <article
      className={cn(
        "group relative overflow-hidden rounded-md border bg-card transition-colors",
        listed && "border-primary/40",
        !pkg.valid && "border-destructive/30",
      )}
    >
      <div
        className={cn(
          "absolute inset-y-0 left-0 w-1",
          !pkg.valid
            ? "bg-destructive"
            : effective && pkg.deployed
              ? "bg-emerald-500"
              : listed
                ? "bg-primary"
                : "bg-border",
        )}
      />
      <div className="p-4 pl-5 sm:p-5 sm:pl-6">
        <div className="flex items-start gap-3">
          <Checkbox
            className="mt-1"
            checked={listed}
            disabled={disabled}
            aria-label={t("mods.togglePackage", {
              name: pkg.mod_name || pkg.package_name || pkg.folder_name,
            })}
            onCheckedChange={(checked) => onToggle(checked === true)}
          />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="min-w-0">
                <h3 className="font-display truncate text-sm font-semibold tracking-[0.01em]">
                  {pkg.mod_name || pkg.package_name || pkg.folder_name}
                </h3>
                <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-muted-foreground">
                  <span className="font-data break-all">
                    {pkg.package_name || pkg.folder_name}
                  </span>
                  {pkg.version ? <span>v{pkg.version}</span> : null}
                  {pkg.author ? <span>{pkg.author}</span> : null}
                </div>
              </div>
              <PackageStatusBadge
                pkg={pkg}
                listed={listed}
                effective={effective}
                t={t}
              />
            </div>

            <div className="mt-3 flex flex-wrap gap-1.5">
              {(pkg.server_install_types?.length
                ? pkg.server_install_types
                : (pkg.install_types ?? [])
              ).map((type) => (
                <Badge
                  key={type}
                  variant="outline"
                  className="font-data text-[10px]"
                >
                  {type}
                </Badge>
              ))}
              {pkg.debug_mode ? (
                <Badge variant="destructive" className="text-[10px]">
                  DEBUG
                </Badge>
              ) : null}
              {pkg.min_revision !== undefined ? (
                <Badge variant="secondary" className="font-data text-[10px]">
                  REV ≥ {pkg.min_revision}
                </Badge>
              ) : null}
              {pkg.workshop_item_id ? (
                <Badge variant="secondary" className="font-data text-[10px]">
                  #{pkg.workshop_item_id}
                </Badge>
              ) : null}
            </div>

            {pkg.dependencies?.length ? (
              <div className="mt-3 flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
                <Link2 className="size-3.5" />
                <span>{t("mods.dependencies")}</span>
                {pkg.dependencies.map((dependency) => (
                  <span
                    key={dependency}
                    className="font-data rounded border bg-muted/35 px-1.5 py-0.5 text-[10px]"
                  >
                    {dependency}
                  </span>
                ))}
              </div>
            ) : null}

            {diagnostics.length ? (
              <div className="mt-3 space-y-1 border-t pt-3 text-[11px] leading-4 text-muted-foreground">
                {diagnostics.slice(0, 3).map((diagnostic, index) => (
                  <div
                    key={`${diagnostic.code}-${index}`}
                    className="flex items-start gap-1.5"
                  >
                    <TriangleAlert className="mt-0.5 size-3 shrink-0 text-amber-600 dark:text-amber-400" />
                    <span>{renderDiagnostic(diagnostic)}</span>
                  </div>
                ))}
              </div>
            ) : null}

            <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t pt-3 text-[10px] text-muted-foreground">
              <span className="font-data min-w-0 truncate" title={pkg.path}>
                {pkg.folder_name}
              </span>
              {pkg.workshop_item_id ? (
                <a
                  href={`https://steamcommunity.com/sharedfiles/filedetails/?id=${pkg.workshop_item_id}`}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 text-primary hover:underline"
                >
                  {t("mods.openWorkshop")} <ExternalLink className="size-3" />
                </a>
              ) : null}
            </div>
          </div>
        </div>
      </div>
    </article>
  );
}

export function OfficialModsPanel() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [globalEnabled, setGlobalEnabled] = useState(false);
  const [workshopRoot, setWorkshopRoot] = useState("");
  const [activeMods, setActiveMods] = useState<string[]>([]);
  const [loadedDigest, setLoadedDigest] = useState("");
  const [search, setSearch] = useState("");
  const [plan, setPlan] = useState<OfficialModChangePlan | null>(null);
  const [restartPreference, setRestartPreference] = useState<boolean | null>(
    null,
  );
  const [manualStopConfirmed, setManualStopConfirmed] = useState(false);
  const [riskConfirmed, setRiskConfirmed] = useState(false);
  const [confirmationOpen, setConfirmationOpen] = useState(false);
  const [lastResult, setLastResult] = useState<OfficialModApplyResult | null>(
    null,
  );

  const statusQuery = useQuery({
    queryKey: queryKeys.officialMods,
    queryFn: api.getOfficialMods,
    refetchInterval: 15_000,
  });
  const controlQuery = useQuery({
    queryKey: queryKeys.control,
    queryFn: api.getServerControlStatus,
    refetchInterval: 10_000,
  });
  const status = statusQuery.data;
  const control = controlQuery.data;

  if (status && status.status_digest !== loadedDigest) {
    setGlobalEnabled(status.settings.global_enabled);
    setWorkshopRoot(status.settings.workshop_root_dir ?? "");
    setActiveMods(status.settings.active_mod_list ?? []);
    setLoadedDigest(status.status_digest);
    setPlan(null);
  }

  const desiredSettings: OfficialModSettings = useMemo(
    () => ({
      global_enabled: globalEnabled,
      workshop_root_dir: workshopRoot.trim(),
      active_mod_list: activeMods,
    }),
    [activeMods, globalEnabled, workshopRoot],
  );

  const inventory: OfficialModInventory | undefined =
    plan?.target_inventory ?? status?.inventory;
  const activeSet = useMemo(
    () => new Set(activeMods.map((name) => name.toLocaleLowerCase())),
    [activeMods],
  );
  const packageMap = useMemo(() => {
    const map = new Map<string, OfficialModPackage>();
    for (const pkg of inventory?.packages ?? []) {
      if (pkg.package_name && !map.has(pkg.package_name.toLocaleLowerCase())) {
        map.set(pkg.package_name.toLocaleLowerCase(), pkg);
      }
    }
    return map;
  }, [inventory?.packages]);

  const dirty = Boolean(
    status &&
    (status.settings.global_enabled !== globalEnabled ||
      (status.settings.workshop_root_dir ?? "") !== workshopRoot.trim() ||
      status.settings.active_mod_list.join("\u0000").toLocaleLowerCase() !==
        activeMods.join("\u0000").toLocaleLowerCase()),
  );

  const renderDiagnostic = (diagnostic: OfficialModDiagnostic) => {
    const key = `mods.notice.${diagnostic.code}`;
    const translated = t(key, {
      package: diagnostic.package_name || diagnostic.folder_name || "--",
      dependency: diagnostic.dependency || "--",
    });
    return translated === key ? diagnostic.message : translated;
  };

  const invalidatePlan = () => {
    setPlan(null);
    setRiskConfirmed(false);
  };

  const resetDraft = () => {
    if (!status) return;
    setGlobalEnabled(status.settings.global_enabled);
    setWorkshopRoot(status.settings.workshop_root_dir ?? "");
    setActiveMods(status.settings.active_mod_list ?? []);
    setPlan(null);
    setRiskConfirmed(false);
  };

  const togglePackage = (pkg: OfficialModPackage, next: boolean) => {
    if (!pkg.package_name) return;
    const packageName = pkg.package_name;
    if (!next) {
      setActiveMods((current) =>
        current.filter(
          (name) =>
            name.toLocaleLowerCase() !== packageName.toLocaleLowerCase(),
        ),
      );
      invalidatePlan();
      return;
    }

    const additions: string[] = [];
    const visit = (name: string, trail = new Set<string>()) => {
      const key = name.toLocaleLowerCase();
      if (trail.has(key)) return;
      trail.add(key);
      const dependencyPackage = packageMap.get(key);
      if (dependencyPackage) {
        for (const dependency of dependencyPackage.dependencies ?? []) {
          visit(dependency, trail);
        }
        if (dependencyPackage.package_name) {
          additions.push(dependencyPackage.package_name);
        }
      } else {
        additions.push(name);
      }
    };
    visit(packageName);
    setActiveMods((current) => {
      const seen = new Set(current.map((name) => name.toLocaleLowerCase()));
      const result = [...current];
      for (const addition of additions) {
        const key = addition.toLocaleLowerCase();
        if (!seen.has(key)) {
          seen.add(key);
          result.push(addition);
        }
      }
      return result;
    });
    invalidatePlan();
  };

  const removeUnknownActive = (name: string) => {
    setActiveMods((current) =>
      current.filter(
        (entry) => entry.toLocaleLowerCase() !== name.toLocaleLowerCase(),
      ),
    );
    invalidatePlan();
  };

  const preflightMutation = useMutation({
    mutationFn: () => api.preflightOfficialMods(desiredSettings),
    onSuccess: (result) => {
      setPlan(result.plan);
      if (result.plan.can_apply) {
        toast.success(t("mods.preflightReady"), {
          description: result.plan.changed
            ? t("mods.preflightChanged")
            : t("mods.preflightUnchanged"),
        });
      } else {
        toast.warning(t("mods.preflightBlocked"));
      }
    },
    onError: (error) => toast.error(getApiErrorMessage(error)),
  });

  const restartAfter = Boolean(
    control?.configured &&
    (restartPreference ?? Boolean(control.online || control.running)),
  );
  const manuallyManaged = !control?.configured;
  const serverStillOnline = Boolean(control?.online || control?.running);
  const serverReady = control?.configured
    ? !control.busy
    : !serverStillOnline && manualStopConfirmed;

  const applyMutation = useMutation({
    mutationFn: () => {
      if (!plan) throw new Error(t("mods.planUnavailable"));
      return api.applyOfficialMods({
        ...plan.desired_settings,
        expected_plan_digest: plan.plan_digest,
        confirm_apply: true,
        confirm_mod_risk: riskConfirmed,
        confirm_server_stopped: Boolean(
          !control?.configured && manualStopConfirmed,
        ),
        restart_after: restartAfter,
        shutdown_seconds: 10,
        shutdown_message: t("mods.shutdownMessage"),
      });
    },
    onSuccess: async (result) => {
      setLastResult(result);
      setConfirmationOpen(false);
      setRiskConfirmed(false);
      toast.success(t("mods.applySuccess"), {
        description: result.restarted
          ? t("mods.applySuccessRestarted")
          : t("mods.applySuccessStopped"),
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.officialMods }),
        queryClient.invalidateQueries({ queryKey: queryKeys.control }),
        queryClient.invalidateQueries({ queryKey: queryKeys.automationStatus }),
        queryClient.invalidateQueries({ queryKey: ["backups"] }),
      ]);
    },
    onError: async (error) => {
      const code = getApiErrorCode(error);
      if (code === "official_mod_restart_failed_rolled_back") {
        toast.error(t("mods.runtimeRejected"), {
          description: t("mods.runtimeRejectedHint"),
        });
      } else {
        toast.error(getApiErrorMessage(error));
      }
      if (code === "official_mod_plan_changed") {
        setConfirmationOpen(false);
        setPlan(null);
      }
      await Promise.all([statusQuery.refetch(), controlQuery.refetch()]);
    },
  });

  const filteredPackages = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase();
    const packages = inventory?.packages ?? [];
    if (!needle) return packages;
    return packages.filter((pkg) =>
      [
        pkg.mod_name,
        pkg.package_name,
        pkg.author,
        pkg.folder_name,
        pkg.version,
        ...(pkg.tags ?? []),
      ]
        .filter(Boolean)
        .some((value) => String(value).toLocaleLowerCase().includes(needle)),
    );
  }, [inventory?.packages, search]);

  if (statusQuery.isPending || controlQuery.isPending) {
    return (
      <Panel title={t("mods.title")} description={t("mods.description")}>
        <LoadingState />
      </Panel>
    );
  }
  if (statusQuery.isError || controlQuery.isError || !status || !control) {
    return (
      <Panel title={t("mods.title")} description={t("mods.description")}>
        <ErrorState
          error={
            statusQuery.error ??
            controlQuery.error ??
            new Error(t("mods.statusUnavailable"))
          }
          retry={() => {
            void statusQuery.refetch();
            void controlQuery.refetch();
          }}
        />
      </Panel>
    );
  }

  const selectedCount = activeMods.length;
  const deployedCount = (inventory?.packages ?? []).filter(
    (pkg) => pkg.deployed,
  ).length;
  const selectableCount = (inventory?.packages ?? []).filter(
    (pkg) => pkg.valid && pkg.server_compatible,
  ).length;
  const unknownActive = activeMods.filter(
    (name) => !packageMap.has(name.toLocaleLowerCase()),
  );
  const canOpenConfirmation = Boolean(
    plan?.can_apply &&
    plan.changed &&
    serverReady &&
    !preflightMutation.isPending &&
    !applyMutation.isPending,
  );

  const stageItems: Array<{
    icon: LucideIcon;
    label: string;
    value: string | number;
    state: "source" | "selected" | "deployed";
  }> = [
    {
      icon: FileJson,
      label: t("mods.stageDiscovered"),
      value: inventory?.packages.length ?? 0,
      state: "source",
    },
    {
      icon: Boxes,
      label: t("mods.stageSelected"),
      value: selectedCount,
      state: "selected",
    },
    {
      icon: PackageCheck,
      label: t("mods.stageDeployed"),
      value: deployedCount,
      state: "deployed",
    },
  ];

  return (
    <div className="space-y-4">
      <Panel
        title={t("mods.title")}
        description={t("mods.description")}
        actions={
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="icon-sm" asChild>
              <a
                href={OFFICIAL_MOD_DOCS}
                target="_blank"
                rel="noreferrer"
                aria-label={t("mods.officialGuide")}
              >
                <ExternalLink />
              </a>
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              disabled={statusQuery.isFetching || applyMutation.isPending}
              onClick={() => void statusQuery.refetch()}
              aria-label={t("action.refresh")}
            >
              <RefreshCw
                className={cn(statusQuery.isFetching && "animate-spin")}
              />
            </Button>
          </div>
        }
        contentClassName="p-4 sm:p-5"
      >
        <div className="grid gap-3 lg:grid-cols-[1fr_auto_1fr_auto_1fr]">
          {stageItems.map((item, index) => (
            <DeploymentStage
              key={item.label}
              {...item}
              arrow={index < stageItems.length - 1}
            />
          ))}
        </div>
        <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t pt-4 text-xs text-muted-foreground">
          <span>{t("mods.officialLoaderFlow")}</span>
          <Badge variant="outline" className="font-data">
            Palworld {status.game_version}
          </Badge>
        </div>
      </Panel>

      {!status.supported ? (
        <Panel contentClassName="p-5">
          <div className="flex items-start gap-3">
            <TriangleAlert className="mt-0.5 size-5 shrink-0 text-amber-600 dark:text-amber-400" />
            <div>
              <h3 className="text-sm font-semibold">
                {t("mods.windowsOnlyTitle")}
              </h3>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {t("mods.windowsOnlyHint", { platform: status.platform })}
              </p>
            </div>
          </div>
        </Panel>
      ) : null}

      {!status.configured ? (
        <Panel contentClassName="p-5">
          <div className="flex items-start gap-3">
            <Folder className="mt-0.5 size-5 shrink-0 text-primary" />
            <div>
              <h3 className="text-sm font-semibold">
                {t("mods.notConfigured")}
              </h3>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {t("mods.configHint")}
              </p>
            </div>
          </div>
        </Panel>
      ) : null}

      {status.forced_disabled ? (
        <div className="flex gap-3 rounded-md border border-amber-500/30 bg-amber-500/7 p-4 text-xs leading-5">
          <Power className="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-400" />
          <div>
            <div className="font-semibold">{t("mods.noModsTitle")}</div>
            <div className="mt-0.5 text-muted-foreground">
              {t("mods.noModsHint")}
            </div>
          </div>
        </div>
      ) : null}

      <Panel
        title={t("mods.settingsTitle")}
        description={t("mods.settingsDescription")}
        contentClassName="p-4 sm:p-5"
      >
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="space-y-5">
            <div className="flex items-start justify-between gap-4 rounded-md border bg-muted/20 p-4">
              <div>
                <Label htmlFor="official-mod-global">
                  {t("mods.globalEnabled")}
                </Label>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  {t("mods.globalEnabledHint")}
                </p>
              </div>
              <Switch
                id="official-mod-global"
                checked={globalEnabled}
                disabled={!status.manageable || applyMutation.isPending}
                onCheckedChange={(checked) => {
                  setGlobalEnabled(checked);
                  invalidatePlan();
                }}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="official-mod-root">
                {t("mods.workshopRoot")}
              </Label>
              <Input
                id="official-mod-root"
                className="font-data text-xs"
                value={workshopRoot}
                disabled={
                  !status.manageable ||
                  Boolean(status.launch_workshop_root) ||
                  applyMutation.isPending
                }
                placeholder={
                  status.install_dir
                    ? `${status.install_dir}\\Mods\\Workshop`
                    : ""
                }
                onChange={(event) => {
                  setWorkshopRoot(event.target.value);
                  invalidatePlan();
                }}
              />
              <p className="text-xs leading-5 text-muted-foreground">
                {status.launch_workshop_root
                  ? t("mods.workshopLaunchOverride", {
                      path: status.launch_workshop_root,
                    })
                  : t("mods.workshopRootHint")}
              </p>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-md border bg-muted/20 p-3">
                <div className="text-[9px] tracking-[0.14em] text-muted-foreground uppercase">
                  {t("mods.settingsFile")}
                </div>
                <div className="font-data mt-1 break-all text-[11px]">
                  {status.settings_path || "--"}
                </div>
              </div>
              <div className="rounded-md border bg-muted/20 p-3">
                <div className="text-[9px] tracking-[0.14em] text-muted-foreground uppercase">
                  {t("mods.workshopSource")}
                </div>
                <div className="font-data mt-1 break-all text-[11px]">
                  {inventory?.workshop_source || "--"}
                </div>
              </div>
            </div>
          </div>

          <div className="rounded-md border bg-muted/20 p-4">
            <div className="flex items-center gap-2 text-xs font-semibold">
              <ShieldCheck className="size-4 text-primary" />
              {t("mods.safetyTitle")}
            </div>
            <div className="mt-4 space-y-3 text-xs">
              <SafetyLine
                label={t("mods.serverRules")}
                value={t("mods.serverRulesValue")}
              />
              <SafetyLine
                label={t("mods.downloadPolicy")}
                value={t("mods.downloadPolicyValue")}
              />
              <SafetyLine
                label={t("mods.saveRecovery")}
                value={
                  status.existing_worlds > 0
                    ? status.safety_backup_ready
                      ? t("mods.backupReady", { count: status.existing_worlds })
                      : t("mods.backupMissing", {
                          count: status.existing_worlds,
                        })
                    : t("mods.backupNotRequired")
                }
                danger={
                  status.existing_worlds > 0 && !status.safety_backup_ready
                }
              />
            </div>
          </div>
        </div>
      </Panel>

      <Panel
        title={t("mods.catalogTitle")}
        description={t("mods.catalogDescription", {
          compatible: selectableCount,
          total: inventory?.packages.length ?? 0,
        })}
        actions={
          dirty ? (
            <Button variant="ghost" size="sm" onClick={resetDraft}>
              <Undo2 /> {t("mods.resetDraft")}
            </Button>
          ) : undefined
        }
        contentClassName="p-4 sm:p-5"
      >
        <div className="relative mb-4">
          <Search className="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-9"
            value={search}
            placeholder={t("mods.search")}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>

        {unknownActive.length ? (
          <div className="mb-4 rounded-md border border-amber-500/30 bg-amber-500/7 p-4">
            <div className="text-xs font-semibold">
              {t("mods.unknownActive")}
            </div>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              {t("mods.unknownActiveHint")}
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              {unknownActive.map((name) => (
                <button
                  key={name}
                  type="button"
                  className="font-data inline-flex items-center gap-1 rounded-full border bg-card px-2.5 py-1 text-[10px] hover:border-destructive/50"
                  onClick={() => removeUnknownActive(name)}
                >
                  {name} <X className="size-3" />
                </button>
              ))}
            </div>
          </div>
        ) : null}

        {!inventory?.workshop_available ? (
          <div className="flex min-h-48 items-center justify-center rounded-md border border-dashed bg-muted/15 p-6 text-center">
            <div className="max-w-lg">
              <Folder className="mx-auto size-7 text-muted-foreground" />
              <h3 className="mt-3 text-sm font-semibold">
                {t("mods.workshopMissingTitle")}
              </h3>
              <p className="mt-1 text-xs leading-5 text-muted-foreground">
                {t("mods.workshopMissingHint", {
                  path: inventory?.workshop_root || "--",
                })}
              </p>
            </div>
          </div>
        ) : filteredPackages.length ? (
          <div className="grid gap-3 xl:grid-cols-2">
            {filteredPackages.map((pkg) => {
              const listed = Boolean(
                pkg.package_name &&
                activeSet.has(pkg.package_name.toLocaleLowerCase()),
              );
              const effective =
                globalEnabled && !status.forced_disabled && listed;
              return (
                <PackageCard
                  key={`${pkg.folder_name}-${pkg.info_sha256}`}
                  pkg={pkg}
                  listed={listed}
                  effective={effective}
                  disabled={
                    !status.manageable ||
                    !pkg.valid ||
                    !pkg.server_compatible ||
                    applyMutation.isPending
                  }
                  onToggle={(next) => togglePackage(pkg, next)}
                  renderDiagnostic={renderDiagnostic}
                  t={t}
                />
              );
            })}
          </div>
        ) : (
          <div className="flex min-h-40 items-center justify-center rounded-md border border-dashed p-6 text-center text-xs text-muted-foreground">
            {search ? t("mods.noSearchResults") : t("mods.noPackages")}
          </div>
        )}
      </Panel>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <Panel
          title={t("mods.preflightTitle")}
          description={t("mods.preflightDescription")}
          contentClassName="p-4 sm:p-5"
        >
          {!plan ? (
            <div className="flex min-h-44 items-center justify-center rounded-md border border-dashed bg-muted/15 p-6 text-center">
              <div className="max-w-lg">
                <CircleDashed className="mx-auto size-7 text-muted-foreground" />
                <h3 className="mt-3 text-sm font-semibold">
                  {dirty ? t("mods.preflightNeeded") : t("mods.noChanges")}
                </h3>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  {t("mods.preflightHint")}
                </p>
              </div>
            </div>
          ) : (
            <div className="space-y-3">
              <div
                className={cn(
                  "flex items-start gap-3 rounded-md border p-4",
                  plan.can_apply
                    ? "border-emerald-500/30 bg-emerald-500/7"
                    : "border-destructive/30 bg-destructive/6",
                )}
              >
                {plan.can_apply ? (
                  <CircleCheck className="mt-0.5 size-5 shrink-0 text-emerald-600 dark:text-emerald-400" />
                ) : (
                  <TriangleAlert className="mt-0.5 size-5 shrink-0 text-destructive" />
                )}
                <div>
                  <div className="text-sm font-semibold">
                    {plan.can_apply
                      ? plan.changed
                        ? t("mods.preflightReady")
                        : t("mods.preflightUnchanged")
                      : t("mods.preflightBlocked")}
                  </div>
                  <div className="font-data mt-1 text-[10px] text-muted-foreground">
                    {plan.plan_digest.slice(0, 16)}…
                  </div>
                </div>
              </div>
              <DiagnosticList
                title={t("mods.blockers")}
                diagnostics={plan.issues}
                tone="danger"
                renderDiagnostic={renderDiagnostic}
              />
              <DiagnosticList
                title={t("mods.warnings")}
                diagnostics={plan.warnings}
                tone="warning"
                renderDiagnostic={renderDiagnostic}
              />
            </div>
          )}

          <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t pt-4">
            <div className="text-xs text-muted-foreground">
              {dirty ? t("mods.draftChanged") : t("mods.draftCurrent")}
            </div>
            <Button
              variant="outline"
              disabled={
                !status.manageable ||
                preflightMutation.isPending ||
                applyMutation.isPending
              }
              onClick={() => preflightMutation.mutate()}
            >
              {preflightMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <ShieldCheck />
              )}
              {preflightMutation.isPending
                ? t("mods.preflighting")
                : t("mods.preflightAction")}
            </Button>
          </div>
        </Panel>

        <Panel
          title={t("mods.serverState")}
          description={t("mods.serverStateDescription")}
          contentClassName="p-4 sm:p-5"
        >
          <div className="space-y-4">
            <div className="rounded-md border bg-muted/20 p-4">
              <div className="flex items-start gap-3">
                <Power
                  className={cn(
                    "mt-0.5 size-4 shrink-0",
                    serverStillOnline ? "text-amber-600" : "text-primary",
                  )}
                />
                <div className="text-xs leading-5">
                  <div className="font-semibold">
                    {control.configured
                      ? t("mods.managedState", { state: control.state })
                      : t("mods.manualState")}
                  </div>
                  <div className="mt-0.5 text-muted-foreground">
                    {control.configured
                      ? t("mods.managedStateHint")
                      : serverStillOnline
                        ? t("mods.manualOnlineBlocked")
                        : t("mods.manualStateHint")}
                  </div>
                </div>
              </div>
            </div>

            {control.configured ? (
              <div className="flex items-start justify-between gap-4">
                <div>
                  <Label htmlFor="official-mod-restart">
                    {t("mods.restartAfter")}
                  </Label>
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">
                    {t("mods.restartAfterHint")}
                  </p>
                </div>
                <Switch
                  id="official-mod-restart"
                  checked={restartAfter}
                  disabled={applyMutation.isPending}
                  onCheckedChange={setRestartPreference}
                />
              </div>
            ) : (
              <label className="flex items-start gap-3 rounded-md border p-3">
                <Checkbox
                  className="mt-0.5"
                  checked={manualStopConfirmed}
                  disabled={serverStillOnline || applyMutation.isPending}
                  onCheckedChange={(checked) =>
                    setManualStopConfirmed(checked === true)
                  }
                />
                <span>
                  <span className="block text-xs font-medium">
                    {t("mods.manualStopConfirm")}
                  </span>
                  <span className="mt-0.5 block text-[11px] leading-4 text-muted-foreground">
                    {t("mods.manualStopHint")}
                  </span>
                </span>
              </label>
            )}

            <Button
              className="w-full"
              disabled={!canOpenConfirmation}
              onClick={() => {
                setRiskConfirmed(false);
                setConfirmationOpen(true);
              }}
            >
              <Package />
              {applyMutation.isPending
                ? t("mods.applying")
                : t("mods.applyAction")}
            </Button>
            {!plan?.can_apply ? (
              <p className="text-center text-[11px] text-muted-foreground">
                {t("mods.preflightFirst")}
              </p>
            ) : !serverReady ? (
              <p className="text-center text-[11px] text-muted-foreground">
                {manuallyManaged
                  ? serverStillOnline
                    ? t("mods.stopManuallyFirst")
                    : t("mods.confirmManualStopFirst")
                  : t("mods.serverBusy")}
              </p>
            ) : null}
          </div>
        </Panel>
      </div>

      {lastResult ? (
        <Panel
          title={t("mods.lastResult")}
          description={
            lastResult.apply.applied_at
              ? formatDateTime(lastResult.apply.applied_at)
              : undefined
          }
          contentClassName="p-4 sm:p-5"
        >
          <div className="flex flex-wrap items-center gap-2">
            <Badge className="border-emerald-500/25 bg-emerald-500/12 text-emerald-700 dark:text-emerald-300">
              <CircleCheck /> {t("mods.settingsApplied")}
            </Badge>
            {lastResult.restarted ? (
              <Badge variant="outline">{t("mods.serverRestarted")}</Badge>
            ) : null}
            {lastResult.apply.created ? (
              <Badge variant="secondary">{t("mods.settingsCreated")}</Badge>
            ) : null}
          </div>
          <div className="mt-4 grid gap-3 sm:grid-cols-2">
            <ResultLine
              label={t("mods.recoveryPath")}
              value={lastResult.apply.recovery_path || "--"}
            />
            <ResultLine
              label={t("mods.safetyBackup")}
              value={lastResult.safety_backup?.path || t("mods.notRequired")}
            />
          </div>
        </Panel>
      ) : null}

      <AlertDialog
        open={confirmationOpen}
        onOpenChange={(open) =>
          !applyMutation.isPending && setConfirmationOpen(open)
        }
      >
        <AlertDialogContent className="sm:max-w-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("mods.confirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("mods.confirmDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <div className="grid gap-3 rounded-md bg-muted/35 p-4 text-xs">
            <ConfirmLine
              label={t("mods.selectedPackages")}
              value={String(plan?.desired_settings.active_mod_list.length ?? 0)}
            />
            <ConfirmLine
              label={t("mods.workshopRoot")}
              value={plan?.target_inventory.workshop_root || "--"}
              mono
            />
            <ConfirmLine
              label={t("mods.safetyBackup")}
              value={
                plan?.safety_backup_required
                  ? t("mods.mandatory")
                  : t("mods.notRequired")
              }
            />
            <ConfirmLine
              label={t("mods.restartAfter")}
              value={restartAfter ? t("common.yes") : t("common.no")}
            />
          </div>

          <div className="flex gap-3 rounded-md border border-amber-500/30 bg-amber-500/8 p-3 text-xs leading-5">
            <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-600" />
            <span>{t("mods.confirmWarning")}</span>
          </div>

          <label className="flex items-start gap-3 rounded-md border p-4">
            <Checkbox
              className="mt-0.5"
              checked={riskConfirmed}
              disabled={applyMutation.isPending}
              onCheckedChange={(checked) => setRiskConfirmed(checked === true)}
            />
            <span>
              <span className="block text-xs font-semibold">
                {t("mods.riskConfirm")}
              </span>
              <span className="mt-1 block text-[11px] leading-5 text-muted-foreground">
                {t("mods.riskConfirmHint")}
              </span>
            </span>
          </label>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={applyMutation.isPending}>
              {t("action.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={
                !riskConfirmed ||
                !canOpenConfirmation ||
                applyMutation.isPending
              }
              onClick={(event) => {
                event.preventDefault();
                applyMutation.mutate();
              }}
            >
              {applyMutation.isPending ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <PackageCheck />
              )}
              {applyMutation.isPending
                ? t("mods.applying")
                : t("mods.confirmRun")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function SafetyLine({
  label,
  value,
  danger = false,
}: {
  label: string;
  value: string;
  danger?: boolean;
}) {
  return (
    <div className="border-b pb-3 last:border-b-0 last:pb-0">
      <div className="text-[9px] tracking-[0.12em] text-muted-foreground uppercase">
        {label}
      </div>
      <div
        className={cn(
          "mt-1 leading-5",
          danger ? "text-destructive" : "font-medium",
        )}
      >
        {value}
      </div>
    </div>
  );
}

function ResultLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border bg-muted/20 p-3">
      <div className="text-[9px] tracking-[0.12em] text-muted-foreground uppercase">
        {label}
      </div>
      <div className="font-data mt-1 break-all text-[11px]">{value}</div>
    </div>
  );
}

function ConfirmLine({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <span
        className={cn(
          "max-w-[70%] break-all text-right font-medium",
          mono && "font-data text-[11px]",
        )}
      >
        {value}
      </span>
    </div>
  );
}
