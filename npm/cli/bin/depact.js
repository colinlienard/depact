#!/usr/bin/env node
"use strict";

const { spawnSync } = require("child_process");

// npm publishes win32 packages as "windows" to avoid spam-detection false
// positives; process.platform still reports "win32" at runtime, so map it.
const OS = { win32: "windows" }[process.platform] ?? process.platform;
const EXT = process.platform === "win32" ? ".exe" : "";
const PKG = `@depact/cli-${OS}-${process.arch}`;

function binaryPath() {
  try {
    return require.resolve(`${PKG}/bin/depact${EXT}`);
  } catch {
    throw new Error(
      `depact: no prebuilt binary for ${process.platform}-${process.arch}.\n` +
        `The optional dependency ${PKG} is not installed. This usually means ` +
        `your platform is unsupported, or optional dependencies were skipped ` +
        `(e.g. --no-optional / --ignore-optional).`
    );
  }
}

const result = spawnSync(binaryPath(), process.argv.slice(2), {
  stdio: "inherit",
});

if (result.error) {
  throw result.error;
}
process.exit(result.status ?? 1);
