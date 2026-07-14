import { useState } from "react";
import {
  ExternalLink,
  FolderSearch,
  Gauge,
  Languages,
  LogIn,
  LogOut,
  Map,
  Menu,
  Moon,
  RefreshCw,
  Shield,
  Sun,
  UsersRound,
  Wrench,
} from "lucide-react";
import { NavLink, Outlet } from "react-router-dom";
import { useTheme } from "next-themes";
import { useQueryClient } from "@tanstack/react-query";

import { LoginDialog } from "@/components/auth/login-dialog";
import { ServerSelector } from "@/components/fleet/server-selector";
import { TelemetryStrip } from "@/components/layout/telemetry-strip";
import { ServerDiscoverySetup } from "@/features/setup/server-discovery-setup";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useServerInfo, useServerMetrics } from "@/hooks/use-server-data";
import { useAuth } from "@/lib/auth";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { Locale } from "@/types/api";
import appIconUrl from "@/assets/app-icon.png";

const navItems = [
  { path: "/", key: "nav.overview", icon: Gauge, end: true },
  { path: "/players", key: "nav.players", icon: UsersRound, end: false },
  { path: "/guilds", key: "nav.guilds", icon: Shield, end: false },
  { path: "/map", key: "nav.map", icon: Map, end: false },
  { path: "/operations", key: "nav.operations", icon: Wrench, end: false },
] as const;

function Navigation({ onNavigate }: { onNavigate?: () => void }) {
  const { t } = useI18n();
  return (
    <nav aria-label="Primary" className="space-y-1 px-3">
      {navItems.map(({ path, key, icon: Icon, end }) => (
        <NavLink
          key={path}
          to={path}
          end={end}
          onClick={onNavigate}
          className={({ isActive }) =>
            cn(
              "flex h-10 items-center gap-3 rounded-md px-3 text-sm font-medium text-sidebar-foreground/68 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring",
              isActive &&
                "bg-sidebar-accent text-sidebar-accent-foreground shadow-[inset_3px_0_0_var(--sidebar-primary)]",
            )
          }
        >
          <Icon className="size-[18px]" />
          <span>{t(key)}</span>
        </NavLink>
      ))}
    </nav>
  );
}

function Brand() {
  const { t } = useI18n();
  return (
    <div className="flex h-[78px] items-center gap-3 border-b border-sidebar-border px-5">
      <img
        src={appIconUrl}
        alt=""
        aria-hidden="true"
        className="size-10 shrink-0 rounded-md border border-sidebar-border object-cover shadow-sm"
      />
      <div className="min-w-0">
        <p className="font-display truncate text-sm font-semibold text-sidebar-foreground">
          PALWORLD
        </p>
        <p className="truncate text-xs text-sidebar-foreground/55">
          {t("app.console")}
        </p>
      </div>
    </div>
  );
}

