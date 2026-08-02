import assert from 'node:assert/strict';
import { access, readFile } from 'node:fs/promises';

const root = new URL('../.output/', import.meta.url);
const browsers = ['chrome', 'edge', 'firefox'];

for (const browser of browsers) {
  await verify(`${browser}-mv3`, false);
  await verify(`${browser}-mv3-spoke`, true);
}

async function verify(directory, spoke) {
  const output = new URL(`${directory}/`, root);
  const manifest = JSON.parse(await readFile(new URL('manifest.json', output), 'utf8'));
  assert.equal(manifest.manifest_version, 3, `${directory}: expected MV3`);
  assert.equal(manifest.name, spoke ? 'Xallet Spoke: Cymonkey' : 'Jangolova Cymonkey');
  assert.equal(manifest.permissions.includes('management'), spoke, `${directory}: management permission mode mismatch`);
  assert.equal(Boolean(manifest.externally_connectable), spoke, `${directory}: external connection mode mismatch`);
  assert.ok(manifest.permissions.includes('scripting'));
  assert.ok(manifest.permissions.includes('declarativeNetRequest'));
  for (const path of ['background.js', 'control.html', 'popup.html', 'cymonkey-main.js', 'content-scripts/cymonkey.js']) {
    await access(new URL(path, output));
  }
}

console.log('verified standalone and Xallet spoke builds for Chrome, Edge, and Firefox');
