import { privilegedCapabilities } from './capabilities';
import { appendEvent, readEvents } from './events';
import { isRecord, type ActionRequest, type EventQuery } from './types';

export async function dispatchEngine(method: string, params: Record<string, unknown> = {}) {
  if (method === 'hello') return hello();
  if (method === 'capabilities') return privilegedCapabilities;
  if (method === 'describe') return describe();
  if (method === 'act') return act(String(params.name || ''), isRecord(params.input) ? params.input : {});
  if (method === 'events') return readEvents(params as EventQuery);
  throw new Error(`unsupported Cymonkey method ${method}`);
}

function hello() {
  return {
    protocolVersion: 'jangolova.cymonkey/v1alpha1',
    implementation: {
      name: 'jangolova-browser-extension-webextension',
      version: browser.runtime.getManifest().version,
    },
    backends: ['webextension'],
    features: [
      'augmentation',
      'browser-extension',
      'events.cursor',
      'scripts.packaged',
      import.meta.env.MODE === 'spoke' ? 'xallet.spoke' : 'standalone',
    ],
  };
}

async function describe() {
  const tab = await activeTab();
  const scripts = await browser.scripting.getRegisteredContentScripts();
  const rules = await browser.declarativeNetRequest.getDynamicRules();
  let page: unknown = null;
  if (tab?.id !== undefined) {
    page = await sendToTab(tab.id, 'describe', {}).catch(() => null);
  }
  return {
    extension: {
      id: browser.runtime.id,
      version: browser.runtime.getManifest().version,
      mode: import.meta.env.MODE === 'spoke' ? 'spoke' : 'standalone',
      browser: import.meta.env.BROWSER,
    },
    activeTab: tab ? { id: tab.id, url: tab.url || null, title: tab.title || null } : null,
    registeredScripts: scripts.map((script) => script.id).sort(),
    dynamicRuleIds: rules.map((rule) => rule.id).sort((left, right) => left - right),
    page,
  };
}

async function act(name: string, input: Record<string, unknown>) {
  if (name === 'script.execute') return executeScripts(input);
  if (name === 'script.register') return registerScripts(input, false);
  if (name === 'script.unregister') return unregisterScripts(input);
  if (name === 'style.insert') return changeStyle(input, false);
  if (name === 'style.remove') return changeStyle(input, true);
  if (name === 'network.rules.install') return installRules(input);
  if (name === 'network.rules.remove') return removeRules(input);
  if (name === 'storage.get') return readStorage(input);
  if (name === 'storage.set') return writeStorage(input);
  if (['dom.query', 'overlay.mount', 'overlay.patch', 'overlay.unmount'].includes(name)) {
    const tab = await targetTab(input.target);
    return sendToTab(requireTabID(tab), 'act', { name, input });
  }
  throw new Error(`unsupported Cymonkey action ${JSON.stringify(name)}`);
}

async function executeScripts(input: Record<string, unknown>) {
  const augmentationId = requireAugmentation(input);
  const files = requireFiles(input.files, augmentationId);
  const targetInput = isRecord(input.target) ? input.target : {};
  const tab = await targetTab(targetInput);
  const tabId = requireTabID(tab);
  const target = Array.isArray(targetInput.frameIds)
    ? { tabId, frameIds: targetInput.frameIds.filter((item): item is number => Number.isInteger(item)) }
    : { tabId, allFrames: Boolean(targetInput.allFrames) };
  const injection = {
    target,
    files,
    world: executionWorld(input.world),
  } as unknown as Parameters<typeof browser.scripting.executeScript>[0];
  const results = await browser.scripting.executeScript(injection);
  await appendEvent('script.executed', { augmentationId, frames: results.length }, tabId);
  return { ok: true, tabId, frames: results.map((item) => item.frameId) };
}

async function registerScripts(input: Record<string, unknown>, update: boolean) {
  const augmentationId = requireAugmentation(input);
  const requestedScripts = isRecord(input.script) ? [input.script] : input.scripts;
  if (!Array.isArray(requestedScripts) || requestedScripts.length === 0) {
    throw new Error('scripts must be a non-empty array');
  }
  const scripts = requestedScripts.map((script) => normalizeRegisteredScript(augmentationId, script));
  if (update) await browser.scripting.updateContentScripts(scripts);
  else await browser.scripting.registerContentScripts(scripts);
  const ids = scripts.map((item) => item.id);
  await appendEvent(update ? 'script.updated' : 'script.registered', { augmentationId, ids });
  return { ok: true, ids };
}

