import {
  SETTING_BY_KEY,
  SETTING_DEFINITIONS,
  type SettingDefinition,
  type SettingValue,
} from "@/features/configuration/server-settings";

export interface ParsedServerSettings {
  values: Record<string, SettingValue>;
  unknown: Record<string, string>;
  loadedKeys: string[];
}

export interface SettingIssue {
  key: string;
  level: "error" | "warning";
  messageKey: string;
  variables?: Record<string, string | number>;
}

function splitTopLevel(value: string) {
  const parts: string[] = [];
  let start = 0;
  let depth = 0;
  let quoted = false;
  let escaped = false;

  for (let index = 0; index < value.length; index += 1) {
    const character = value[index];
    if (escaped) {
      escaped = false;
      continue;
    }
    if (character === "\\" && quoted) {
      escaped = true;
      continue;
    }
    if (character === '"') {
      quoted = !quoted;
      continue;
    }
    if (!quoted) {
      if (character === "(") depth += 1;
      if (character === ")") depth -= 1;
      if (character === "," && depth === 0) {
        parts.push(value.slice(start, index).trim());
        start = index + 1;
      }
    }
  }

  const tail = value.slice(start).trim();
  if (tail) parts.push(tail);
  return parts;
}

function extractOptionSettings(content: string) {
  const marker = "OptionSettings";
  const markerIndex = content.indexOf(marker);
  if (markerIndex < 0) throw new Error("OptionSettings was not found");
  const openIndex = content.indexOf("(", markerIndex + marker.length);
  if (openIndex < 0)
    throw new Error("OptionSettings opening bracket was not found");

  let depth = 1;
  let quoted = false;
  let escaped = false;
  for (let index = openIndex + 1; index < content.length; index += 1) {
    const character = content[index];
    if (escaped) {
      escaped = false;
      continue;
    }
    if (character === "\\" && quoted) {
      escaped = true;
      continue;
    }
    if (character === '"') {
      quoted = !quoted;
      continue;
    }
    if (!quoted) {
      if (character === "(") depth += 1;
      if (character === ")") depth -= 1;
      if (depth === 0) return content.slice(openIndex + 1, index);
    }
  }
  throw new Error("OptionSettings closing bracket was not found");
}

function parseQuoted(value: string) {
  const trimmed = value.trim();
  if (!trimmed.startsWith('"') || !trimmed.endsWith('"')) return trimmed;
  try {
    return JSON.parse(trimmed) as string;
  } catch {
    return trimmed.slice(1, -1).replaceAll('""', '"');
  }
}

function parseList(value: string) {
  const trimmed = value.trim().replace(/^\(/, "").replace(/\)$/, "");
  if (!trimmed) return [];
  return splitTopLevel(trimmed)
    .map((item) => parseQuoted(item).trim())
    .filter(Boolean);
}

function parseKnownValue(definition: SettingDefinition, rawValue: string) {
  switch (definition.type) {
    case "boolean":
      if (!["true", "false"].includes(rawValue.trim().toLowerCase())) {
        throw new Error(`${definition.key} is not a valid boolean`);
      }
      return rawValue.trim().toLowerCase() === "true";
    case "integer":
    case "float": {
      const value = Number(rawValue);
      if (!Number.isFinite(value)) {
        throw new Error(`${definition.key} is not a valid number`);
      }
      return value;
    }
    case "list":
      return parseList(rawValue);
    case "text":
    case "password":
      return parseQuoted(rawValue);
    case "select":
      return rawValue.trim();
  }
}

export function parsePalWorldSettings(content: string): ParsedServerSettings {
  const optionSettings = extractOptionSettings(content.trim());
  const values: Record<string, SettingValue> = {};
  const unknown: Record<string, string> = {};
  const loadedKeys: string[] = [];

  splitTopLevel(optionSettings).forEach((entry) => {
    const equalsIndex = entry.indexOf("=");
    if (equalsIndex <= 0) return;
    const key = entry.slice(0, equalsIndex).trim();
    const rawValue = entry.slice(equalsIndex + 1).trim();
    const definition = SETTING_BY_KEY.get(key);
    if (definition) {
      values[key] = parseKnownValue(definition, rawValue);
    } else {
      unknown[key] = rawValue;
    }
    loadedKeys.push(key);
  });

  if (loadedKeys.length === 0) throw new Error("OptionSettings is empty");
  return { values, unknown, loadedKeys };
}

function serializeList(definition: SettingDefinition, value: string[]) {
  const entries = value.map((item) => item.trim()).filter(Boolean);
  if (definition.listStyle === "quoted") {
    return `(${entries.map((item) => JSON.stringify(item)).join(",")})`;
  }
  return `(${entries.join(",")})`;
}

