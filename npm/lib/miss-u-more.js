// miss-u-more — the Miss U More command line, as an npm package.
//
// This is a native reimplementation of cmd/miss-u-more (the Go client) rather than a downloader
// for it: the API is two endpoints behind a bearer header, so the whole client
// is smaller than the code needed to fetch, verify and cache a binary — and it
// works on every platform Node runs on, with no postinstall script.
//
// Keep behaviour in step with cmd/miss-u-more and the Python client: same
// commands, same env vars, same token file.

import { readFile, writeFile, mkdir, unlink } from "node:fs/promises";
import { createInterface } from "node:readline/promises";
import { homedir } from "node:os";
import { join, dirname } from "node:path";

const DEFAULT_BASE = "https://missu.fyi";
const SOURCE = "cli"; // notification reads "… via the command line"

const USAGE = `miss-u-more — let one person know you miss them.

Usage:
  miss-u-more                press the button
  miss-u-more status         the score, whether you're Connected, and any cooldown
  miss-u-more login          store a personal API token for later runs
  miss-u-more logout         forget the stored token
  miss-u-more version        print the version

Flags:
  --token <token>      use this token instead of MISSU_TOKEN / the stored one
  --base <url>         API base URL (default $MISSU_BASE_URL or ${DEFAULT_BASE})
  --json               print raw JSON instead of prose

Mint a token at https://missu.fyi -> Settings -> Assistants (MCP).
`;

// tokenPath mirrors the Go CLI: ~/.config/miss-u-more/token, honouring XDG_CONFIG_HOME.
function tokenPath() {
  const base = process.env.XDG_CONFIG_HOME || join(homedir(), ".config");
  return join(base, "miss-u-more", "token");
}

async function resolveToken(flagToken) {
  if (flagToken) return flagToken;
  const env = (process.env.MISSU_TOKEN || "").trim();
  if (env) return env;
  try {
    const stored = (await readFile(tokenPath(), "utf8")).trim();
    if (stored) return stored;
  } catch {
    // no stored token; fall through to the error below
  }
  throw new Error("no token — run `miss-u-more login`, or set MISSU_TOKEN");
}

// request issues one authenticated call and turns any non-2xx into the API's
// own error message, so the user sees "not connected yet", not "HTTP 403".
async function request(base, token, method, path, body) {
  const res = await fetch(base.replace(/\/+$/, "") + path, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(body ? { "Content-Type": "application/json" } : {}),
    },
    ...(body ? { body: JSON.stringify(body) } : {}),
  });

  const text = await res.text();
  if (!res.ok) {
    let message = res.statusText;
    try {
      message = JSON.parse(text).error || message;
    } catch {
      // non-JSON error body; keep the status text
    }
    const retry = res.headers.get("retry-after");
    throw new Error(retry ? `${message} (retry in ${retry}s)` : message);
  }
  return text ? JSON.parse(text) : null;
}

// renderStatus prints the server's own `summary` sentence — the same words an
// assistant would say (internal/mcpsrv.RenderStatus) — so this package holds no
// copy of the prose to drift from. The fallback covers a server older than the
// summary field.
function renderStatus(st, tapped) {
  const lead = tapped ? "Miss U sent 💛\n\n" : "";
  if (st.summary) return lead + st.summary;

  const miss = st.miss || {};
  if (!miss.connected) {
    return (
      lead +
      (miss.email
        ? `You've named ${miss.email}, but they haven't named you back (yet) — not Connected.`
        : "Nobody's been named yet — open missu.fyi and choose who you miss.")
    );
  }
  const who = miss.connection?.name || miss.connection?.email || "them";
  const stats = miss.stats || {};
  return (
    lead +
    `You and ${who} are Connected 💛 — you've sent ${stats.sent_count ?? 0},` +
    ` they've sent ${stats.received_count ?? 0}.`
  );
}

async function login(base) {
  console.log("Mint a token at https://missu.fyi -> Settings -> Assistants (MCP).");
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  const token = (await rl.question("Paste it here: ")).trim();
  rl.close();
  if (!token) throw new Error("no token given");

  // Verify before saving, so a typo fails now rather than on the next tap.
  const st = await request(base, token, "GET", "/v1/me/status");

  const path = tokenPath();
  await mkdir(dirname(path), { recursive: true, mode: 0o700 });
  await writeFile(path, token + "\n", { mode: 0o600 });
  console.log(`Signed in as ${st.user.email}. Token saved to ${path}.`);
}

async function main() {
  const argv = process.argv.slice(2);
  let cmd = "";
  const flags = { token: "", base: "", json: false, help: false };

  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--token") flags.token = argv[++i] || "";
    else if (a === "--base") flags.base = argv[++i] || "";
    else if (a === "--json") flags.json = true;
    else if (a === "--help" || a === "-h") flags.help = true;
    else if (!a.startsWith("-") && !cmd) cmd = a;
    else throw new Error(`unknown flag ${a} — run \`miss-u-more --help\``);
  }

  if (flags.help || cmd === "help") {
    process.stdout.write(USAGE);
    return;
  }

  const base = flags.base || process.env.MISSU_BASE_URL || DEFAULT_BASE;

  switch (cmd) {
    case "version": {
      const pkg = JSON.parse(
        await readFile(new URL("../package.json", import.meta.url), "utf8"),
      );
      console.log(`miss-u-more ${pkg.version}`);
      return;
    }
    case "logout":
      await unlink(tokenPath()).catch(() => {});
      console.log("Token forgotten.");
      return;
    case "login":
      return login(base);
    case "":
    case "status": {
      const token = await resolveToken(flags.token);
      if (cmd === "") await request(base, token, "POST", "/v1/missu", { source: SOURCE });
      const st = await request(base, token, "GET", "/v1/me/status");
      console.log(flags.json ? JSON.stringify(st, null, 2) : renderStatus(st, cmd === ""));
      return;
    }
    default:
      throw new Error(`unknown command "${cmd}" — run \`miss-u-more --help\``);
  }
}

// The executable is bin/miss-u-more.js, which just calls main(). Keeping the entry
// point separate from the library means no "was I invoked directly?" guard —
// which silently no-ops when the package is installed as a symlink (npm link,
// pnpm, a `file:` dependency).
export { main, renderStatus, tokenPath, request, resolveToken };
