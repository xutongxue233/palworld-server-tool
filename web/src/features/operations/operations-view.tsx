import { useState } from "react";
import {
  CalendarClock,
  DatabaseBackup,
  Download,
  FolderInput,
  LockKeyhole,
  Puzzle,
  RadioTower,
  ShieldCheck,
} from "lucide-react";
import { useOutletContext } from "react-router-dom";

import { SectionHeader } from "@/components/common/section-header";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAuth } from "@/lib/auth";
import { useI18n } from "@/lib/i18n";

import { BackupsPanel } from "@/features/operations/backups-panel";
import { AutomationPanel } from "@/features/operations/automation-panel";
import { ServerControls } from "@/features/operations/server-controls";
import { SaveMigrationPanel } from "@/features/operations/save-migration-panel";
import { SteamCMDPanel } from "@/features/operations/steamcmd-panel";
import { OfficialModsPanel } from "@/features/operations/official-mods-panel";
import { WhitelistPanel } from "@/features/operations/whitelist-panel";

export default function OperationsView() {
  const { t } = useI18n();
  const { isAuthenticated } = useAuth();
  const { openLogin } = useOutletContext<{ openLogin: () => void }>();
  const [activeTab, setActiveTab] = useState("controls");

  return (
    <div>
      <SectionHeader
        eyebrow="CONTROL / AUTHORIZED"
        title={t("operations.title")}
        description={t("operations.subtitle")}
      />

      {!isAuthenticated ? (
        <div className="flex min-h-[520px] items-center justify-center px-4 py-10">
          <div className="max-w-md text-center">
            <div className="telemetry-grid mx-auto flex size-14 items-center justify-center rounded-md border bg-muted text-primary">
              <LockKeyhole className="size-6" />
            </div>
            <h2 className="mt-5 text-xl font-semibold">{t("auth.login")}</h2>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">
              {t("auth.description")}
            </p>
            <Button className="mt-5" onClick={openLogin}>
              <LockKeyhole /> {t("auth.login")}
            </Button>
          </div>
        </div>
      ) : (
        <Tabs
          value={activeTab}
          onValueChange={setActiveTab}
          className="min-w-0"
        >
          <div className="border-b px-4 sm:px-6 lg:px-8">
            <TabsList
              variant="line"
              className="scrollbar-thin h-14 max-w-full justify-start overflow-x-auto"
            >
              <TabsTrigger value="controls">
                <RadioTower /> {t("operations.controls")}
              </TabsTrigger>
              <TabsTrigger value="whitelist">
                <ShieldCheck /> {t("operations.whitelist")}
              </TabsTrigger>
              <TabsTrigger value="automation">
                <CalendarClock /> {t("operations.automation")}
              </TabsTrigger>
              <TabsTrigger value="deployment">
                <Download /> {t("operations.deployment")}
              </TabsTrigger>
              <TabsTrigger value="mods">
                <Puzzle /> {t("operations.mods")}
              </TabsTrigger>
              <TabsTrigger value="migration">
                <FolderInput /> {t("operations.migration")}
              </TabsTrigger>
              <TabsTrigger value="backups">
                <DatabaseBackup /> {t("operations.backups")}
              </TabsTrigger>
            </TabsList>
          </div>
          <div className="p-4 sm:p-6 lg:p-8">
            <TabsContent value="controls" className="m-0">
              <ServerControls />
            </TabsContent>
            <TabsContent value="whitelist" className="m-0">
              <WhitelistPanel />
            </TabsContent>
            <TabsContent value="automation" className="m-0">
              <AutomationPanel />
            </TabsContent>
            <TabsContent value="deployment" className="m-0">
              <SteamCMDPanel />
            </TabsContent>
            <TabsContent value="mods" className="m-0">
              <OfficialModsPanel />
            </TabsContent>
            <TabsContent value="migration" className="m-0">
              <SaveMigrationPanel />
            </TabsContent>
            <TabsContent value="backups" className="m-0">
              <BackupsPanel />
            </TabsContent>
          </div>
        </Tabs>
      )}
    </div>
  );
}
