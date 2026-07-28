#!/usr/bin/env node
// Entry point only — the client lives in ../lib/miss-u-more.js so that importing it
// (from tests, or another program) never runs the CLI.
import { main } from "../lib/miss-u-more.js";

main().catch((err) => {
  console.error("miss-u-more: " + err.message);
  process.exit(1);
});
