import { isRecord } from '../types';
import { publishCymonkeyEvent } from './events';

export async function readScopedStorage(namespace: string, input: Record<string, unknown>) {
  const ownerId = requireOwner(input);
  if (!Array.isArray(input.keys) || input.keys.some((key) => typeof key !== 'string' || !key)) {
    throw new Error('keys must be an array of strings');
  }
  const keys = input.keys as string[];
  const namespaced = keys.map((key) => storageKey(namespace, ownerId, key));
  const stored = await browser.storage.local.get(namespaced);
  return { values: Object.fromEntries(keys.map((key) => [key, stored[storageKey(namespace, ownerId, key)] ?? null])) };
}

export async function writeScopedStorage(namespace: string, input: Record<string, unknown>) {
  const ownerId = requireOwner(input);
  if (!isRecord(input.values)) throw new Error('values must be an object');
  const values = Object.fromEntries(
    Object.entries(input.values).map(([key, value]) => [storageKey(namespace, ownerId, key), value]),
  );
  await browser.storage.local.set(values);
  const keys = Object.keys(input.values).sort();
  if (namespace === 'cymonkey') await publishCymonkeyEvent('storage.updated', { augmentationId: ownerId, keys });
  return { ok: true, keys };
}

export async function readPlatformRecord(key: string): Promise<Record<string, string>> {
  const stored = await browser.storage.local.get(key);
  const value = stored[key];
  if (!isRecord(value)) return {};
  return Object.fromEntries(Object.entries(value).filter((entry): entry is [string, string] => typeof entry[1] === 'string'));
}

export async function writePlatformRecord(key: string, value: Record<string, string>) {
  await browser.storage.local.set({ [key]: value });
}

function requireOwner(input: Record<string, unknown>) {
  const value = input.augmentationId ?? input.runtimeId;
  if (typeof value !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(value)) {
    throw new Error('scoped storage owner id is required');
  }
  return value;
}

function storageKey(namespace: string, ownerId: string, key: string) {
  if (!key) throw new Error('storage key is required');
  return `jangolova.${namespace}.${ownerId}.${key}`;
}
