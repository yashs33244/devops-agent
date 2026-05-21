"use client";

import type { ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type ConfirmDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: ReactNode;
  onConfirm: () => void;
  confirmLabel?: string;
  variant?: "destructive" | "default";
  icon?: ReactNode;
};

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  onConfirm,
  confirmLabel = "Confirm",
  variant = "destructive",
  icon,
}: ConfirmDialogProps) {
  const isDestructive = variant === "destructive";
  const iconNode = icon ?? (isDestructive ? <AlertTriangle size={18} /> : null);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className={cn(
          "max-w-sm rounded-2xl",
          isDestructive && "border-error/20",
        )}
      >
        <DialogHeader>
          <div className="flex items-start gap-3">
            {iconNode && (
              <div
                className={cn(
                  "shrink-0 size-10 flex items-center justify-center rounded-lg border",
                  isDestructive
                    ? "border-error/30 bg-error/10 text-error"
                    : "border-lime/30 bg-lime/10 text-lime",
                )}
              >
                {iconNode}
              </div>
            )}
            <div>
              <DialogTitle className="text-base">{title}</DialogTitle>
              <DialogDescription className="mt-1 text-muted">
                {description}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>
        <DialogFooter className="flex-row gap-3 pt-1 sm:flex-row">
          <Button
            variant="outline"
            className="flex-1"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            variant={isDestructive ? "destructive" : "default"}
            className="flex-1"
            onClick={() => {
              onConfirm();
              onOpenChange(false);
            }}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
