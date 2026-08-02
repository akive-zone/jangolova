import {
  permissionIncrease,
  publicDescription,
  registrationID,
  registrationPlan,
  requiresRenewedApproval,
  sourceRevision,
  validateManifest,
  type BrowserRegistration,
  type UserScriptManifest,
} from '@jangolova/userscript-runtime';
import { publishCymonkeyEvent } from './events';
import { isRecord } from '../types';

const storageKey = 'jangolova.userscripts.v1';

type NativeUserScripts = {
  getScripts(filter?: {ids?: string[]}): Promise<Array<{id: string}>>;
  register(scripts: BrowserRegistration[]): Promise<void>;
  update(scripts: BrowserRegistration[]): Promise<void>;
  unregister(filter?: {ids?: string[]}): Promise<void>;
};

export async function dispatchUserscript(method: string, input: Record<string, unknown>) {
  if (method === 'userscript.list') return listUserscripts();
  if (method === 'userscript.describe') return describeUserscript(requireID(input.id));
  if (method === 'userscript.install') return installUserscript(input);
  if (method === 'userscript.update') return updateUserscript(input);
  if (method === 'userscript.uninstall') return uninstallUserscript(requireID(input.id));
  if (method === 'userscript.enable') return setUserscriptEnabled(requireID(input.id), true);
  if (method === 'userscript.disable') return setUserscriptEnabled(requireID(input.id), false);
  throw new Error(`unsupported userscript method ${JSON.stringify(method)}`);
}

export async function describeUserscriptManager() {
  return {
    runtime: await describeRuntime(),
    installed: await listUserscripts(),
  };
}

export async function describeRuntime() {
  const api = nativeAPI();
  if (!api) return {status: 'unavailable', backend: null, reason: 'browser userScripts API is not exposed'};
  try {
    await api.getScripts();
    return {status: 'available', backend: 'webextension-userScripts'};
  } catch {
    return {status: 'unavailable', backend: 'webextension-userScripts', reason: 'browser userScripts permission or user setting is disabled'};
  }
}

export async function reconcileUserscripts() {
  const records = await readRecords();
  await syncNativeCatalog(records);
  const api = nativeAPI();
  if (!api) return describeRuntime();
  try {
    const expected = Object.values(records).filter((value) => value.spec.enabled);
    const expectedIDs = new Set(expected.map((value) => registrationID(value.metadata.id)));
    const registered = await api.getScripts();
    const orphaned = registered.map((value) => value.id).filter((id) => id.startsWith('jg-us-') && !expectedIDs.has(id));
    if (orphaned.length) await api.unregister({ids: orphaned});
    const actual = new Set(registered.map((value) => value.id));
    const missing = expected.filter((value) => !actual.has(registrationID(value.metadata.id)));
    if (missing.length) await api.register(missing.map(registrationPlan));
    return {status: 'available', registered: expectedIDs.size, restored: missing.length, removed: orphaned.length};
  } catch {
    return {status: 'unavailable', reason: 'userscript runtime reconciliation failed'};
  }
}

async function installUserscript(input: Record<string, unknown>) {
  if (input.approved !== true) throw new Error('userscript installation requires explicit approval');
  const manifest = await requireManifest(input.manifest, input.approveMainWorld === true);
  const records = await readRecords();
  if (records[manifest.metadata.id]) throw new Error('userscript is already installed');
  if (manifest.spec.enabled) await register(manifest);
  records[manifest.metadata.id] = manifest;
  await writeRecords(records);
  await publishCymonkeyEvent('userscript.installed', eventData(manifest));
  return publicDescription(manifest);
}

async function updateUserscript(input: Record<string, unknown>) {
  const manifest = await requireManifest(input.manifest, input.approveMainWorld === true);
  const records = await readRecords();
  const previous = records[manifest.metadata.id];
  if (!previous) throw new Error('userscript is not installed');
  const increase = permissionIncrease(previous, manifest);
  if (requiresRenewedApproval(increase) && input.approvedPermissionIncrease !== true) {
    throw new Error('userscript update requires renewed permission approval');
  }
  if (previous.spec.enabled && manifest.spec.enabled) await updateRegistration(manifest);
  else if (previous.spec.enabled) await unregister(manifest.metadata.id);
  else if (manifest.spec.enabled) await register(manifest);
  records[manifest.metadata.id] = manifest;
  await writeRecords(records);
  await publishCymonkeyEvent('userscript.updated', {...eventData(manifest), permissionIncrease: increase});
  return publicDescription(manifest);
}

