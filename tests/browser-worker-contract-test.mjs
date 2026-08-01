import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import test from "node:test";

const root = new URL("../", import.meta.url);

test("browser worker owns interaction libraries but not a browser runtime", async () => {
  const manifest = JSON.parse(await readFile(new URL("package.json", root)));
  const worker = await readFile(new URL("scripts/browser-worker.mjs", root), "utf8");

  const syntax = spawnSync(process.execPath, ["--check", fileURLToPath(new URL("scripts/browser-worker.mjs", root))], { encoding: "utf8" });
  assert.equal(syntax.status, 0, syntax.stderr);

  assert.equal(manifest.dependencies["playwright-core"], "1.51.1");
  assert.equal(manifest.dependencies["puppeteer-core"], "25.4.0");
  assert.match(worker, /chromium\.connectOverCDP/);
  assert.match(worker, /puppeteer\.connect/);
  assert.match(worker, /protocol: "webDriverBiDi"/);
  assert.match(worker, /connectOverCDP\(endpoint, \{ headers:/);
  assert.match(worker, /headers: safeHeaders\(headers\)/);
  assert.doesNotMatch(worker, /\.launch\s*\(/);
  for (const method of ["hello", "capabilities", "describe", "act", "events"]) {
    assert.ok(worker.includes(`method === "${method}"`), `missing ${method}`);
  }
});
