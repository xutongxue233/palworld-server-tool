import {
  HeartPulse,
  Pencil,
  Shield,
  Sparkles,
  Swords,
  TrendingUp,
  Wrench,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { getPalImage, getPalName, getSkillMetadata } from "@/lib/game-data";
import { useI18n } from "@/lib/i18n";
import type { Pal } from "@/types/api";

function Stat({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Swords;
  label: string;
  value: number | string;
}) {
  return (
    <div className="border-r px-3 py-3 last:border-r-0">
      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Icon className="size-3.5" />
        {label}
      </div>
      <p className="font-data mt-1 text-lg font-semibold">{value}</p>
    </div>
  );
}

export function PalDetailDialog({
  pal,
  onOpenChange,
  onRename,
  onEditLevel,
  onRestoreHealth,
}: {
  pal: Pal | null;
  onOpenChange: (open: boolean) => void;
  onRename?: () => void;
  onEditLevel?: () => void;
  onRestoreHealth?: () => void;
}) {
  const { locale, t } = useI18n();
  if (!pal) return null;
  const name = pal.nickname || getPalName(pal.type, locale);
  const currentHp = Math.round(pal.hp / 1000);
  const maxHp = pal.max_hp > 0 ? Math.round(pal.max_hp / 1000) : "--";

  return (
    <Dialog open={Boolean(pal)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[88dvh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <div className="flex items-start gap-4 pr-8">
            <img
              src={getPalImage(pal.type, pal.is_boss)}
              alt={name}
              className="size-16 shrink-0 rounded-md border bg-muted object-contain p-1"
            />
            <div className="min-w-0">
              <DialogTitle className="truncate text-xl">{name}</DialogTitle>
              <DialogDescription className="font-data mt-1">
                {getPalName(pal.type, locale)} · Lv.{pal.level}
              </DialogDescription>
              <div className="mt-2 flex flex-wrap gap-1.5">
                {pal.is_boss ? <Badge>{t("pal.boss")}</Badge> : null}
                {pal.is_lucky ? (
                  <Badge variant="secondary">{t("pal.lucky")}</Badge>
                ) : null}
                {pal.is_tower ? (
                  <Badge variant="destructive">{t("pal.tower")}</Badge>
                ) : null}
                {pal.gender ? (
                  <Badge variant="outline">{pal.gender}</Badge>
                ) : null}
              </div>
              {onRename || onEditLevel || onRestoreHealth ? (
                <div className="mt-3 flex flex-wrap gap-2">
                  {onRename ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={onRename}
                    >
                      <Pencil />
                      {t("action.renamePal")}
                    </Button>
                  ) : null}
                  {onEditLevel ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={onEditLevel}
                    >
                      <TrendingUp />
                      {t("action.editPalLevel")}
                    </Button>
                  ) : null}
                  {onRestoreHealth &&
                  pal.instance_id &&
                  pal.max_hp > 0 &&
                  pal.hp < pal.max_hp ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={onRestoreHealth}
                    >
                      <HeartPulse />
                      {t("action.restorePalHealth")}
                    </Button>
                  ) : null}
                </div>
              ) : null}
            </div>
          </div>
        </DialogHeader>

        <div className="grid grid-cols-2 overflow-hidden rounded-md border sm:grid-cols-5">
          <Stat
            icon={HeartPulse}
            label={t("pal.hp")}
            value={`${currentHp}/${maxHp}`}
          />
          <Stat icon={Swords} label={t("pal.melee")} value={pal.melee} />
          <Stat icon={Sparkles} label={t("pal.ranged")} value={pal.ranged} />
          <Stat icon={Shield} label={t("pal.defense")} value={pal.defense} />
          <Stat
            icon={Wrench}
            label={t("pal.workspeed")}
            value={pal.workspeed}
          />
        </div>

        <section>
          <h3 className="mb-2 text-sm font-semibold">{t("pal.skills")}</h3>
          {pal.skills?.length ? (
            <div className="divide-y rounded-md border">
              {pal.skills.map((skill) => {
                const metadata = getSkillMetadata(skill, locale);
                return (
                  <div key={skill} className="px-4 py-3">
                    <p className="text-sm font-medium">{metadata.name}</p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {metadata.desc || skill}
                    </p>
                  </div>
                );
              })}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">
              {t("message.empty")}
            </p>
          )}
        </section>
      </DialogContent>
    </Dialog>
  );
}
