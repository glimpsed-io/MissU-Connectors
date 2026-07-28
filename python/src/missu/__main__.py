"""Command-line entry point: `missu`, `missu status`, `missu login`, ..."""

from __future__ import annotations

import argparse
import json
import os
import sys

from . import (
    DEFAULT_BASE,
    SOURCE,
    MissUError,
    __version__,
    render_status,
    request,
    resolve_token,
    token_path,
)

USAGE = """missu — let one person know you miss them.

Usage:
  missu                press the button
  missu status         the score, whether you're Connected, and any cooldown
  missu login          store a personal API token for later runs
  missu logout         forget the stored token
  missu version        print the version

Flags:
  --token <token>      use this token instead of MISSU_TOKEN / the stored one
  --base <url>         API base URL (default $MISSU_BASE_URL or %s)
  --json               print raw JSON instead of prose

Mint a token at https://missu.fyi -> Settings -> Assistants (MCP).
""" % DEFAULT_BASE


def login(base: str) -> None:
    """Prompt for a pasted token, verify it, then store it for later runs."""
    print("Mint a token at https://missu.fyi -> Settings -> Assistants (MCP).")
    try:
        token = input("Paste it here: ").strip()
    except EOFError:
        token = ""
    if not token:
        raise MissUError("no token given")

    # Verify before saving, so a typo fails now rather than on the next tap.
    status = request(base, token, "GET", "/v1/me/status") or {}

    path = token_path()
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.write_text(token + "\n", encoding="utf-8")
    path.chmod(0o600)
    email = (status.get("user") or {}).get("email", "you")
    print(f"Signed in as {email}. Token saved to {path}.")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("command", nargs="?", default="")
    parser.add_argument("--token", default="")
    parser.add_argument("--base", default="")
    parser.add_argument("--json", action="store_true", dest="as_json")
    parser.add_argument("-h", "--help", action="store_true", dest="show_help")

    try:
        args = parser.parse_args(argv if argv is not None else sys.argv[1:])
    except SystemExit:
        sys.stderr.write(USAGE)
        return 2

    if args.show_help or args.command == "help":
        sys.stdout.write(USAGE)
        return 0

    base = args.base or os.environ.get("MISSU_BASE_URL") or DEFAULT_BASE

    try:
        if args.command == "version":
            print(f"missu {__version__}")
            return 0

        if args.command == "logout":
            token_path().unlink(missing_ok=True)
            print("Token forgotten.")
            return 0

        if args.command == "login":
            login(base)
            return 0

        if args.command in ("", "status"):
            token = resolve_token(args.token)
            tapped = args.command == ""
            if tapped:
                request(base, token, "POST", "/v1/missu", {"source": SOURCE})
            status = request(base, token, "GET", "/v1/me/status") or {}
            if args.as_json:
                print(json.dumps(status, indent=2, ensure_ascii=False))
            else:
                print(render_status(status, tapped))
            return 0

        raise MissUError(f'unknown command "{args.command}" — run `missu --help`')

    except MissUError as e:
        print(f"missu: {e}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
