import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

test("presentation worker attaches through Puppeteer CDP and never launches a runtime", async () => {
  const source = await readFile(new URL("../scripts/presentation-worker.mjs", import.meta.url), "utf8");
  assert.match(source, /puppeteer\.connect/);
  assert.match(source, /window\.jangolova/);
  assert.match(source, /presentation\.capture/);
  assert.doesNotMatch(source, /puppeteer\.launch/);
});

test("web presentation host exposes declarative create, patch, and semantic activation", async () => {
  const source = await readFile(new URL("../examples/web-presentation/main.js", import.meta.url), "utf8");
  for (const capability of ["presentation.create", "presentation.replace", "presentation.write", "presentation.execute", "presentation.patch", "presentation.describe", "presentation.activate"]) {
    assert.match(source, new RegExp(capability.replaceAll(".", "\\.")));
  }
  assert.match(source, /window\.jangolova/);
});
