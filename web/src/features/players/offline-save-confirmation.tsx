import { PowerOff } from "lucide-react";

import { Checkbox } from "@/components/ui/checkbox";
import { useI18n } from "@/lib/i18n";

export function OfflineSaveConfirmation({
  checked,
  disabled = false,
  onCheckedChange,
}: {
  checked: boolean;
  disabled?: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  const { t } = useI18n();

  return (
    <div className="border-l-2 border-[var(--warning)] pl-3">
      <div className="flex items-center gap-2 text-sm font-semibold">
        <PowerOff className="size-4 text-[var(--warning)]" />
        {t("delivery.offlineTitle")}
      </div>
      <p className="mt-1 text-xs leading-5 text-muted-foreground">
        {t("delivery.offlineWarning")}
      </p>
      <label className="mt-3 flex cursor-pointer items-start gap-2.5 text-sm">
        <Checkbox
          checked={checked}
          disabled={disabled}
          onCheckedChange={(value) => onCheckedChange(value === true)}
          className="mt-0.5"
        />
        <span>{t("delivery.confirmStopped")}</span>
      </label>
    </div>
  );
}