async function unregisterScripts(input: Record<string, unknown>) {
  const augmentationId = requireAugmentation(input);
  const requestedIds = typeof input.id === 'string' ? [input.id] : input.ids;
  if (!Array.isArray(requestedIds) || requestedIds.length === 0) {
    throw new Error('ids must be a non-empty array');
  }
  const ids = requestedIds.map((id) => scriptID(augmentationId, requireString(id, 'script id')));
  await browser.scripting.unregisterContentScripts({ ids });
  await appendEvent('script.unregistered', { augmentationId, ids });
  return { ok: true, ids };
}

async function changeStyle(input: Record<string, unknown>, remove: boolean) {
  const augmentationId = requireAugmentation(input);
  const css = requireString(input.css, 'css');
  const targetInput = isRecord(input.target) ? input.target : {};
  const tab = await targetTab(targetInput);
  const injection = {
    target: { tabId: requireTabID(tab), allFrames: Boolean(targetInput.allFrames) },
    css,
  };
  if (remove) await browser.scripting.removeCSS(injection);
  else await browser.scripting.insertCSS(injection);
  await appendEvent(remove ? 'style.removed' : 'style.inserted', { augmentationId }, injection.target.tabId);
  return { ok: true, tabId: injection.target.tabId };
}

async function installRules(input: Record<string, unknown>) {
  const augmentationId = requireAugmentation(input);
  if (!Array.isArray(input.rules) || input.rules.length === 0) {
    throw new Error('rules must be a non-empty array');
  }
  const rules = input.rules.map((value) => {
    if (!isRecord(value) || !Number.isInteger(value.id) || Number(value.id) <= 0) {
      throw new Error('network rule id must be a positive integer');
    }
    return value;
  });
  const ownership = await ruleOwnership();
  for (const rule of rules) {
    const id = Number(rule.id);
    const owner = ownership[String(id)];
    if (owner && owner !== augmentationId) {
      throw new Error(`network rule ${id} belongs to augmentation ${JSON.stringify(owner)}`);
    }
  }
  const ruleIds = rules.map((rule) => Number(rule.id));
  await browser.declarativeNetRequest.updateDynamicRules({
    removeRuleIds: ruleIds,
    addRules: rules as unknown as Parameters<typeof browser.declarativeNetRequest.updateDynamicRules>[0]['addRules'],
  });
  for (const id of ruleIds) ownership[String(id)] = augmentationId;
  await browser.storage.local.set({ 'cymonkey.ruleOwnership': ownership });
  await appendEvent('network.rules.installed', { augmentationId, ruleIds });
  return { ok: true, ruleIds };
}

async function removeRules(input: Record<string, unknown>) {
  const augmentationId = requireAugmentation(input);
  if (!Array.isArray(input.ruleIds) || input.ruleIds.length === 0 || input.ruleIds.some((id) => !Number.isInteger(id))) {
    throw new Error('ruleIds must be a non-empty array of integers');
  }
  const ruleIds = input.ruleIds as number[];
  const ownership = await ruleOwnership();
  for (const id of ruleIds) {
    if (ownership[String(id)] !== augmentationId) {
      throw new Error(`network rule ${id} is not owned by augmentation ${JSON.stringify(augmentationId)}`);
    }
  }
  await browser.declarativeNetRequest.updateDynamicRules({ removeRuleIds: ruleIds });
  for (const id of ruleIds) delete ownership[String(id)];
  await browser.storage.local.set({ 'cymonkey.ruleOwnership': ownership });
  await appendEvent('network.rules.removed', { augmentationId, ruleIds });
  return { ok: true, ruleIds };
}

async function readStorage(input: Record<string, unknown>) {
  const augmentationId = requireAugmentation(input);
  if (!Array.isArray(input.keys) || input.keys.some((key) => typeof key !== 'string' || !key)) {
    throw new Error('keys must be an array of strings');
  }
  const keys = input.keys as string[];
  const namespaced = keys.map((key) => storageKey(augmentationId, key));
  const stored = await browser.storage.local.get(namespaced);
  return {
    values: Object.fromEntries(keys.map((key) => [key, stored[storageKey(augmentationId, key)] ?? null])),
  };
}

async function writeStorage(input: Record<string, unknown>) {
  const augmentationId = requireAugmentation(input);
  if (!isRecord(input.values)) throw new Error('values must be an object');
  const values = Object.fromEntries(
    Object.entries(input.values).map(([key, value]) => [storageKey(augmentationId, key), value]),
  );
  await browser.storage.local.set(values);
  const keys = Object.keys(input.values).sort();
  await appendEvent('storage.updated', { augmentationId, keys });
  return { ok: true, keys };
}

