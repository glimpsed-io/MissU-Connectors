# Miss U More — connectors

Command-line clients for [Miss U More](https://missu.fyi) and the packaging that
distributes them. The app itself is one button; this repository is how you press
it from a terminal.

The server lives in a separate, private repository. Everything here talks to the
public API and nothing else, which is why it can be public.

```console
$ miss-u-more
Miss U sent 💛

You and Casey are Connected 💛 — you've sent 12, they've sent 11. Miss U is ready (4 left this minute).
```

## Install

| | |
| --- | --- |
| npm | `npx miss-u-more` (or `npm i -g miss-u-more`) |
| PyPI | `uvx miss-u-more` (or `pipx install miss-u-more`) |
| Homebrew | `brew install glimpsed-io/tap/miss-u-more` |
| Scoop | `scoop bucket add glimpsed-io https://github.com/glimpsed-io/scoop-bucket && scoop install miss-u-more` |
| Docker | `docker run --rm -e MISSU_TOKEN ghcr.io/glimpsed-io/miss-u-more` |
| Go | `go install github.com/glimpsed-io/MissU-Connectors/cmd/miss-u-more@latest` |
| Binary | grab an archive from [Releases](https://github.com/glimpsed-io/MissU-Connectors/releases) |

> **Status:** the code is complete and tested, but nothing is published yet — see
> [Publishing](#publishing). Until then, build from source:
> `go build ./cmd/miss-u-more`.

## Use

```sh
miss-u-more            # press the button
miss-u-more status     # the score, whether you're Connected, and any cooldown
miss-u-more login      # store a token so later runs need no environment variable
miss-u-more logout     # forget the stored token
miss-u-more --json     # raw JSON, for scripts
```

## Auth

Mint a personal API token at **missu.fyi → Settings → Assistants (MCP)**, then
either run `miss-u-more login` or set `MISSU_TOKEN`.

The token acts as you for exactly two endpoints — read your status, and press the
button — and nothing else. Revoke it any time from the same settings page.

| Variable | Meaning |
| --- | --- |
| `MISSU_TOKEN` | the personal API token |
| `MISSU_BASE_URL` | the server to talk to (default `https://missu.fyi`) |

All three clients read and write the same `~/.config/miss-u-more/token` (mode 0600), so
`miss-u-more login` once works whichever one you reach for.

## Layout

```
cmd/miss-u-more/     the Go CLI — the binary behind Homebrew, Scoop, Docker, go install
npm/           the npm package (native Node, zero dependencies)
python/        the PyPI package (native Python, stdlib only)
```

### Why three implementations instead of one binary plus wrappers?

Because the public API is two endpoints behind a bearer header:

```
GET  /v1/me/status   the score, connection, cooldown — plus a `summary`
                     sentence the server renders
POST /v1/missu       press the button
```

Each client is ~150 lines. A wrapper that downloads, verifies and caches a binary
per platform would be *more* code than the client it downloads, would need a
postinstall script, and would fail in sandboxes that block network access at
install time.

The one thing that could drift — how the scoreboard is worded — can't: the server
returns the sentence in `summary`, and every client prints it verbatim. The
fallback prose each client carries is only for a server older than that field.

## Development

```sh
go test ./...                                        # Go CLI
cd npm    && node --test                             # npm package
cd python && PYTHONPATH=src python -m unittest discover -s tests
```

Point any client at a local server with `MISSU_BASE_URL=http://127.0.0.1:23462`.

Release pipeline dry run, no credentials needed, publishes nothing:

```sh
goreleaser release --snapshot --clean
```

## Publishing

Every publish workflow is `workflow_dispatch`-only and defaults to a dry run —
pushing code never publishes anything. Before the first real run:

### Decide first

- **Licence.** There is no `LICENSE` file yet, and these are public artifacts. Pick
  one, add it, then set `license` in `npm/package.json`, `python/pyproject.toml`,
  and under `homebrew_casks`/`scoops` in `.goreleaser.yaml`.
- **The name.** Confirm `miss-u-more` is free on npm and PyPI. If not, the fallbacks are
  `@glimpsed/miss-u-more` on npm and a suffixed variant on PyPI — a one-line change in each manifest.

### npm — `publish-npm.yml`

Uses OIDC trusted publishing, so there is no token to store. On npmjs.com, add a
trusted publisher for this repo + `publish-npm.yml` (the package must exist
first, so either publish once by hand or pre-create it).

### PyPI — `publish-pypi.yml`

Also OIDC. Create GitHub environments named `testpypi` and `pypi`, then add
matching trusted publishers on TestPyPI and PyPI for this repo +
`publish-pypi.yml`. Ship to TestPyPI first — the workflow defaults to it.

### GHCR — part of `publish-cli.yml`

Nothing to create; `GITHUB_TOKEN` can push. After the first release, set the
package to public in the repo's Packages settings.

### Homebrew and Scoop — part of `publish-cli.yml`

Create two public repos, `glimpsed-io/homebrew-tap` and
`glimpsed-io/scoop-bucket`, then add a fine-grained PAT with `contents: write` on
each as the secrets `HOMEBREW_TAP_TOKEN` and `SCOOP_BUCKET_TOKEN`.
