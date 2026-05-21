"use client";

import { cn } from "@/lib/utils";

type FormFieldProps = {
  label: string;
  htmlFor?: string;
  error?: string;
  children: React.ReactNode;
  className?: string;
};

export function FormField({ label, htmlFor, error, children, className }: FormFieldProps) {
  return (
    <div data-slot="form-field" className={cn("space-y-1.5", className)}>
      <label
        htmlFor={htmlFor}
        className="text-xs uppercase tracking-wider text-muted"
      >
        {label}
      </label>
      {children}
      {error && (
        <p className="text-xs text-error">{error}</p>
      )}
    </div>
  );
}
