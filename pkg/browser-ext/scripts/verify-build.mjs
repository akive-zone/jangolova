import assert from 'node:assert/strict';
import { access, readFile } from 'node:fs/promises';

const root = new URL('../.output/', import.meta.url);
const browsers = ['chrome', 'edge', 'firefox'];

for (const browser of browsers) {
  await verify(`${browser}-mv3`);
}

async function verify(directory) {
  const output = new URL(`${directory}/`, root);
  const manifest = JSON.parse(await readFile(new URL('manifest.json', output), 'utf8'));
  assert.equal(manifest.manifest_version, 3, `${directory}: expected MV3`);
  assert.equal(manifest.name, 'Jangolova Browser Extension');
  assert.ok(manifest.permissions.includes('management'), `${directory}: Xallet Spook discovery permission missing`);
  assert.ok(manifest.externally_connectable, `${directory}: Xallet Spook control entry point missing`);
  assert.ok(manifest.permissions.includes('scripting'));
  assert.ok(manifest.permissions.includes('declarativeNetRequest'));
  for (const path of ['background.js', 'control.html', 'popup.html', 'cymonkey-main.js', 'content-scripts/cymonkey.js']) {
    await access(new URL(path, output));
  }
}

console.log('verified single-build standalone plus Xallet Spook behavior for Chrome, Edge, and Firefox');
