"""miss-u-more — the Miss U More command line, as a PyPI package.

A native reimplementation of cmd/miss-u-more (the Go client) rather than a downloader for it: the
API is two endpoints behind a bearer header, so the whole client is smaller than
the code needed to fetch, verify and cache a binary — and it works wherever
Python does, with no build step and no dependencies.

Keep behaviour in step with cmd/miss-u-more and the npm client: same commands,
same environment variables, same token file.
"""

from __future__ import annotations

import json
import os
import urllib.error
import urllib.request
from pathlib import Path

__version__ = "0.1.0"

DEFAULT_BASE = "https://missu.fyi"
SOURCE = "cli"  # notification reads "… via the command line"


class MissUError(Exception):
    """A failure worth showing the user verbatim, not a traceback."""


def token_path() -> Path:
    """Mirror the Go CLI: ~/.config/miss-u-more/token, honouring XDG_CONFIG_HOME."""
    base = os.environ.get("XDG_CONFIG_HOME") or (Path.home() / ".config")
    return Path(base) / "miss-u-more" / "token"


def resolve_token(flag_token: str = "") -> str:
    """Find the bearer credential: flag, then environment, then stored file."""
    if flag_token:
        return flag_token
    env = os.environ.get("MISSU_TOKEN", "").strip()
    if env:
        return env
    try:
        stored = token_path().read_text(encoding="utf-8").strip()
    except OSError:
        stored = ""
    if stored:
        return stored
    raise MissUError("no token — run `miss-u-more login`, or set MISSU_TOKEN")


def request(base: str, token: str, method: str, path: str, body: dict | None = None) -> dict | None:
    """Issue one authenticated call, raising MissUError with the API's own message."""
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        base.rstrip("/") + path,
        data=data,
        method=method,
        headers={
            "Authorization": f"Bearer {token}",
            **({"Content-Type": "application/json"} if data else {}),
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=15) as res:
            raw = res.read()
    except urllib.error.HTTPError as e:
        detail = e.read().decode(errors="replace")
        message = e.reason
        try:
            message = json.loads(detail).get("error") or message
        except ValueError:
            pass
        retry = e.headers.get("Retry-After") if e.headers else None
        raise MissUError(f"{message} (retry in {retry}s)" if retry else str(message)) from None
    except urllib.error.URLError as e:
        raise MissUError(f"could not reach {base}: {e.reason}") from None
    return json.loads(raw) if raw else None


def render_status(status: dict, tapped: bool) -> str:
    """Print the server's own `summary` sentence — the same words an assistant
    would say (internal/mcpsrv.RenderStatus) — so this package holds no copy of
    the prose to drift from. The fallback covers a pre-`summary` server.
    """
    lead = "Miss U sent 💛\n\n" if tapped else ""
    if status.get("summary"):
        return lead + status["summary"]

    miss = status.get("miss") or {}
    if not miss.get("connected"):
        if miss.get("email"):
            return lead + (
                f"You've named {miss['email']}, but they haven't named you back (yet)"
                " — not Connected."
            )
        return lead + "Nobody's been named yet — open missu.fyi and choose who you miss."

    conn = miss.get("connection") or {}
    who = conn.get("name") or conn.get("email") or "them"
    stats = miss.get("stats") or {}
    return lead + (
        f"You and {who} are Connected 💛 — you've sent {stats.get('sent_count', 0)},"
        f" they've sent {stats.get('received_count', 0)}."
    )
