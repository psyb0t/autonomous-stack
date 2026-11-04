"""Send one plain-text progress update to the configured owner, without polling."""

import json
import os
import re
import sys
import urllib.error
import urllib.request

MAX_MESSAGE_UNITS = 4096
MAX_RESPONSE_BYTES = 65536
REQUEST_TIMEOUT_SECONDS = 15


def send(text: str) -> None:
    """Validate configuration and send to the fixed Telegram endpoint."""
    token = os.environ.get("CLAUDEBOX_TELEGRAM_MODE_TOKEN", "")
    chat = os.environ.get("TELEGRAM_CHAT_ID", "")
    if not re.fullmatch(r"[0-9]+:[A-Za-z0-9_-]+", token):
        raise ValueError("Telegram bot token is missing or malformed")
    if not re.fullmatch(r"-?[1-9][0-9]*", chat):
        raise ValueError("Telegram owner chat ID is missing or malformed")
    if not text.strip() or len(text.encode("utf-16-le")) // 2 > MAX_MESSAGE_UNITS:
        raise ValueError("Telegram update must contain 1 through 4096 UTF-16 units")
    payload = json.dumps({
        "chat_id": int(chat), "text": text,
        "link_preview_options": {"is_disabled": True},
    }).encode()
    request = urllib.request.Request(
        f"https://api.telegram.org/bot{token}/sendMessage",
        data=payload, headers={"Content-Type": "application/json"},
    )
    try:
        # The HTTPS host is fixed and the token grammar cannot contain a URL or path.
        # nosemgrep: python.lang.security.audit.dynamic-urllib-use-detected.dynamic-urllib-use-detected
        with urllib.request.urlopen(request, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            body = response.read(MAX_RESPONSE_BYTES + 1)
        if len(body) > MAX_RESPONSE_BYTES:
            raise ValueError("oversized response")
        result = json.loads(body)
    except (urllib.error.URLError, OSError, ValueError) as error:
        # Exception URLs can contain the bot token; expose only a fixed message.
        raise RuntimeError("Telegram progress delivery failed") from error
    if not isinstance(result, dict) or result.get("ok") is not True:
        raise RuntimeError("Telegram rejected the progress update")


def main() -> int:
    """Return a JSON CLI result without credentials or provider error bodies."""
    try:
        send(sys.stdin.read(MAX_MESSAGE_UNITS * 2 + 1))
    except (ValueError, RuntimeError, UnicodeError) as error:
        print(json.dumps({"ok": False, "error": str(error)}))
        return 1
    print(json.dumps({"ok": True}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
