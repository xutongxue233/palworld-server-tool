import { useMemo, useRef, useState, type ChangeEvent } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  ClipboardPaste,
  Copy,
  Download,
  ExternalLink,
  FileCog,
  FileInput,
  FileText,
  HardDriveDownload,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
  Search,
  ServerCog,
  ShieldCheck,
  SlidersHorizontal,
  Upload,
} from "lucide-react";
import { useOutletContext } from "react-router-dom";
import { toast } from "sonner";

import { SectionHeader } from "@/components/common/section-header";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import {
  getDefaultSettings,
  getLocalizedText,
  SETTING_DEFINITIONS,
  SETTING_GROUPS,
  type SettingDefinition,
  type SettingGroup,
  type SettingValue,
} from "@/features/configuration/server-settings";
import {
  normalizeApiSettings,
  parsePalWorldSettings,
  serializePalWorldSettings,
  settingValuesEqual,
  validateServerSettings,
  type SettingIssue,
} from "@/features/configuration/server-settings-model";
import { api, getApiErrorMessage } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { copyText, downloadBlob } from "@/lib/format";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { Locale, WorldOptionOverrideStatus } from "@/types/api";

type ConfigSource = "defaults" | "file" | "paste" | "server" | "server-file";
type GroupFilter = "all" | SettingGroup;

function ListTextInput({
  value,
  onChange,
  placeholder,
}: {
  value: string[];
  onChange: (value: string[]) => void;
  placeholder: string;
}) {
  const [draft, setDraft] = useState(value.join(", "));
  const commit = () => {
    onChange(
      draft
        .split(/[,\n]/)
        .map((item) => item.trim())
        .filter(Boolean),
    );
  };

  return (
    <Textarea
      value={draft}
      onChange={(event) => setDraft(event.target.value)}
      onBlur={commit}
      placeholder={placeholder}
      className="min-h-20 resize-y font-data text-xs"
    />
  );
}

