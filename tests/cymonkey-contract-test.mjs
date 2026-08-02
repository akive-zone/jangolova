import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import test from "node:test";

const root = new URL("../", import.meta.url);
const browserExtensionPackage = existsSync(new URL("pkg/browser-jangolova/package.json", root))
  ? "pkg/browser-jangolova/package.json"
  : "pkg/browser-cymonkey/package.json";
const browserExtensionConfig = browserExtensionPackage.replace("package.json", "wxt.config.ts");

async function source(path) {
  return readFile(new URL(path, root), "utf8");
}

test("Cymonkey worker provides CDP and BiDi baselines with an optional extension", async () => {
  const worker = await source("scripts/cymonkey-worker.mjs");
  const syntax = spawnSync(process.execPath, ["--check", fileURLToPath(new URL("scripts/cymonkey-worker.mjs", root))], { encoding: "utf8" });
  assert.equal(syntax.status, 0, syntax.stderr);
  assert.match(worker, /puppeteer\.connect/);
  assert.match(worker, /webDriverBiDi/);
  assert.match(worker, /jangolova\.cymonkey\/v1alpha1/);
  assert.match(worker, /extensionConfig\.mode === "required"/);
  assert.match(worker, /baseCapabilities\(targetProtocol\)/);
  assert.match(worker, /chrome-extension:\/\//);
  assert.match(worker, /cymonkeyDispatch/);
  assert.doesNotMatch(worker, /puppeteer\.launch/);
  assert.doesNotMatch(worker, /chromium\.launch/);
});

test("one reversible live client is shared by CDP and BiDi fixtures", async () => {
  const clientPath = new URL("tests/cymonkey-live-client.mjs", root);
  const client = await source("tests/cymonkey-live-client.mjs");
  const cdpFixture = await source("tests/docker/browser-interaction-smoke-test.sh");
  const bidiFixture = await source("tests/docker/firefox-bidi-smoke-test.sh");
  const syntax = spawnSync(process.execPath, ["--check", fileURLToPath(clientPath)], { encoding: "utf8" });
  assert.equal(syntax.status, 0, syntax.stderr);
  assert.match(client, /augmentation\.install/);
  assert.match(client, /augmentation\.uninstall/);
  assert.match(client, /finally/);
  assert.match(cdpFixture, /cymonkey-live-client\.mjs[\s\S]*--expect-backend cdp/);
  assert.match(bidiFixture, /cymonkey-live-client\.mjs[\s\S]*--expect-backend bidi/);
});

test("versioned Cymonkey schemas define capability provenance and one augmentation contract", async () => {
  const protocol = JSON.parse(await source("protocol/cymonkey/v1alpha1/protocol.schema.json"));
  const augmentation = JSON.parse(await source("protocol/cymonkey/v1alpha1/augmentation.schema.json"));
  assert.equal(protocol.$defs.hello.properties.protocolVersion.const, "jangolova.cymonkey/v1alpha1");
  for (const field of ["backend", "support", "lifetime", "persistence", "effect", "inputSchema"]) {
    assert.ok(protocol.$defs.capability.required.includes(field), `capability schema missing ${field}`);
  }
  assert.equal(augmentation.properties.apiVersion.const, "jangolova.cymonkey/v1alpha1");
  assert.equal(augmentation.properties.kind.const, "Augmentation");
  assert.deepEqual(augmentation.$defs.script.oneOf, [{ required: ["source"] }, { required: ["files"] }]);
});

test("transport mappings expose semantics without raw protocol passthrough", async () => {
  const worker = await source("scripts/cymonkey-worker.mjs");
  const safari = await source("adapters/cymonkey/safari_backend.go");
  for (const capability of ["augmentation.install", "script.execute", "script.register", "dom.query", "dom.observe", "dom.patch", "network.observe", "storage.get"]) {
    assert.match(worker, new RegExp(capability.replaceAll(".", "\\.")), `worker missing ${capability}`);
  }
  assert.match(safari, /browser\.evaluate/);
  assert.match(safari, /strings\.Contains\(lower, "preload"\)/);
  assert.match(safari, /network\.observe/);
  assert.doesNotMatch(worker, /cdp\.call|bidi\.call|browser\.api|chrome\.evaluate/);
});

test("CDP interception rules are augmentation-owned and cleaned up before disconnect", async () => {
  const worker = await source("scripts/cymonkey-worker.mjs");
  assert.match(worker, /existing\.augmentationId !== augmentationId/);
  assert.match(worker, /network rule \$\{id\} is not owned by augmentation/);
  assert.match(worker, /await disableInterception\(\).*browser\.disconnect\(\)/s);
  assert.match(worker, /page\.setRequestInterception\(false\)/);
  assert.match(worker, /protocol === "cdp"/);
});

test("WXT package defines standalone and Xallet spoke build matrices", async () => {
  const pkg = JSON.parse(await source(browserExtensionPackage));
  const config = await source(browserExtensionConfig);
  assert.match(pkg.scripts["build:standalone"], /chrome.*edge.*firefox/);
  assert.match(pkg.scripts["build:spoke"], /chrome.*edge.*firefox/);
  assert.match(pkg.scripts["build:spoke:chrome"], /--mode spoke/);
  assert.match(config, /mode === 'spoke'/);
  assert.match(config, /outDirTemplate: '\{\{browser\}\}-mv\{\{manifestVersion\}\}\{\{modeSuffix\}\}'/);
  assert.match(config, /\.\.\.\(spoke \? \['management'\] : \[\]\)/);
  assert.match(config, /externally_connectable: spoke/);
  assert.match(config, /data_collection_permissions/);
});

test("Cymonkey separates its page-safe and privileged extension planes", async () => {
  const background = await source("pkg/browser-cymonkey/entrypoints/background.ts");
  const content = await source("pkg/browser-cymonkey/entrypoints/cymonkey.content.ts");
  const page = await source("pkg/browser-cymonkey/entrypoints/cymonkey-main.ts");
  const capabilities = await source("pkg/browser-cymonkey/src/capabilities.ts");
  const engine = await source("pkg/browser-cymonkey/src/engine.ts");

  assert.match(page, /root\.cymonkey = Object\.freeze/);
  assert.match(content, /allowedPageActions/);
  assert.match(content, /cannot invoke privileged action/);
  assert.match(background, /acceptsExternalSender\(sender\.id\)/);
  assert.match(engine, /jangolova\.cymonkey\/v1alpha1/);
  assert.match(engine, /jangolova-(?:cymonkey|browser-extension)-webextension/);

  for (const capability of [
    "script.execute", "script.register", "script.unregister", "style.insert", "style.remove",
    "network.rules.install", "network.rules.remove", "storage.get", "storage.set",
  ]) {
    assert.match(capabilities, new RegExp(capability.replaceAll(".", "\\.")), `missing ${capability}`);
    assert.doesNotMatch(page, new RegExp(capability.replaceAll(".", "\\.")), `page bridge exposes ${capability}`);
  }
  assert.doesNotMatch(background, /browser\.api|chrome\.evaluate/);
});

test("Xallet spoke registration is optional and authenticated by discovered hub ID", async () => {
  const spoke = await source("pkg/browser-cymonkey/src/xallet-spoke.ts");
  const background = await source("pkg/browser-cymonkey/entrypoints/background.ts");
  assert.match(spoke, /extension\.name === hubName && extension\.enabled/);
  assert.match(spoke, /REGISTER_SPOKE/);
  assert.match(spoke, /UPDATE_SPOKE_STATE/);
  assert.match(spoke, /senderId === this\.hubId/);
  assert.match(background, /import\.meta\.env\.MODE === 'spoke'/);
  assert.match(background, /JANGOLOVA_EXTENSION_CALL/);
});
