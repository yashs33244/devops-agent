"""Shared message serialization helpers for converting SDK messages to JSON."""

from __future__ import annotations

from dataclasses import asdict

from claude_agent_sdk import (
    AssistantMessage,
    RateLimitEvent,
    ResultMessage,
    SystemMessage,
    TextBlock,
    ThinkingBlock,
    ToolResultBlock,
    ToolUseBlock,
    UserMessage,
)
from claude_agent_sdk import StreamEvent as SDKStreamEvent

_CONTENT_BLOCK_TYPE: dict[type, str] = {
    TextBlock: "text",
    ThinkingBlock: "thinking",
    ToolUseBlock: "tool_use",
    ToolResultBlock: "tool_result",
}

_MESSAGE_TYPE_MAP: dict[type, str] = {
    UserMessage: "user",
    AssistantMessage: "assistant",
    ResultMessage: "result",
    RateLimitEvent: "rate_limit_event",
}


def message_type(message) -> str:
    """Derive a type discriminator string for an SDK message."""
    msg_type = _MESSAGE_TYPE_MAP.get(type(message))
    if msg_type:
        if msg_type == "result":
            return f"result.{message.subtype}"
        return msg_type
    if isinstance(message, SystemMessage):
        return f"system.{message.subtype}"
    if isinstance(message, SDKStreamEvent):
        return "stream_event"
    return type(message).__name__.lower()


def serialize_message(message) -> dict:
    """Convert an SDK message dataclass to a JSON-serializable dict with type discriminators."""
    msg_type = message_type(message)

    try:
        data = asdict(message)
    except TypeError:
        data = make_serializable(
            message.__dict__ if hasattr(message, "__dict__") else {"raw": str(message)}
        )

    data["type"] = msg_type

    # Inject type discriminators on content blocks
    if isinstance(message, (AssistantMessage, UserMessage)) and isinstance(
        message.content, list
    ):
        for i, block in enumerate(message.content):
            block_type = _CONTENT_BLOCK_TYPE.get(type(block))
            if block_type:
                data["content"][i]["type"] = block_type

    return data


def make_serializable(obj):
    """Recursively convert an object to JSON-serializable form."""
    if isinstance(obj, dict):
        return {k: make_serializable(v) for k, v in obj.items()}
    if isinstance(obj, (list, tuple)):
        return [make_serializable(item) for item in obj]
    if isinstance(obj, (str, int, float, bool, type(None))):
        return obj
    if hasattr(obj, "__dict__"):
        return make_serializable(obj.__dict__)
    return str(obj)
