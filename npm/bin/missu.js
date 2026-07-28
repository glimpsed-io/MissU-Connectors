#!/usr/bin/env node
// Entry point only — the client lives in ../lib/missu.js so that importing it
// (from tests, or another program) never runs the CLI.
import { main } from "../lib/missu.js";

main().catch((err) => {
  console.error("missu: " + err.message);
  process.exit(1);
});
