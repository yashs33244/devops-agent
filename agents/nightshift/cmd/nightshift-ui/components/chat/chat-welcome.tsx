"use client";

import {
  PromptInput,
  PromptInputTextarea,
  PromptInputBody,
  PromptInputFooter,
  PromptInputSubmit,
} from "@/components/ai-elements/prompt-input";
import { ChatContextMenu } from "@/components/chat/context-menu";
import { PromptMascot } from "@/components/chat/prompt-mascot";
import {
  HiddenFileInput,
  StagedChips,
  useFileStaging,
} from "@/components/chat/file-staging";
import { Plus } from "lucide-react";

export function ChatWelcome({
  onSend,
  disabled,
}: {
  onSend: (text: string, files: File[]) => void | Promise<void>;
  disabled?: boolean;
}) {
  const {
    fileInputRef,
    staged,
    stagingError,
    stageFiles,
    removeStaged,
    submit,
  } = useFileStaging();

  return (
    <div className="flex-1 flex flex-col items-center justify-center px-4">
      <div className="w-full max-w-[720px]">
        <h1 className="text-3xl font-semibold text-primary tracking-tight text-center mb-7">
          What should we work on next?
        </h1>

        <StagedChips staged={staged} stagingError={stagingError} onRemove={removeStaged} />

        <div className="relative rounded-2xl border border-night-border bg-night-surface overflow-hidden">
          <PromptMascot />
          <PromptInput
            onSubmit={(message) => submit(message, onSend)}
            className="[&_[data-slot=input-group]]:rounded-none [&_[data-slot=input-group]]:border-0 [&_[data-slot=input-group]]:shadow-none"
          >
            <PromptInputBody>
              <PromptInputTextarea
                placeholder={disabled ? "Starting..." : "Ask anything"}
                disabled={disabled}
                className="min-h-[88px] max-h-[240px] bg-transparent border-0 focus:ring-0 text-[15px] text-primary placeholder:text-muted px-4 pt-4"
              />
            </PromptInputBody>
            <PromptInputFooter className="justify-between px-3 pb-3">
              <ChatContextMenu onUploadFile={() => fileInputRef.current?.click()}>
                <button type="button" className="size-8 flex items-center justify-center rounded-lg text-muted hover:text-secondary hover:bg-night-hover transition-colors">
                  <Plus size={16} />
                </button>
              </ChatContextMenu>
              <PromptInputSubmit disabled={disabled} />
            </PromptInputFooter>
          </PromptInput>
        </div>

        <HiddenFileInput inputRef={fileInputRef} onPick={stageFiles} />
      </div>
    </div>
  );
}
