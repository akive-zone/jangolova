import { privilegedCapabilityNames } from '../src/capabilities';
import { dispatchCymonkey } from '../src/engine';
import { dispatchJangolova } from '../src/runtime';
import { publishCymonkeyEvent } from '../src/services/events';
import { isExtensionControlCall } from '../src/services/policy';
import { errorMessage, isRecord, type XalletSpokeState } from '../src/types';
import { XalletSpokeClient } from '../src/xallet-spoke';

export default defineBackground(() => {
  let state: XalletSpokeState = {
    status: 'ready',
    mode: import.meta.env.MODE === 'spoke' ? 'spoke' : 'standalone',
    browser: import.meta.env.BROWSER,
    capabilities: privilegedCapabilityNames,
    extensionId: browser.runtime.id,
  };
  const spoke = import.meta.env.MODE === 'spoke'
    ? new XalletSpokeClient('Jangolova Browser Extension', state, 'popup.html')
    : null;
  spoke?.start();

  browser.runtime.onMessage.addListener((message, sender) => {
    return handleMessage(message, sender.tab?.id);
  });

  browser.runtime.onMessageExternal?.addListener((message, sender) => {
    if (!isExtensionControlCall(message)) return undefined;
    if (!spoke?.acceptsExternalSender(sender.id)) {
      return Promise.resolve({ ok: false, error: 'external caller is not the registered Xallet Hub' });
    }
    return handleControlCall(message);
  });

  async function handleMessage(message: unknown, senderTabId?: number) {
    if (!isRecord(message)) return undefined;
    if (message.channel === 'jangolova.cymonkey.event') {
      const event = isRecord(message.event) ? message.event : {};
      return publishCymonkeyEvent(
        typeof event.type === 'string' ? event.type : 'cymonkey.event',
        isRecord(event.data) ? event.data : {},
        senderTabId,
      );
    }
    if (message.channel === 'jangolova.cymonkey.control') {
      return dispatchCymonkey(String(message.method || ''), isRecord(message.params) ? message.params : {});
    }
    if (message.channel === 'jangolova.extension.control') {
      return dispatchJangolova(String(message.method || ''), isRecord(message.params) ? message.params : {});
    }
    if (message.type === 'GET_SPOKE_STATE') return state;
    if (isExtensionControlCall(message)) return handleControlCall(message);
    return undefined;
  }

  async function handleControlCall(message: Record<string, unknown>) {
    const method = String(message.method || '');
    const params = isRecord(message.params) ? message.params : {};
    state = { ...state, status: 'running', lastAction: method, lastError: undefined };
    await spoke?.updateState(state);
    try {
      const result = message.type === 'CYMONKEY_CALL'
        ? await dispatchCymonkey(method, params)
        : await dispatchJangolova(method, params);
      state = { ...state, status: 'ready', lastAction: method, lastError: undefined };
      await spoke?.updateState(state);
      return { ok: true, result };
    } catch (error) {
      state = { ...state, status: 'failed', lastAction: method, lastError: errorMessage(error) };
      await spoke?.updateState(state);
      return { ok: false, error: state.lastError };
    }
  }
});
