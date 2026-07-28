# missu

The [Miss U More](https://missu.fyi) command line: let one person know you miss
them, without leaving the terminal.

```console
$ npx missu
Miss U sent 💛

You and Casey are Connected 💛 — you've sent 12, they've sent 11. Miss U is ready (4 left this minute).
```

## Install

```sh
npm install -g missu   # or just use npx missu, no install
```

Requires Node 18+. No dependencies.

## Use

```sh
missu            # press the button
missu status     # the score, whether you're Connected, and any cooldown
missu login      # store a token so later runs need no environment variable
missu logout     # forget the stored token
missu --json     # raw JSON, for scripts
```

## Auth

Mint a personal API token at **missu.fyi → Settings → Assistants (MCP)**, then
either run `missu login` (stores it in `~/.config/missu/token`, mode 0600) or set
`MISSU_TOKEN`.

The token acts as you for exactly two endpoints — read your status, and press the
button — and nothing else. Revoke it any time from the same settings page.

| Variable | Meaning |
| --- | --- |
| `MISSU_TOKEN` | the personal API token |
| `MISSU_BASE_URL` | the server to talk to (default `https://missu.fyi`) |

## Why so small?

The whole API is two endpoints behind a bearer header, so this package is a
native implementation rather than a downloader for a binary — nothing to fetch on
install, no postinstall script, and it runs wherever Node does. If you would
rather have a single binary, `brew install glimpsed-io/tap/missu` and
`scoop install missu` ship one.
