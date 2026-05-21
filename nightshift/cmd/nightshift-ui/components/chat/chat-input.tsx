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
import { uploadSessionFile, type ArtifactInfo } from "@/lib/api";

export function ChatInput({
  onSend,
  onStop,
  streaming,
  disabled,
  seed,
  conversationId,
}: {
  onSend: (text: string, files: File[]) => void | Promise<void>;
  onStop?: () => void;
  streaming?: boolean;
  disabled?: boolean;
  seed?: string | null;
  conversationId?: string | null;
}) {
  const {
    fileInputRef,
    staged,
    stagingError,
    stageFiles,
    removeStaged,
    submit,
  } = useFileStaging(conversationId);

  const showUpload = !!conversationId;

  return (
    <div className="px-8 md:px-12 lg:px-16 pb-4 pt-2">
      <StagedChips staged={staged} stagingError={stagingError} onRemove={removeStaged} />

      {/* `relative` so the absolute-positioned PromptMascot anchors to the
          input card below, not the padded wrapper. */}
      <div className="relative">
        <PromptMascot seed={seed} />
        <PromptInput
          onSubmit={(message) => {
            if (streaming || disabled) return;
            return submit(message, onSend);
          }}
          className="rounded-xl bg-night-surface overflow-hidden [&_[data-slot=input-group]]:rounded-xl [&_[data-slot=input-group]]:border-night-border [&_[data-slot=input-group]]:shadow-none"
        >
          <PromptInputBody>
            <PromptInputTextarea
              placeholder={disabled ? "Waiting for response..." : "Type a command..."}
              disabled={disabled}
              className="min-h-[44px] max-h-[200px] bg-transparent border-0 focus:ring-0 text-sm text-primary placeholder:text-muted"
            />
          </PromptInputBody>
          <PromptInputFooter className="justify-between px-2 pb-2">
            <ChatContextMenu
              onUploadFile={
                showUpload ? () => fileInputRef.current?.click() : undefined
              }
            >
              <button type="button" className="size-6 flex items-center justify-center rounded text-muted hover:text-secondary hover:bg-night-hover transition-colors">
                <Plus size={14} />
              </button>
            </ChatContextMenu>
            <PromptInputSubmit
              status={streaming ? "streaming" : undefined}
              onStop={onStop}
              disabled={disabled || (streaming && !onStop)}
            />
          </PromptInputFooter>
        </PromptInput>
      </div>

      {showUpload && <HiddenFileInput inputRef={fileInputRef} onPick={stageFiles} />}
    </div>
  );
}

// Sequential for stable artifact ordering and to bound the client-side
// burst on large uploads.
export async function uploadStagedFiles(
  sessionId: string,
  files: File[],
): Promise<ArtifactInfo[]> {
  const out: ArtifactInfo[] = [];
  for (const f of files) {
    out.push(await uploadSessionFile(sessionId, f));
  }
  return out;
}
