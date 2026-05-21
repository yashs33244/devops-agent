"use client";

import { useParams } from "next/navigation";
import { ChatLayout } from "@/components/chat/chat-layout";

export default function ConversationPage() {
  const { conversationId } = useParams<{ conversationId: string }>();
  return <ChatLayout initialConversationId={conversationId} />;
}