async function uninstallUserscript(id: string) {
  const records = await readRecords();
  const existing = records[id];
  if (!existing) throw new Error('userscript is not installed');
  if (existing.spec.enabled) await unregister(id);
  delete records[id];
  await writeRecords(records);
  await publishCymonkeyEvent('userscript.uninstalled', {id});
  return {ok: true, id};
}

async function setUserscriptEnabled(id: string, enabled: boolean) {
  const records = await readRecords();
  const existing = records[id];
  if (!existing) throw new Error('userscript is not installed');
  if (existing.spec.enabled === enabled) return publicDescription(existing);
  const next = structuredClone(existing);
  next.spec.enabled = enabled;
  if (enabled) await register(next); else await unregister(id);
  records[id] = next;
  await writeRecords(records);
  await publishCymonkeyEvent(enabled ? 'userscript.enabled' : 'userscript.disabled', eventData(next));
  return publicDescription(next);
}

async function listUserscripts() {
  const records = await readRecords();
  return Object.values(records).sort((left, right) => left.metadata.id.localeCompare(right.metadata.id)).map(publicDescription);
}

async function describeUserscript(id: string) {
  const records = await readRecords();
  const value = records[id];
  if (!value) throw new Error('userscript is not installed');
  return publicDescription(value);
}

async function requireManifest(value: unknown, allowMainWorld: boolean) {
  if (!isRecord(value)) throw new Error('userscript manifest is required');
  const manifest = validateManifest(value as unknown as UserScriptManifest, {allowMainWorld});
  if (await sourceRevision(manifest.source.code) !== manifest.metadata.revision) throw new Error('userscript source does not match its revision');
  return manifest;
}

async function register(value: UserScriptManifest) {
  const api = await requireNativeAPI();
  await api.register([registrationPlan(value)]);
}

async function updateRegistration(value: UserScriptManifest) {
  const api = await requireNativeAPI();
  await api.update([registrationPlan(value)]);
}

async function unregister(id: string) {
  const api = await requireNativeAPI();
  await api.unregister({ids: [registrationID(id)]});
}

async function requireNativeAPI() {
  const api = nativeAPI();
  if (!api) throw new Error('browser userscript runtime is unavailable');
  try { await api.getScripts(); } catch { throw new Error('browser userscript permission or user setting is disabled'); }
  return api;
}

function nativeAPI(): NativeUserScripts | undefined {
  return (browser as unknown as {userScripts?: NativeUserScripts}).userScripts;
}

async function readRecords(): Promise<Record<string, UserScriptManifest>> {
  const stored = await browser.storage.local.get(storageKey);
  const value = stored[storageKey];
  return isRecord(value) ? value as unknown as Record<string, UserScriptManifest> : {};
}

async function writeRecords(value: Record<string, UserScriptManifest>) {
  await browser.storage.local.set({[storageKey]: value});
  await syncNativeCatalog(value);
}

async function syncNativeCatalog(value: Record<string, UserScriptManifest>) {
  if (import.meta.env.BROWSER !== 'safari') return;
  const runtime = browser.runtime as typeof browser.runtime & {
    sendNativeMessage?: (application: string, message: unknown) => Promise<unknown>;
  };
  if (!runtime.sendNativeMessage) return;
  const values = Object.values(value).map((manifest) => ({
    id: manifest.metadata.id,
    name: manifest.metadata.name,
    revision: manifest.metadata.revision,
    enabled: manifest.spec.enabled,
    matches: [...manifest.spec.matches],
  }));
  try {
    await runtime.sendNativeMessage('dev.jangolova.Jangolova', {
      method: 'userscripts.catalog.replace',
      values,
    });
  } catch {
    // The containing app is optional. Userscript state remains authoritative here.
  }
}

function requireID(value: unknown) {
  if (typeof value !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(value)) throw new Error('valid userscript id is required');
  return value;
}

function eventData(value: UserScriptManifest) {
  return {id: value.metadata.id, revision: value.metadata.revision, enabled: value.spec.enabled};
}
