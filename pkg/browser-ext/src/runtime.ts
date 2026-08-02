import { privilegedCapabilityNames } from './capabilities';
import { dispatchCymonkey } from './engine';
import { readEvents } from './services/events';
import { callPacman } from './services/pacman';
import { isRecord } from './types';

let xalletSpook: 'discovering' | 'unavailable' | 'connected' = 'discovering';

export function setXalletSpookStatus(status: typeof xalletSpook) {
  xalletSpook = status;
}

export async function dispatchJangolova(method: string, params: Record<string, unknown> = {}) {
  if (method === 'hello') return hello();
  if (method === 'capabilities') return capabilities();
  if (method === 'describe') return describe();
  if (method === 'events') return readEvents(params);
  if (method === 'cymonkey.call') {
    return dispatchCymonkey(String(params.method || ''), isRecord(params.params) ? params.params : {});
  }
  if (method === 'pacman.call') return callPacman(params.request, params.target);
  throw new Error(`unsupported Jangolova extension method ${JSON.stringify(method)}`);
}

function hello() {
  return {
    protocolVersion: 'jangolova.browser-extension/v1alpha1',
    implementation: { name: 'jangolova-browser-extension', version: browser.runtime.getManifest().version },
    subsystems: ['cymonkey', 'pacman'],
    backend: 'webextension',
  };
}

function capabilities() {
  return {
    platform: ['events.read', 'injection.packaged', 'network.rules', 'storage.scoped'],
    cymonkey: privilegedCapabilityNames,
    pacman: ['pacman.call'],
  };
}

async function describe() {
  return {
    product: 'Jangolova Browser Extension',
    extensionId: browser.runtime.id,
    distribution: 'single-build',
    subsystems: { cymonkey: true, pacman: true },
    integrations: { xalletSpook: { status: xalletSpook } },
  };
}
