import { isRecord } from '../types';
import { publishCymonkeyEvent } from './events';
import { requireTabID, targetTab } from './tabs';

export async function executePackagedScripts(augmentationId: string, input: Record<string, unknown>) {
  const files = requirePackagedFiles(input.files, augmentationId);
  const targetInput = isRecord(input.target) ? input.target : {};
  const tabId = requireTabID(await targetTab(targetInput));
  const target = Array.isArray(targetInput.frameIds)
    ? { tabId, frameIds: targetInput.frameIds.filter((item): item is number => Number.isInteger(item)) }
    : { tabId, allFrames: Boolean(targetInput.allFrames) };
  const results = await browser.scripting.executeScript({
    target,
    files,
    world: executionWorld(input.world),
  } as unknown as Parameters<typeof browser.scripting.executeScript>[0]);
  await publishCymonkeyEvent('script.executed', { augmentationId, frames: results.length }, tabId);
  return { ok: true, tabId, frames: results.map((item) => item.frameId) };
}

export async function registerPackagedScripts(augmentationId: string, input: Record<string, unknown>, update = false) {
  const requested = isRecord(input.script) ? [input.script] : input.scripts;
  if (!Array.isArray(requested) || requested.length === 0) throw new Error('scripts must be a non-empty array');
  const scripts = requested.map((value) => normalizeScript(augmentationId, value));
  if (update) await browser.scripting.updateContentScripts(scripts);
  else await browser.scripting.registerContentScripts(scripts);
  const ids = scripts.map((item) => item.id);
  await publishCymonkeyEvent(update ? 'script.updated' : 'script.registered', { augmentationId, ids });
  return { ok: true, ids };
}

export async function unregisterPackagedScripts(augmentationId: string, input: Record<string, unknown>) {
  const requested = typeof input.id === 'string' ? [input.id] : input.ids;
  if (!Array.isArray(requested) || requested.length === 0) throw new Error('ids must be a non-empty array');
  const ids = requested.map((id) => scriptID(augmentationId, requireString(id, 'script id')));
  await browser.scripting.unregisterContentScripts({ ids });
  await publishCymonkeyEvent('script.unregistered', { augmentationId, ids });
  return { ok: true, ids };
}

export async function changeStyle(augmentationId: string, input: Record<string, unknown>, remove: boolean) {
  const css = requireString(input.css, 'css');
  const targetInput = isRecord(input.target) ? input.target : {};
  const tabId = requireTabID(await targetTab(targetInput));
  const injection = { target: { tabId, allFrames: Boolean(targetInput.allFrames) }, css };
  if (remove) await browser.scripting.removeCSS(injection); else await browser.scripting.insertCSS(injection);
  await publishCymonkeyEvent(remove ? 'style.removed' : 'style.inserted', { augmentationId }, tabId);
  return { ok: true, tabId };
}

function normalizeScript(augmentationId: string, value: unknown) {
  if (!isRecord(value)) throw new Error('script must be an object');
  if (!Array.isArray(value.matches) || value.matches.length === 0 || value.matches.some((match) => typeof match !== 'string')) {
    throw new Error('script.matches must be a non-empty array of strings');
  }
  return {
    id: scriptID(augmentationId, requireString(value.id, 'script.id')),
    matches: value.matches as string[],
    js: requirePackagedFiles(value.files, augmentationId),
    runAt: runAt(value.runAt),
    allFrames: Boolean(value.allFrames),
    persistAcrossSessions: value.persistAcrossSessions !== false,
    world: executionWorld(value.world),
    css: Array.isArray(value.css) && value.css.length ? requirePackagedFiles(value.css, augmentationId) : undefined,
    excludeMatches: Array.isArray(value.excludeMatches) ? value.excludeMatches as string[] : undefined,
  };
}

function requirePackagedFiles(value: unknown, augmentationId: string) {
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
  throw new Error('invalid runAt');
}

function requireString(value: unknown, name: string) {
  if (typeof value !== 'string' || !value) throw new Error(`${name} is required`);
  return value;
}