function FieldControl({
  definition,
  value,
  onChange,
  locale,
}: {
  definition: SettingDefinition;
  value: SettingValue;
  onChange: (value: SettingValue) => void;
  locale: Locale;
}) {
  if (definition.type === "boolean") {
    return (
      <div className="flex h-9 items-center justify-end gap-3">
        <span className="font-data text-xs text-muted-foreground">
          {value ? "True" : "False"}
        </span>
        <Switch checked={Boolean(value)} onCheckedChange={onChange} />
      </div>
    );
  }

  if (definition.type === "select") {
    return (
      <Select value={String(value)} onValueChange={onChange}>
        <SelectTrigger className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {definition.options?.map((option) => (
            <SelectItem key={option} value={option}>
              {option}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  }

  if (definition.type === "list" && definition.options?.length) {
    const selected = Array.isArray(value) ? value : [];
    return (
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {definition.options.map((option) => {
          const checked = selected.includes(option);
          return (
            <label
              key={option}
              className={cn(
                "flex h-9 cursor-pointer items-center gap-2 rounded-md border px-3 text-xs transition-colors",
                checked && "border-primary/50 bg-primary/8 text-foreground",
              )}
            >
              <Checkbox
                checked={checked}
                onCheckedChange={(nextChecked) =>
                  onChange(
                    nextChecked
                      ? [...selected, option]
                      : selected.filter((item) => item !== option),
                  )
                }
              />
              {option}
            </label>
          );
        })}
      </div>
    );
  }

  if (definition.type === "list") {
    const listValue = Array.isArray(value) ? value : [];
    return (
      <ListTextInput
        key={JSON.stringify(listValue)}
        value={listValue}
        onChange={onChange}
        placeholder="PALBOX, RepairBench"
      />
    );
  }

  if (definition.type === "integer" || definition.type === "float") {
    return (
      <div className="relative">
        <Input
          type="number"
          value={String(value)}
          min={definition.min}
          max={definition.max}
          step={definition.step ?? (definition.type === "integer" ? 1 : 0.1)}
          onChange={(event) =>
            onChange(
              event.target.value === "" ? "" : Number(event.target.value),
            )
          }
          className={cn(definition.unit && "pr-20")}
        />
        {definition.unit ? (
          <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-[11px] text-muted-foreground">
            {getLocalizedText(definition.unit, locale)}
          </span>
        ) : null}
      </div>
    );
  }

  return (
    <Input
      type={definition.type === "password" ? "password" : "text"}
      value={String(value)}
      onChange={(event) => onChange(event.target.value)}
      autoComplete="off"
    />
  );
}

function MetricCell({
  label,
  value,
  detail,
  tone = "default",
}: {
  label: string;
  value: string | number;
  detail?: string;
  tone?: "default" | "warning" | "danger";
}) {
  return (
    <div className="min-w-0 border-r px-4 py-3 last:border-r-0 sm:px-5">
      <p className="text-[11px] text-muted-foreground">{label}</p>
      <div className="mt-1 flex min-w-0 items-baseline gap-2">
        <span
          className={cn(
            "font-data text-lg font-semibold",
            tone === "warning" && "text-amber-600 dark:text-amber-400",
            tone === "danger" && "text-destructive",
          )}
        >
          {value}
        </span>
        {detail ? (
          <span className="truncate text-[11px] text-muted-foreground">
            {detail}
          </span>
        ) : null}
      </div>
    </div>
  );
}

function PreviewPanel({
  serialized,
  issues,
  title,
  description,
  copyLabel,
  validLabel,
}: {
  serialized: string;
  issues: SettingIssue[];
  title: string;
  description: string;
  copyLabel: string;
  validLabel: string;
}) {
  const { t } = useI18n();

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-start justify-between gap-3 border-b px-4 py-4">
        <div>
          <h2 className="text-sm font-semibold">{title}</h2>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">
            {description}
          </p>
        </div>
        <Button
          size="icon-sm"
          variant="outline"
          onClick={() => {
            void copyText(serialized);
            toast.success(copyLabel);
          }}
        >
          <Copy />
          <span className="sr-only">{copyLabel}</span>
        </Button>
      </div>
      {issues.length ? (
        <div className="max-h-36 space-y-2 overflow-y-auto border-b px-4 py-3">
          {issues.slice(0, 6).map((issue, index) => (
            <div
              key={`${issue.key}-${index}`}
              className="flex items-start gap-2 text-xs"
            >
              <AlertTriangle
                className={cn(
                  "mt-0.5 size-3.5 shrink-0",
                  issue.level === "error"
                    ? "text-destructive"
                    : "text-amber-600 dark:text-amber-400",
                )}
              />
              <span>
                <span className="font-data">{issue.key}</span>:{" "}
                {t(issue.messageKey, issue.variables)}
              </span>
            </div>
          ))}
        </div>
      ) : (
        <div className="flex items-center gap-2 border-b px-4 py-3 text-xs text-emerald-700 dark:text-emerald-400">
          <CheckCircle2 className="size-4" />
          {validLabel}
        </div>
      )}
      <Textarea
        readOnly
        value={serialized}
        wrap="off"
        className="min-h-0 flex-1 resize-none rounded-none border-0 bg-muted/35 p-4 font-data text-[11px] leading-5 shadow-none focus-visible:ring-0"
      />
    </div>
  );
}

export default function ConfigurationView() {
  const { locale, t } = useI18n();
  const { isAuthenticated } = useAuth();
  const { openLogin } = useOutletContext<{ openLogin: () => void }>();
  const defaults = useMemo(() => getDefaultSettings(), []);
  const [values, setValues] = useState<Record<string, SettingValue>>(defaults);
  const [unknown, setUnknown] = useState<Record<string, string>>({});
  const [source, setSource] = useState<ConfigSource>("defaults");
  const [group, setGroup] = useState<GroupFilter>("all");
  const [search, setSearch] = useState("");
  const [changedOnly, setChangedOnly] = useState(false);
  const [pasteOpen, setPasteOpen] = useState(false);
  const [pasteContent, setPasteContent] = useState("");
  const [previewOpen, setPreviewOpen] = useState(false);
  const [loadingServer, setLoadingServer] = useState(false);
  const [loadingServerFile, setLoadingServerFile] = useState(false);
  const [writingServerFile, setWritingServerFile] = useState(false);
  const [syncingWorldOption, setSyncingWorldOption] = useState(false);
  const [worldOptionDialogOpen, setWorldOptionDialogOpen] = useState(false);
  const [serverFile, setServerFile] = useState<{
    path: string;
    sha256: string;
  } | null>(null);
  const [serverFileBaseline, setServerFileBaseline] = useState("");
  const [restartRequired, setRestartRequired] = useState(false);
  const [worldOption, setWorldOption] =
    useState<WorldOptionOverrideStatus | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const issues = useMemo(() => validateServerSettings(values), [values]);
  const errors = issues.filter((issue) => issue.level === "error");
  const warnings = issues.filter((issue) => issue.level === "warning");
  const serialized = useMemo(
    () => serializePalWorldSettings(values, unknown),
    [unknown, values],
  );
  const changedKeys = useMemo(
    () =>
      new Set(
        SETTING_DEFINITIONS.filter(
          (definition) =>
            !settingValuesEqual(
              values[definition.key] ?? definition.defaultValue,
              definition.defaultValue,
            ),
        ).map((definition) => definition.key),
      ),
    [values],
  );
  const serverFileDirty = Boolean(
    serverFile && serverFileBaseline && serialized !== serverFileBaseline,
  );

  const visibleDefinitions = useMemo(() => {
    const query = search.trim().toLowerCase();
    return SETTING_DEFINITIONS.filter((definition) => {
      if (group !== "all" && definition.group !== group) return false;
      if (changedOnly && !changedKeys.has(definition.key)) return false;
      if (!query) return true;
      return [
        definition.key,
        definition.label.zh,
        definition.label.en,
        definition.description.zh,
        definition.description.en,
      ].some((value) => value.toLowerCase().includes(query));
    });
  }, [changedKeys, changedOnly, group, search]);

  const updateValue = (key: string, value: SettingValue) => {
    setValues((current) => ({ ...current, [key]: value }));
  };

  const resetAll = () => {
    setValues(getDefaultSettings());
    setUnknown({});
    setSource("defaults");
    setServerFile(null);
    setServerFileBaseline("");
    setRestartRequired(false);
    setWorldOption(null);
    toast.success(t("config.resetDone"));
  };

  const applyImportedContent = (
    content: string,
    nextSource: ConfigSource,
    notify = true,
  ) => {
    try {
      const parsed = parsePalWorldSettings(content);
      const nextValues = { ...getDefaultSettings(), ...parsed.values };
      setValues(nextValues);
      setUnknown(parsed.unknown);
      setSource(nextSource);
      if (notify) {
        toast.success(t("config.imported"), {
          description: t("config.importedCount", {
            count: parsed.loadedKeys.length,
          }),
        });
      }
      return serializePalWorldSettings(nextValues, parsed.unknown);
    } catch (error) {
      toast.error(t("config.invalid"), {
        description: getApiErrorMessage(error),
      });
      return null;
    }
  };

  const handleFileInput = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    setServerFile(null);
    setServerFileBaseline("");
    setRestartRequired(false);
    setWorldOption(null);
    applyImportedContent(await file.text(), "file");
  };

  const loadServerSettings = async () => {
    if (!isAuthenticated) {
      openLogin();
      return;
    }
    setLoadingServer(true);
    try {
      const result = normalizeApiSettings(await api.getSettings());
      setValues({ ...getDefaultSettings(), ...result.values });
      setUnknown(result.unknown);
      setSource("server");
      setServerFile(null);
      setServerFileBaseline("");
      setRestartRequired(false);
      setWorldOption(null);
      toast.success(t("config.serverLoaded"));
    } catch (error) {
      toast.error(t("message.error"), {
        description: getApiErrorMessage(error),
      });
    } finally {
      setLoadingServer(false);
    }
  };

  const loadServerFile = async () => {
    if (!isAuthenticated) {
      openLogin();
      return;
    }
    setLoadingServerFile(true);
    try {
      const result = await api.getGameConfigFile();
      setWorldOption(result.world_option);
      if (!result.configured) {
        toast.error(t("config.serverFileNotConfigured"));
        return;
      }
      if (!result.content || !result.sha256 || !result.path) {
        throw new Error(t("config.serverFileIncomplete"));
      }
      const normalized = applyImportedContent(
        result.content,
        "server-file",
        false,
      );
      if (normalized) {
        setServerFile({ path: result.path, sha256: result.sha256 });
        setServerFileBaseline(normalized);
        setRestartRequired(false);
        toast.success(t("config.serverFileLoaded"), {
          description: result.path,
        });
        if (result.world_option.present) {
          toast.warning(t("config.worldOptionOverrideTitle"), {
            description: t("config.worldOptionOverrideToast"),
          });
        }
      }
    } catch (error) {
      toast.error(t("message.error"), {
        description: getApiErrorMessage(error),
      });
    } finally {
      setLoadingServerFile(false);
    }
  };

  const writeServerFile = async () => {
    if (!serverFile || errors.length) return;
    setWritingServerFile(true);
    try {
      const result = await api.putGameConfigFile(serialized, serverFile.sha256);
      setServerFile((current) =>
        current ? { ...current, sha256: result.sha256 } : current,
      );
      setServerFileBaseline(serialized);
      setRestartRequired(result.restart_required);
      toast.success(t("config.serverFileWritten"), {
        description: t("config.serverFileBackup", {
          path: result.backup_path,
        }),
      });
    } catch (error) {
      toast.error(t("config.serverFileWriteFailed"), {
        description: getApiErrorMessage(error),
      });
    } finally {
      setWritingServerFile(false);
    }
  };

  const synchronizeWorldOption = async () => {
    if (
      !serverFile ||
      serverFileDirty ||
      errors.length ||
      !worldOption?.supported ||
      worldOption.message
    ) {
      return;
    }
    setSyncingWorldOption(true);
    try {
      const result = await api.syncWorldOption(
        serialized,
        worldOption.sha256 ?? "",
      );
      toast.success(
        result.world_option.created
          ? t("config.worldOptionGenerated")
          : t("config.worldOptionSynchronized"),
        {
          description: t("config.worldOptionSafetyBackup", {
            path: result.safety_backup.path,
          }),
        },
      );
      if (result.world_option.skipped_keys.length) {
        toast.warning(t("config.worldOptionSkipped"), {
          description: result.world_option.skipped_keys.join(", "),
        });
      }
      if (result.restart_error) {
        toast.warning(t("config.worldOptionRestartFailed"), {
          description: result.restart_error,
        });
      }
      setRestartRequired(!result.restarted);
      const refreshed = await api.getGameConfigFile();
      setWorldOption(refreshed.world_option);
      setWorldOptionDialogOpen(false);
    } catch (error) {
      toast.error(t("config.worldOptionSyncFailed"), {
        description: getApiErrorMessage(error),
      });
    } finally {
      setSyncingWorldOption(false);
    }
  };

  const downloadConfiguration = () => {
    if (errors.length) return;
    downloadBlob(
      new Blob([serialized], { type: "text/plain;charset=utf-8" }),
      "PalWorldSettings.ini",
    );
  };

  const sourceLabel = t(`config.source.${source}`);
  const preview = (
    <PreviewPanel
      serialized={serialized}
      issues={issues}
      title={t("config.preview")}
      description={t("config.previewDescription")}
      copyLabel={t("message.copied")}
      validLabel={t("config.valid")}
    />
  );

  return (
    <div>
      <SectionHeader
        eyebrow="CONFIG / PAL-CONF 1.0.0"
        title={t("config.title")}
        description={t("config.subtitle")}
        actions={
          <>
            <Button variant="outline" size="sm" asChild>
              <a
                href="https://docs.palworldgame.com/settings-and-operation/configuration"
                target="_blank"
                rel="noreferrer"
              >
                <ExternalLink />
                {t("config.officialDocs")}
              </a>
            </Button>
            <Button
              size="sm"
              onClick={downloadConfiguration}
              disabled={errors.length > 0}
            >
              <Download />
              {t("config.download")}
            </Button>
          </>
        }
      />

      <div className="grid grid-cols-2 border-b sm:grid-cols-4">
        <MetricCell label={t("config.source")} value={sourceLabel} />
        <MetricCell
          label={t("config.changed")}
          value={changedKeys.size}
          detail={`/ ${SETTING_DEFINITIONS.length}`}
        />
        <MetricCell
          label={t("config.validation")}
          value={errors.length + warnings.length}
          detail={`${errors.length} ${t("config.errors")}`}
          tone={
            errors.length ? "danger" : warnings.length ? "warning" : "default"
          }
        />
        <MetricCell
          label={t("config.preserved")}
          value={Object.keys(unknown).length}
          detail={t("config.unknownFields")}
        />
      </div>

      <div className="grid border-b bg-muted/20 sm:grid-cols-3">
        {[
          {
            label: t("config.stateRead"),
            active: Boolean(serverFile),
            detail: serverFile?.path || t("config.stateReadHint"),
          },
          {
            label: t("config.stateWritten"),
            active: restartRequired && !serverFileDirty,
            detail: serverFileDirty
              ? t("config.stateUnsaved")
              : t("config.stateWrittenHint"),
          },
          {
            label: t("config.stateRestart"),
            active: restartRequired,
            detail: restartRequired
              ? t("config.stateRestartRequired")
              : t("config.stateRestartHint"),
          },
        ].map((item, index) => (
          <div
            key={item.label}
            className="flex min-w-0 items-start gap-3 border-b px-4 py-3 last:border-b-0 sm:border-b-0 sm:border-r sm:last:border-r-0 sm:px-6"
          >
            <span
              className={cn(
                "font-data flex size-6 shrink-0 items-center justify-center rounded-full border text-[10px]",
                item.active
                  ? "border-emerald-500/50 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"
                  : "text-muted-foreground",
              )}
            >
              {index + 1}
            </span>
            <span className="min-w-0">
              <span className="block text-xs font-medium">{item.label}</span>
              <span className="mt-0.5 block truncate text-[11px] text-muted-foreground">
                {item.detail}
              </span>
            </span>
          </div>
        ))}
      </div>

      {serverFile && worldOption?.present ? (
        <div className="flex flex-col gap-3 border-b border-amber-500/30 bg-amber-500/8 px-4 py-4 sm:flex-row sm:items-start sm:px-6 lg:px-8">
          <AlertTriangle className="mt-0.5 size-5 shrink-0 text-amber-600" />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-sm font-semibold">
                {t("config.worldOptionOverrideTitle")}
              </p>
              <Badge
                variant="outline"
                className="border-amber-500/40 text-amber-700 dark:text-amber-400"
              >
                WorldOption.sav
              </Badge>
            </div>
            <p className="mt-1 max-w-4xl text-xs leading-5 text-muted-foreground">
              {t("config.worldOptionOverrideDescription")}
            </p>
            {worldOption.path ? (
              <p className="mt-2 truncate font-data text-[10px] text-muted-foreground">
                {worldOption.path}
              </p>
            ) : null}
            {worldOption.message ? (
              <p className="mt-2 text-xs text-destructive">
                {worldOption.message}
              </p>
            ) : null}
          </div>
          <div className="flex shrink-0 flex-wrap gap-2">
            <Button
              size="sm"
              disabled={
                serverFileDirty ||
                errors.length > 0 ||
                Boolean(worldOption.message)
              }
              onClick={() => setWorldOptionDialogOpen(true)}
            >
              <FileCog />
              {t("config.syncWorldOption")}
            </Button>
            <Button variant="outline" size="sm" asChild>
              <a
                href="https://pal-conf.bluefissure.com/"
                target="_blank"
                rel="noreferrer"
              >
                <ExternalLink />
                {t("config.openWorldOptionTool")}
              </a>
            </Button>
          </div>
        </div>
      ) : null}

      {serverFile && worldOption?.supported && !worldOption.present ? (
        <div className="flex flex-col gap-3 border-b bg-muted/20 px-4 py-4 sm:flex-row sm:items-center sm:px-6 lg:px-8">
          <FileCog className="size-5 shrink-0 text-muted-foreground" />
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium">
              {t("config.worldOptionAbsentTitle")}
            </p>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              {t("config.worldOptionAbsentDescription")}
            </p>
          </div>
          <Button
            variant="outline"
            size="sm"
            disabled={serverFileDirty || errors.length > 0}
            onClick={() => setWorldOptionDialogOpen(true)}
          >
            <FileCog />
            {t("config.generateWorldOption")}
          </Button>
        </div>
      ) : null}

      <div className="flex flex-wrap items-center gap-2 border-b px-4 py-3 sm:px-6 lg:px-8">
        <input
          ref={fileInputRef}
          type="file"
          accept=".ini,text/plain"
          className="hidden"
          onChange={(event) => void handleFileInput(event)}
        />
        <Button
          variant="outline"
          size="sm"
          onClick={() => fileInputRef.current?.click()}
        >
          <FileInput />
          {t("config.importFile")}
        </Button>
        <Button variant="outline" size="sm" onClick={() => setPasteOpen(true)}>
          <ClipboardPaste />
          {t("config.paste")}
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void loadServerSettings()}
          disabled={loadingServer}
        >
          <RefreshCw className={cn(loadingServer && "animate-spin")} />
          {t("config.loadServer")}
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void loadServerFile()}
          disabled={loadingServerFile}
        >
          <HardDriveDownload
            className={cn(loadingServerFile && "animate-pulse")}
          />
          {t("config.loadServerFile")}
        </Button>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              size="sm"
              disabled={
                !serverFile ||
                !serverFileDirty ||
                errors.length > 0 ||
                writingServerFile
              }
            >
              {writingServerFile ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <Upload />
              )}
              {t("config.writeServerFile")}
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t("config.writeServerFile")}</AlertDialogTitle>
              <AlertDialogDescription>
                {t("config.writeServerFileConfirm")}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("action.cancel")}</AlertDialogCancel>
              <AlertDialogAction onClick={() => void writeServerFile()}>
                {t("action.confirm")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
        <Button
          variant="outline"
          size="sm"
          className="xl:hidden"
          onClick={() => setPreviewOpen(true)}
        >
          <FileText />
          {t("config.preview")}
        </Button>
        <div className="ml-auto">
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="ghost" size="sm">
                <RotateCcw />
                {t("config.reset")}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{t("config.reset")}</AlertDialogTitle>
                <AlertDialogDescription>
                  {t("config.resetConfirm")}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{t("action.cancel")}</AlertDialogCancel>
                <AlertDialogAction onClick={resetAll}>
                  {t("action.confirm")}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>

      <div className="border-b px-4 py-3 sm:px-6 lg:px-8">
        <div className="flex gap-1 overflow-x-auto pb-1">
          <Button
            size="sm"
            variant={group === "all" ? "secondary" : "ghost"}
            onClick={() => setGroup("all")}
          >
            <SlidersHorizontal />
            {t("config.allGroups")}
          </Button>
          {SETTING_GROUPS.map((item) => (
            <Button
              key={item.id}
              size="sm"
              variant={group === item.id ? "secondary" : "ghost"}
              onClick={() => setGroup(item.id)}
            >
              {getLocalizedText(item.label, locale)}
            </Button>
          ))}
        </div>
        <div className="mt-3 flex flex-col gap-3 sm:flex-row sm:items-center">
          <div className="relative min-w-0 flex-1">
            <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("config.search")}
              className="pl-9"
            />
          </div>
          <label className="flex h-9 shrink-0 items-center gap-3 text-sm">
            <Switch checked={changedOnly} onCheckedChange={setChangedOnly} />
            {t("config.changedOnly")}
          </label>
          <Badge variant="outline" className="justify-center">
            {visibleDefinitions.length} {t("config.fields")}
          </Badge>
        </div>
      </div>

      <div className="grid min-w-0 xl:grid-cols-[minmax(0,1fr)_430px]">
        <section className="min-w-0 xl:border-r">
          {visibleDefinitions.length ? (
            visibleDefinitions.map((definition) => {
              const value = values[definition.key] ?? definition.defaultValue;
              const changed = changedKeys.has(definition.key);
              const fieldIssues = issues.filter(
                (issue) => issue.key === definition.key,
              );
              return (
                <div
                  key={definition.key}
                  className="grid gap-4 border-b px-4 py-4 sm:px-6 lg:grid-cols-[minmax(0,1fr)_minmax(260px,380px)_32px] lg:items-start lg:px-8"
                >
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="text-sm font-semibold">
                        {getLocalizedText(definition.label, locale)}
                      </h2>
                      {changed ? (
                        <Badge variant="secondary">
                          {t("config.modified")}
                        </Badge>
                      ) : null}
                      {definition.official === false ? (
                        <Badge variant="outline">
                          {t("config.compatibility")}
                        </Badge>
                      ) : null}
                    </div>
                    <p className="font-data mt-1 break-all text-[11px] text-muted-foreground">
                      {definition.key}
                    </p>
                    <p className="mt-2 max-w-2xl text-xs leading-5 text-muted-foreground">
                      {getLocalizedText(definition.description, locale)}
                    </p>
                    {fieldIssues.map((issue, index) => (
                      <p
                        key={`${issue.key}-${index}`}
                        className={cn(
                          "mt-2 flex items-start gap-1.5 text-xs",
                          issue.level === "error"
                            ? "text-destructive"
                            : "text-amber-600 dark:text-amber-400",
                        )}
                      >
                        <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
                        {t(issue.messageKey, issue.variables)}
                      </p>
                    ))}
                  </div>
                  <FieldControl
                    definition={definition}
                    value={value}
                    locale={locale}
                    onChange={(nextValue) =>
                      updateValue(definition.key, nextValue)
                    }
                  />
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    disabled={!changed}
                    onClick={() =>
                      updateValue(
                        definition.key,
                        Array.isArray(definition.defaultValue)
                          ? [...definition.defaultValue]
                          : definition.defaultValue,
                      )
                    }
                  >
                    <RotateCcw />
                    <span className="sr-only">{t("config.resetField")}</span>
                  </Button>
                </div>
              );
            })
          ) : (
            <div className="flex min-h-80 flex-col items-center justify-center gap-3 px-6 text-center text-muted-foreground">
              <ServerCog className="size-8" />
              <p className="text-sm">{t("message.empty")}</p>
            </div>
          )}
        </section>

        <aside className="hidden min-h-0 xl:block">
          <div className="sticky top-[112px] h-[calc(100dvh-112px)] min-h-[560px]">
            {preview}
          </div>
        </aside>
      </div>

      <AlertDialog
        open={worldOptionDialogOpen}
        onOpenChange={(open) => {
          if (!syncingWorldOption) setWorldOptionDialogOpen(open);
        }}
      >
        <AlertDialogContent className="sm:max-w-xl">
          <AlertDialogHeader>
            <AlertDialogTitle>
              {worldOption?.present
                ? t("config.syncWorldOptionTitle")
                : t("config.generateWorldOptionTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("config.worldOptionSyncDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <div className="overflow-hidden rounded-md border">
            {[
              ["01", t("config.worldOptionStepStop")],
              ["02", t("config.worldOptionStepBackup")],
              ["03", t("config.worldOptionStepValidate")],
              ["04", t("config.worldOptionStepInstall")],
            ].map(([number, label]) => (
              <div
                key={number}
                className="flex items-center gap-3 border-b px-3 py-2.5 last:border-b-0"
              >
                <span className="font-data flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-[9px] text-primary">
                  {number}
                </span>
                <span className="text-xs font-medium">{label}</span>
              </div>
            ))}
          </div>

          <div className="flex gap-3 rounded-md border border-amber-500/30 bg-amber-500/8 p-3 text-xs leading-5">
            <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-600" />
            <span>{t("config.worldOptionPrecedenceWarning")}</span>
          </div>

          <div className="grid gap-2 rounded-md bg-muted/35 p-3 text-xs">
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">
                {t("config.worldOptionSource")}
              </span>
              <span className="max-w-72 truncate font-data">
                {serverFile?.path}
              </span>
            </div>
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">
                {t("config.worldOptionMode")}
              </span>
              <span className="font-medium">
                {worldOption?.present
                  ? t("config.worldOptionModeSync")
                  : t("config.worldOptionModeGenerate")}
              </span>
            </div>
            <div className="flex items-center gap-2 border-t pt-2 text-muted-foreground">
              <ShieldCheck className="size-3.5 text-emerald-600" />
              {t("config.worldOptionSavedIniRequired")}
            </div>
          </div>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={syncingWorldOption}>
              {t("action.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={syncingWorldOption}
              onClick={(event) => {
                event.preventDefault();
                void synchronizeWorldOption();
              }}
            >
              {syncingWorldOption ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <FileCog />
              )}
              {syncingWorldOption
                ? t("config.worldOptionSyncing")
                : worldOption?.present
                  ? t("config.syncWorldOption")
                  : t("config.generateWorldOption")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={pasteOpen} onOpenChange={setPasteOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("config.pasteTitle")}</DialogTitle>
            <DialogDescription>
              {t("config.pasteDescription")}
            </DialogDescription>
          </DialogHeader>
          <Textarea
            value={pasteContent}
            onChange={(event) => setPasteContent(event.target.value)}
            className="min-h-72 font-data text-xs"
            placeholder="[/Script/Pal.PalGameWorldSettings]"
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setPasteOpen(false)}>
              {t("action.cancel")}
            </Button>
            <Button
              onClick={() => {
                setServerFile(null);
                setServerFileBaseline("");
                setRestartRequired(false);
                setWorldOption(null);
                applyImportedContent(pasteContent, "paste");
                setPasteOpen(false);
              }}
            >
              <ClipboardPaste />
              {t("config.import")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={previewOpen} onOpenChange={setPreviewOpen}>
        <DialogContent className="h-[86dvh] max-w-[calc(100%-2rem)] p-0 sm:max-w-3xl">
          <DialogHeader className="sr-only">
            <DialogTitle>{t("config.preview")}</DialogTitle>
            <DialogDescription>
              {t("config.previewDescription")}
            </DialogDescription>
          </DialogHeader>
          <div className="min-h-0 pt-10">{preview}</div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
