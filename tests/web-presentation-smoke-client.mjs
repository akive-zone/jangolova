#!/usr/bin/env node
import assert from "node:assert/strict";
import process from "node:process";

const providerURL = process.env.JANGOLOVA_PROVIDER_URL || "http://127.0.0.1:7392";
const token = process.env.JANGOLOVA_PROVIDER_TOKEN;
const cdpURL = process.env.PRESENTATION_CDP_URL || "http://127.0.0.1:9224";
const sourceURL = process.env.PRESENTATION_SOURCE_URL || "http://127.0.0.1:8081/";
const instanceID = "presentation-smoke";

assert.ok(token, "JANGOLOVA_PROVIDER_TOKEN is required");

async function request(path, { method = "GET", body } = {}) {
  const response = await fetch(`${providerURL}${path}`, {
    method,
    headers: {
      authorization: `Bearer ${token}`,
      ...(body === undefined ? {} : { "content-type": "application/json" }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await response.text();
  const value = text ? JSON.parse(text) : undefined;
  if (!response.ok) {
    throw new Error(`${method} ${path} returned ${response.status}: ${text}`);
  }
  return { status: response.status, value };
}

async function call(method, params) {
  const { value } = await request(`/v1/instances/${instanceID}/call`, {
    method: "POST",
    body: { method, params },
  });
  return value.result;
}

const { value: connected } = await request("/v1/instances", {
  method: "POST",
  body: {
    apiVersion: "interaction.engine/v1alpha1",
    instanceId: instanceID,
    engine: {
      adapter: "web-presentation",
      source: sourceURL,
      options: {
        policy: {
          maxHTMLBytes: 4096,
          maxCSSBytes: 2048,
          maxJavaScriptBytes: 2048,
          maxTotalBytes: 8192,
          allowedSourceOrigins: ["http://127.0.0.1:8081"],
          allowedAssetOrigins: ["self"],
        },
      },
    },
    target: {
      kind: "browser",
      endpoints: [{ name: "cdp", protocol: "cdp", url: cdpURL }],
    },
  },
});
assert.equal(connected.status, "connected");
for (const capability of ["presentation.write", "presentation.capture", "events"]) {
  assert.ok(connected.capabilities.includes(capability), `missing ${capability}`);
}

const write = await call("act", {
  name: "presentation.write",
  input: {
    expectedRevision: "0",
    html: '<article id="authored-card"><h1>Authored live</h1><button id="advance">Advance</button><img id="blocked-asset" src="http://127.0.0.1:8082/pixel.svg" alt=""></article>',
    css: "#authored-card { width: 420px; padding: 24px; background: rgb(20, 40, 80); }",
    js: "root.querySelector('#advance').addEventListener('click', () => emit('advance.clicked', { step: 2 }));",
  },
});
assert.equal(write.ok, true);
assert.equal(write.revision, "1");

await assert.rejects(
  call("act", {
    name: "presentation.write",
    input: { expectedRevision: "0", html: "<p>stale overwrite</p>" },
  }),
  /revision conflict/,
);

await new Promise((resolve) => setTimeout(resolve, 200));

const inspection = await call("act", {
  name: "presentation.execute",
  input: {
    code: "return { heading: root.querySelector('h1')?.textContent, button: root.querySelector('button')?.textContent, cardCount: root.querySelectorAll('#authored-card').length, blockedAssetWidth: root.querySelector('#blocked-asset')?.naturalWidth };",
  },
});
assert.deepEqual(inspection.result, {
  heading: "Authored live",
  button: "Advance",
  cardCount: 1,
  blockedAssetWidth: 0,
});

const batch = await call("events", {
  after: "0",
  types: ["presentation.write"],
  limit: 10,
});
assert.equal(batch.events.length, 1);
assert.equal(batch.events[0].type, "presentation.write");
assert.ok(batch.events[0].data.bytes > 0);

const capture = await call("act", {
  name: "presentation.capture",
  input: { fullPage: true },
});
const png = Buffer.from(capture.pngBase64, "base64");
assert.ok(png.length > 1000, `PNG is unexpectedly small: ${png.length} bytes`);
assert.deepEqual([...png.subarray(0, 8)], [137, 80, 78, 71, 13, 10, 26, 10]);

const disconnected = await request(`/v1/instances/${instanceID}`, { method: "DELETE" });
assert.equal(disconnected.status, 204);

process.stdout.write("Authored presentation policy, revision, DOM, event, capture, and disconnect checks passed\n");
