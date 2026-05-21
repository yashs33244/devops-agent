"use client";

import Link from "next/link";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Plus, Plug, Bot, Zap, Paperclip } from "lucide-react";

export function ChatContextMenu({
  children,
  onUploadFile,
}: {
  children?: React.ReactNode;
  onUploadFile?: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        {children ?? (
          <button
            type="button"
            className="size-7 flex items-center justify-center rounded-lg text-muted hover:text-secondary hover:bg-night-hover transition-colors"
          >
            <Plus size={16} />
          </button>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-52">
        {onUploadFile && (
          <>
            <DropdownMenuItem
              onSelect={(e) => {
                e.preventDefault();
                onUploadFile();
              }}
              className="flex items-center gap-2 w-full"
            >
              <Paperclip size={14} />
              Upload file
            </DropdownMenuItem>
            <DropdownMenuSeparator />
          </>
        )}
        <DropdownMenuItem asChild>
          <Link href="/agents" className="flex items-center gap-2 w-full">
            <Bot size={14} />
            Subagents
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/skills" className="flex items-center gap-2 w-full">
            <Zap size={14} />
            Skills
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/connectors" className="flex items-center gap-2 w-full">
            <Plug size={14} />
            Connectors
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
