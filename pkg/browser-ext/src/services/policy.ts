import { isRecord } from '../types';

export function isExtensionControlCall(message: unknown): message is Record<string, unknown> {
  return isRecord(message) && (message.type === 'JANGOLOVA_EXTENSION_CALL' || message.type === 'CYMONKEY_CALL');
}

export function requireScopedIdentifier(value: unknown, name: string) {
  if (typeof value !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(value)) {
    throw new Error(`${name} contains unsupported characters`);
  }
  return value;
}
