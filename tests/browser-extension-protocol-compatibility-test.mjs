import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const root = new URL('../', import.meta.url);
const fixture = async (name) => JSON.parse(await readFile(new URL(`protocol/browser-extension/v1alpha1/fixtures/${name}.json`, root), 'utf8'));

test('generated browser-extension bindings match the checked-in schema', async () => {
  const result = spawnSync(process.execPath, ['scripts/generate-browser-extension-protocol.mjs', '--check'], {
    cwd: new URL('.', root), encoding: 'utf8',
  });
  assert.equal(result.status, 0, result.stderr || result.stdout);
  const [schema, typescript, go] = await Promise.all([
    readFile(new URL('protocol/browser-extension/v1alpha1/protocol.schema.json', root), 'utf8'),
    readFile(new URL('pkg/browser-ext/src/generated/browser-extension-v1alpha1.ts', root), 'utf8'),
    readFile(new URL('internal/browserextensionprotocol/generated_v1alpha1.go', root), 'utf8'),
  ]);
  const parsed = JSON.parse(schema);
  const methods = parsed.$defs.controlCall.allOf.flatMap((condition) => condition.then.properties.method.enum);
  for (const method of new Set(methods)) {
    assert.match(typescript, new RegExp(JSON.stringify(method).replaceAll('.', '\\.')));
    assert.match(go, new RegExp(JSON.stringify(method).replaceAll('.', '\\.')));
  }
});

test('legacy and current Cymonkey envelopes preserve the same exchange', async () => {
  const legacy = await fixture('legacy-cymonkey-act');
  const current = await fixture('extension-cymonkey-act');
  assert.deepEqual(normalize(legacy), normalize(current));
  assert.equal((await fixture('policy-replace')).method, 'policy.replace');
  assert.equal((await fixture('websocket-auth')).protocolVersion, 'jangolova.browser-extension/v1alpha1');
});

function normalize(value) {
  if (value.type === 'CYMONKEY_CALL') return {method: value.method, params: value.params};
  assert.equal(value.method, 'cymonkey.call');
  return {method: value.params.method, params: value.params.params};
}