function normalizeRegisteredScript(augmentationId: string, value: unknown) {
  if (!isRecord(value)) throw new Error('script must be an object');
  const id = requireString(value.id, 'script.id');
  if (!Array.isArray(value.matches) || value.matches.length === 0 || value.matches.some((match) => typeof match !== 'string')) {
    throw new Error('script.matches must be a non-empty array of strings');
  }
  const script = {
    id: scriptID(augmentationId, id),
    matches: value.matches as string[],
    js: requireFiles(value.files, augmentationId),
    runAt: runAt(value.runAt),
    allFrames: Boolean(value.allFrames),
    persistAcrossSessions: value.persistAcrossSessions !== false,
    world: executionWorld(value.world),
    css: Array.isArray(value.css) && value.css.length > 0 ? requireFiles(value.css, augmentationId) : undefined,
    excludeMatches: Array.isArray(value.excludeMatches) ? value.excludeMatches as string[] : undefined,
  };
  return script;
}

async function sendToTab(tabId: number, method: string, params: Record<string, unknown>) {
  return browser.tabs.sendMessage(tabId, { channel: 'jangolova.cymonkey.control', method, params });
}

async function activeTab() {
  const tabs = await browser.tabs.query({ active: true, lastFocusedWindow: true });
  const ownOrigin = browser.runtime.getURL('/');
  const active = tabs.find((tab) => !tab.url?.startsWith(ownOrigin));
  if (active) return active;
  const candidates = await browser.tabs.query({});
  return candidates
    .filter((tab) => !tab.url?.startsWith(ownOrigin))
    .sort((left, right) => Number(right.lastAccessed || 0) - Number(left.lastAccessed || 0))[0] || null;
}

async function targetTab(value: unknown) {
  const target = isRecord(value) ? value : {};
  if (Number.isInteger(target.tabId)) return browser.tabs.get(Number(target.tabId));
  const tab = await activeTab();
  if (!tab?.id) throw new Error('Cymonkey has no active target tab');
  return tab;
}

async function ruleOwnership(): Promise<Record<string, string>> {
  const stored = await browser.storage.local.get('cymonkey.ruleOwnership');
  const value = stored['cymonkey.ruleOwnership'];
  if (!isRecord(value)) return {};
  return Object.fromEntries(Object.entries(value).filter((entry): entry is [string, string] => typeof entry[1] === 'string'));
}

function requireAugmentation(input: Record<string, unknown>) {
  const id = requireString(input.augmentationId, 'augmentationId');
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(id)) {
    throw new Error('augmentationId contains unsupported characters');
  }
  return id;
}

function requireFiles(value: unknown, augmentationId: string) {
  if (!Array.isArray(value) || value.length === 0) throw new Error('files must be a non-empty array');
  const prefix = `augmentations/${augmentationId}/`;
  return value.map((file) => {
    if (typeof file !== 'string' || !file.startsWith(prefix) || file.includes('..') || /[:?#\\]/.test(file)) {
      throw new Error(`extension file must be packaged below ${prefix}`);
    }
    return file;
  });
}

function scriptID(augmentationId: string, id: string) {
  const value = `cm-${augmentationId}-${id}`.replace(/[^A-Za-z0-9_-]/g, '-');
  if (value.length > 128) throw new Error('registered script id is too long');
  return value;
}

function executionWorld(value: unknown): 'ISOLATED' | 'MAIN' {
  if (value === undefined || value === 'ISOLATED') return 'ISOLATED';
  if (value === 'MAIN') return 'MAIN';
  throw new Error('script world must be ISOLATED or MAIN');
}

function runAt(value: unknown): 'document_start' | 'document_end' | 'document_idle' {
  if (value === undefined) return 'document_idle';
  if (value === 'document_start' || value === 'document_end' || value === 'document_idle') return value;
  throw new Error('runAt must be document_start, document_end, or document_idle');
}

function storageKey(augmentationId: string, key: string) {
  return `cymonkey.augmentation.${augmentationId}.${requireString(key, 'storage key')}`;
}

function requireString(value: unknown, name: string) {
  if (typeof value !== 'string' || !value) throw new Error(`${name} is required`);
  return value;
}

function requireTabID(tab: { id?: number }) {
  if (!Number.isInteger(tab.id)) throw new Error('target tab has no id');
  return tab.id as number;
}
