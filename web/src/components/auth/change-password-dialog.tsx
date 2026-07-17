import { useState } from "react";
import { CheckCircle2, KeyRound, LoaderCircle } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

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

export function ChangePasswordDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { changePassword } = useAuth();
  const queryClient = useQueryClient();
  const { t } = useI18n();
  const [password, setPassword] = useState("");
  const [passwordConfirmation, setPasswordConfirmation] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const passwordLongEnough = Array.from(password).length >= 8;
  const passwordsMatch =
    passwordConfirmation.length > 0 && password === passwordConfirmation;
  const canSubmit = passwordLongEnough && passwordsMatch;

  const handleOpenChange = (nextOpen: boolean) => {
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
      await changePassword(password, passwordConfirmation);
      await queryClient.invalidateQueries();
      toast.success(t("message.passwordChanged"));
      handleOpenChange(false);
    } catch (error) {
      toast.error(t("message.passwordChangeFailed"), {
        description: getApiErrorMessage(error),
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit}>
          <DialogHeader>
            <div className="mb-2 flex size-10 items-center justify-center rounded-md border bg-muted text-primary">
              <KeyRound className="size-5" />
            </div>
            <DialogTitle className="font-display">
              {t("auth.changePassword")}
            </DialogTitle>
            <DialogDescription className="leading-5">
              {t("auth.changePasswordDescription")}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-5">
            <div className="grid gap-2">
              <Label htmlFor="new-admin-password">
                {t("auth.newPassword")}
              </Label>
              <Input
                id="new-admin-password"
                type="password"
                autoComplete="new-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                maxLength={512}
                placeholder={t("auth.newPasswordPlaceholder")}
                autoFocus
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="new-admin-password-confirmation">
                {t("auth.confirmPassword")}
              </Label>
              <Input
                id="new-admin-password-confirmation"
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
            <div className="flex items-start gap-2 rounded-md border bg-muted/55 px-3 py-2.5 text-xs leading-5 text-muted-foreground">
              <CheckCircle2
                className={
                  passwordLongEnough
                    ? "mt-0.5 size-3.5 shrink-0 text-primary"
                    : "mt-0.5 size-3.5 shrink-0"
                }
              />
              <span>{t("auth.passwordRequirement")}</span>
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => handleOpenChange(false)}
            >
              {t("action.cancel")}
            </Button>
            <Button type="submit" disabled={!canSubmit || isSubmitting}>
              {isSubmitting ? (
                <LoaderCircle className="animate-spin" />
              ) : (
                <KeyRound />
              )}
              {t("action.saveChanges")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