function HeaderActions({
  onLogin,
  onConfigureServer,
}: {
  onLogin: () => void;
  onConfigureServer: () => void;
}) {
  const queryClient = useQueryClient();
  const { setTheme, resolvedTheme } = useTheme();
  const { locale, setLocale, t } = useI18n();
  const { isAuthenticated, logout } = useAuth();
  const [isRefreshing, setIsRefreshing] = useState(false);

  const refresh = async () => {
    setIsRefreshing(true);
    await queryClient.invalidateQueries();
    window.setTimeout(() => setIsRefreshing(false), 350);
  };

  return (
    <div className="flex items-center gap-1.5">
      <ServerSelector />
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" onClick={refresh}>
            <RefreshCw className={cn(isRefreshing && "animate-spin")} />
            <span className="sr-only">{t("action.refresh")}</span>
          </Button>
        </TooltipTrigger>
        <TooltipContent>{t("action.refresh")}</TooltipContent>
      </Tooltip>
      <DropdownMenu>
        <Tooltip>
          <TooltipTrigger asChild>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon">
                <Languages />
                <span className="sr-only">{t("action.language")}</span>
              </Button>
            </DropdownMenuTrigger>
          </TooltipTrigger>
          <TooltipContent>{t("action.language")}</TooltipContent>
        </Tooltip>
        <DropdownMenuContent align="end">
          {(
            [
              ["zh", "简体中文"],
              ["en", "English"],
              ["ja", "日本語"],
            ] as [Locale, string][]
          ).map(([value, label]) => (
            <DropdownMenuItem
              key={value}
              onClick={() => setLocale(value)}
              className={cn(locale === value && "bg-accent")}
            >
              {label}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            onClick={() =>
              setTheme(resolvedTheme === "dark" ? "light" : "dark")
            }
          >
            {resolvedTheme === "dark" ? <Sun /> : <Moon />}
            <span className="sr-only">{t("action.theme")}</span>
          </Button>
        </TooltipTrigger>
        <TooltipContent>{t("action.theme")}</TooltipContent>
      </Tooltip>
      {isAuthenticated ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className="hidden sm:flex">
              <span className="size-1.5 rounded-full bg-emerald-500" />
              {t("status.authenticated")}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuLabel>{t("status.authenticated")}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={onConfigureServer}>
              <FolderSearch />
              {t("setup.open")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={logout} variant="destructive">
              <LogOut />
              {t("auth.logout")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ) : (
        <Button size="sm" onClick={onLogin}>
          <LogIn />
          <span className="hidden sm:inline">{t("auth.login")}</span>
        </Button>
      )}
    </div>
  );
}

export function AppShell() {
  const { t } = useI18n();
  const { isAuthenticated } = useAuth();
  const [loginOpen, setLoginOpen] = useState(false);
  const [discoveryOpen, setDiscoveryOpen] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const serverQuery = useServerInfo();
  const metricsQuery = useServerMetrics();
  const online = serverQuery.isSuccess && !serverQuery.isError;

  return (
    <div className="min-h-dvh bg-background">
      <aside className="fixed inset-y-0 left-0 z-40 hidden w-[236px] border-r border-sidebar-border bg-sidebar lg:flex lg:flex-col">
        <Brand />
        <div className="flex-1 py-4">
          <Navigation />
        </div>
        <div className="space-y-3 border-t border-sidebar-border p-4">
          <Badge
            variant="outline"
            className="w-full justify-center border-sidebar-border bg-sidebar-accent text-sidebar-foreground"
          >
            {online ? t("status.online") : t("status.offline")}
          </Badge>
          <a
            href="https://github.com/xutongxue233/palworld-server-tool"
            target="_blank"
            rel="noreferrer"
            className="flex items-center justify-center gap-2 text-xs text-sidebar-foreground/55 transition-colors hover:text-sidebar-foreground"
          >
            GitHub <ExternalLink className="size-3" />
          </a>
        </div>
      </aside>

      <div className="lg:pl-[236px]">
        <TelemetryStrip
          server={serverQuery.data}
          metrics={metricsQuery.data}
          online={online}
        />
        <div className="flex h-14 items-center justify-between border-b px-3 sm:px-5 lg:px-6">
          <div className="flex min-w-0 items-center gap-2 lg:hidden">
            <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
              <SheetTrigger asChild>
                <Button variant="ghost" size="icon">
                  <Menu />
                  <span className="sr-only">{t("nav.open")}</span>
                </Button>
              </SheetTrigger>
              <SheetContent
                side="left"
                className="w-[280px] border-sidebar-border bg-sidebar p-0 text-sidebar-foreground"
              >
                <SheetHeader className="sr-only">
                  <SheetTitle>{t("app.name")}</SheetTitle>
                  <SheetDescription>{t("app.console")}</SheetDescription>
                </SheetHeader>
                <Brand />
                <div className="py-4">
                  <Navigation onNavigate={() => setMobileOpen(false)} />
                </div>
              </SheetContent>
            </Sheet>
            <img
              src={appIconUrl}
              alt=""
              aria-hidden="true"
              className="size-7 shrink-0 rounded-md border object-cover"
            />
            <span className="font-display truncate text-sm font-semibold">
              {t("app.name")}
            </span>
          </div>
          <div className="hidden items-center gap-2 text-xs text-muted-foreground lg:flex">
            <span className="size-1.5 rounded-full bg-[var(--signal)]" />
            {metricsQuery.isFetching
              ? t("status.refreshing")
              : t("status.updated")}
          </div>
          <HeaderActions
            onLogin={() => setLoginOpen(true)}
            onConfigureServer={() => setDiscoveryOpen(true)}
          />
        </div>

        <ServerDiscoverySetup
          isAuthenticated={isAuthenticated}
          onLogin={() => setLoginOpen(true)}
          open={discoveryOpen}
          onOpenChange={setDiscoveryOpen}
        />

        <main
          id="main-content"
          className="min-h-[calc(100dvh-7rem)] pb-16 lg:pb-0"
        >
          <Outlet context={{ openLogin: () => setLoginOpen(true) }} />
        </main>
      </div>

      <nav className="fixed inset-x-0 bottom-0 z-40 grid h-16 grid-cols-5 border-t bg-background/96 px-1 backdrop-blur lg:hidden">
        {navItems.map(({ path, key, icon: Icon, end }) => (
          <NavLink
            key={path}
            to={path}
            end={end}
            className={({ isActive }) =>
              cn(
                "flex min-w-0 flex-col items-center justify-center gap-1 text-[10px] font-medium text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring",
                isActive && "text-primary",
              )
            }
          >
            <Icon className="size-5" />
            <span className="max-w-full truncate">{t(key)}</span>
          </NavLink>
        ))}
      </nav>

      <LoginDialog open={loginOpen} onOpenChange={setLoginOpen} />
    </div>
  );
}
