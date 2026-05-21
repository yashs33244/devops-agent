"use client";

import { useState, useRef, useEffect } from "react";
import { Pencil } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

type RenameDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentName: string;
  onConfirm: (newName: string) => void;
};

export function RenameDialog({ open, onOpenChange, currentName, onConfirm }: RenameDialogProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [name, setName] = useState(currentName);

  // Reset name when dialog opens with a new currentName
  useEffect(() => {
    if (open) setName(currentName);
  }, [open, currentName]);

  // Select filename before extension when dialog opens
  useEffect(() => {
    if (!open) return;
    // Small delay to allow Radix to render the content
    const timer = setTimeout(() => {
      const input = inputRef.current;
      if (input) {
        input.focus();
        const dotIdx = currentName.lastIndexOf(".");
        if (dotIdx > 0) {
          input.setSelectionRange(0, dotIdx);
        } else {
          input.select();
        }
      }
    }, 0);
    return () => clearTimeout(timer);
  }, [open, currentName]);

  const trimmed = name.trim();
  const isValid = trimmed.length > 0 && !trimmed.includes("/") && trimmed !== currentName;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (isValid) onConfirm(trimmed);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false} className="max-w-sm rounded-2xl">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <div className="flex items-start gap-3">
              <div className="shrink-0 size-10 flex items-center justify-center rounded-lg border border-lime/30 bg-lime/10 text-lime">
                <Pencil size={18} />
              </div>
              <div className="flex-1 min-w-0">
                <DialogTitle className="text-base font-semibold text-primary">Rename</DialogTitle>
                <DialogDescription className="text-sm text-muted mt-1">Enter a new name</DialogDescription>
              </div>
            </div>
          </DialogHeader>
          <input
            ref={inputRef}
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={currentName}
            className="form-input w-full text-sm mt-4"
          />
          <DialogFooter className="flex gap-3 pt-5">
            <Button
              type="button"
              variant="outline"
              className="flex-1"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={!isValid}
              className="flex-1"
            >
              Rename
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
