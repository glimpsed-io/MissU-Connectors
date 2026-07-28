import { test } from "node:test";
import assert from "node:assert/strict";

import { renderStatus, request } from "../lib/missu.js";

test("renderStatus prefers the server's canonical summary", () => {
  const out = renderStatus({ summary: "You and Casey are Connected 💛 — you've sent 7." }, false);
  assert.equal(out, "You and Casey are Connected 💛 — you've sent 7.");
});

test("renderStatus leads with confirmation after a tap", () => {
  const out = renderStatus({ summary: "You and Casey are Connected 💛" }, true);
  assert.match(out, /^Miss U sent 💛\n\n/);
  assert.match(out, /You and Casey are Connected/);
});

test("renderStatus falls back to local prose on an older server", () => {
  const out = renderStatus(
    {
      miss: { connected: true, connection: { name: "Casey" }, stats: { sent_count: 7, received_count: 5 } },
    },
    false,
  );
  assert.match(out, /You and Casey are Connected/);
  assert.match(out, /you've sent 7, they've sent 5/);
});

test("renderStatus explains a one-sided miss", () => {
  const out = renderStatus({ miss: { connected: false, email: "them@x.test" } }, false);
  assert.match(out, /haven't named you back/);
  assert.match(out, /them@x\.test/);
});

test("request sends the bearer token and surfaces API error messages", async () => {
  const { createServer } = await import("node:http");
  const seen = {};
  const server = createServer((req, res) => {
    seen.auth = req.headers.authorization;
    seen.path = req.url;
    res.writeHead(403, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "connect to someone first" }));
  });
  await new Promise((r) => server.listen(0, "127.0.0.1", r));
  const base = `http://127.0.0.1:${server.address().port}`;

  await assert.rejects(
    () => request(base, "tok_abc", "GET", "/v1/me/status"),
    /connect to someone first/,
  );
  assert.equal(seen.auth, "Bearer tok_abc");
  assert.equal(seen.path, "/v1/me/status");
  server.close();
});
