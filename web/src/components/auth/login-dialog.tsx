import { useState } from "react";
import {
  CheckCircle2,
  KeyRound,
  LoaderCircle,
  LogIn,
  ShieldCheck,
} from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/lib/auth";
import { getApiErrorMessage } from "@/lib/api";
import { useI18n } from "@/lib/i18n";

export function LoginDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { initializePassword, login, passwordConfigured } = useAuth();
  const queryClient = useQueryClient();
  const { t } = useI18n();
  const [password, setPassword] = useState("");
  const [passwordConfirmation, setPasswordConfirmation] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const setupRequired = passwordConfigured === false;
  const passwordLongEnough = Array.from(password).length >= 8;
  const passwordsMatch =
    passwordConfirmation.length > 0 && password === passwordConfirmation;
  const canSubmit = setupRequired
    ? passwordLongEnough && passwordsMatch
    : password.length > 0;

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen && setupRequired) return;
    if (!nextOpen) {
      setPassword("");
      setPasswordConfirmation("");
    }
    onOpenChange(nextOpen);
  };

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!canSubmit) return;
    setIsSubmitting(true);
    try {
      if (setupRequired) {
        await initializePassword(password, passwordConfirmation);
      } else {
        await login(password);
      }
      await queryClient.invalidateQueries();
      toast.success(
        t(
          setupRequired
            ? "message.passwordInitialized"
            : "message.loginSuccess",
        ),
      );
      setPassword("");
      setPasswordConfirmation("");
      onOpenChange(false);
    } catch (error) {
      toast.error(
        t(
          setupRequired ? "message.passwordSetupFailed" : "message.loginFailed",
        ),
        {
          description: getApiErrorMessage(error),
        },
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={open || setupRequired} onOpenChange={handleOpenChange}>
      <DialogContent
        className="overflow-hidden p-0 sm:max-w-md"
        showCloseButton={!setupRequired}
        onEscapeKeyDown={(event) => {
          if (setupRequired) event.preventDefault();
        }}
        onPointerDownOutside={(event) => {
          if (setupRequired) event.preventDefault();
        }}
      >
        <form onSubmit={submit}>
          <DialogHeader className="telemetry-grid border-b px-6 py-5">
            <div className="flex items-start gap-3 text-left">
              <div className="flex size-10 shrink-0 items-center justify-center rounded-md border bg-background/85 text-primary shadow-sm">
                {setupRequired ? (
                  <ShieldCheck className="size-5" />
                ) : (
                  <KeyRound className="size-5" />
                )}
              </div>
              <div className="min-w-0">
                <p className="font-data text-[10px] tracking-[0.16em] text-muted-foreground uppercase">
                  {t(setupRequired ? "auth.setupEyebrow" : "auth.loginEyebrow")}
                </p>
                <DialogTitle className="font-display mt-1">
                  {t(setupRequired ? "auth.setup" : "auth.login")}
                </DialogTitle>
                <DialogDescription className="mt-2 leading-5">
                  {t(
                    setupRequired
                      ? "auth.setupDescription"
                      : "auth.description",
                  )}
                </DialogDescription>
              </div>
            </div>
          </DialogHeader>
          <div className="grid gap-4 px-6 py-5">
            <div className="grid gap-2">
              <Label htmlFor="admin-password">{t("auth.password")}</Label>
              <Input
                id="admin-password"
                type="password"
                autoComplete={
                  setupRequired ? "new-password" : "current-password"
                }
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                maxLength={512}
                placeholder={
                  setupRequired
                    ? t("auth.newPasswordPlaceholder")
                    : t("auth.passwordPlaceholder")
                }
                aria-describedby={
                  setupRequired ? "admin-password-requirement" : undefined
                }
                autoFocus
              />
            </div>
            {setupRequired ? (
              <>
                <div className="grid gap-2">
                  <Label htmlFor="admin-password-confirmation">
                    {t("auth.confirmPassword")}
                  </Label>
                  <Input
                    id="admin-password-confirmation"
                    type="password"
                    autoComplete="new-password"
                    value={passwordConfirmation}
                    onChange={(event) =>
                      setPasswordConfirmation(event.target.value)
                    }
                    maxLength={512}
                    placeholder={t("auth.confirmPasswordPlaceholder")}
                    aria-invalid={
                      passwordConfirmation.length > 0 && !passwordsMatch
                    }
                  />
                  {passwordConfirmation.length > 0 && !passwordsMatch ? (
                    <p className="text-xs text-destructive">
                      {t("auth.passwordMismatch")}
                    </p>
                  ) : null}
                </div>
                <div
                  id="admin-password-requirement"
                  className="flex items-start gap-2 rounded-md border bg-muted/55 px-3 py-2.5 text-xs leading-5 text-muted-foreground"
                >
                  <CheckCircle2
                    className={
                      passwordLongEnough
                        ? "mt-0.5 size-3.5 shrink-0 text-primary"
                        : "mt-0.5 size-3.5 shrink-0"
                    }
                  />
                  <span>{t("auth.passwordRequirement")}</span>
                </div>
              </>
            ) : null}
          </div>
          <DialogFooter className="border-t bg-muted/25 px-6 py-4">
            {!setupRequired ? (
              <Button
                type="button"
                variant="outline"
                onClick={() => handleOpenChange(false)}
              >
                {t("action.cancel")}
              </Button>
            ) : null}
            <Button type="submit" disabled={!canSubmit || isSubmitting}>
              {isSubmitting ? (
                <LoaderCircle className="animate-spin" />
              ) : setupRequired ? (
                <ShieldCheck />
              ) : (
                <LogIn />
              )}
              {t(setupRequired ? "auth.setupAction" : "auth.login")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