function serializeKnownValue(
  definition: SettingDefinition,
  value: SettingValue,
) {
  switch (definition.type) {
    case "boolean":
      return value ? "True" : "False";
    case "integer":
      return String(Math.trunc(Number(value)));
    case "float":
      return Number(value).toFixed(6);
    case "list":
      return serializeList(definition, Array.isArray(value) ? value : []);
    case "text":
    case "password":
      return JSON.stringify(String(value));
    case "select":
      return String(value);
  }
}

export function serializePalWorldSettings(
  values: Record<string, SettingValue>,
  unknown: Record<string, string> = {},
) {
  const knownEntries = SETTING_DEFINITIONS.map((definition) => {
    const value = values[definition.key] ?? definition.defaultValue;
    return `${definition.key}=${serializeKnownValue(definition, value)}`;
  });
  const unknownEntries = Object.entries(unknown)
    .filter(([key]) => !SETTING_BY_KEY.has(key))
    .map(([key, rawValue]) => `${key}=${rawValue}`);
  return `[/Script/Pal.PalGameWorldSettings]\nOptionSettings=(${[
    ...knownEntries,
    ...unknownEntries,
  ].join(",")})\n`;
}

export function normalizeApiSettings(settings: Record<string, unknown>) {
  const values: Record<string, SettingValue> = {};
  const unknown: Record<string, string> = {};

  Object.entries(settings).forEach(([key, rawValue]) => {
    const definition = SETTING_BY_KEY.get(key);
    if (!definition) {
      unknown[key] =
        typeof rawValue === "string" ? rawValue : JSON.stringify(rawValue);
      return;
    }
    if (definition.type === "boolean") {
      values[key] =
        typeof rawValue === "boolean"
          ? rawValue
          : String(rawValue).toLowerCase() === "true";
    } else if (definition.type === "integer" || definition.type === "float") {
      const numeric = Number(rawValue);
      if (Number.isFinite(numeric)) values[key] = numeric;
    } else if (definition.type === "list") {
      values[key] = Array.isArray(rawValue)
        ? rawValue.map(String)
        : parseList(String(rawValue));
    } else {
      values[key] = String(rawValue ?? "");
    }
  });

  return { values, unknown };
}

export function settingValuesEqual(left: SettingValue, right: SettingValue) {
  if (Array.isArray(left) || Array.isArray(right)) {
    return JSON.stringify(left) === JSON.stringify(right);
  }
  return left === right;
}

export function validateServerSettings(values: Record<string, SettingValue>) {
  const issues: SettingIssue[] = [];

  SETTING_DEFINITIONS.forEach((definition) => {
    const value = values[definition.key] ?? definition.defaultValue;
    if (definition.type === "integer" || definition.type === "float") {
      if (String(value).trim() === "") {
        issues.push({
          key: definition.key,
          level: "error",
          messageKey: "config.validation.required",
        });
        return;
      }
      const numeric = Number(value);
      if (!Number.isFinite(numeric)) {
        issues.push({
          key: definition.key,
          level: "error",
          messageKey: "config.validation.validNumber",
        });
        return;
      }
      if (definition.min !== undefined && numeric < definition.min) {
        issues.push({
          key: definition.key,
          level: "error",
          messageKey: "config.validation.minimum",
          variables: { value: definition.min },
        });
      }
      if (definition.max !== undefined && numeric > definition.max) {
        issues.push({
          key: definition.key,
          level: "error",
          messageKey: "config.validation.maximum",
          variables: { value: definition.max },
        });
      }
    }
  });

  const adminPassword = String(values.AdminPassword ?? "");
  if ((values.RESTAPIEnabled || values.RCONEnabled) && !adminPassword.trim()) {
    issues.push({
      key: "AdminPassword",
      level: "warning",
      messageKey: "config.validation.adminPassword",
    });
  }

  const ports = ["PublicPort", "RCONPort", "RESTAPIPort"] as const;
  ports.forEach((key, index) => {
    ports.slice(index + 1).forEach((otherKey) => {
      if (Number(values[key]) === Number(values[otherKey])) {
        issues.push({
          key: otherKey,
          level: "warning",
          messageKey: "config.validation.duplicatePort",
          variables: { key, otherKey },
        });
      }
    });
  });

  if (Number(values.PalSpawnNumRate) > 2) {
    issues.push({
      key: "PalSpawnNumRate",
      level: "warning",
      messageKey: "config.validation.highSpawn",
    });
  }
  if (Number(values.BaseCampWorkerMaxNum) > 30) {
    issues.push({
      key: "BaseCampWorkerMaxNum",
      level: "warning",
      messageKey: "config.validation.highWorkers",
    });
  }
  if (
    Number(values.VoiceChatZeroVolumeDistance) <
    Number(values.VoiceChatMaxVolumeDistance)
  ) {
    issues.push({
      key: "VoiceChatZeroVolumeDistance",
      level: "error",
      messageKey: "config.validation.voiceDistance",
    });
  }

  return issues;
}
