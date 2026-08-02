import { privilegedCapabilityNames } from '../src/capabilities';
import { dispatchEngine } from '../src/engine';
import { appendEvent } from '../src/events';
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
    if (!isRecord(message) || !isExtensionCallMessage(message)) return undefined;
    if (!spoke?.acceptsExternalSender(sender.id)) {
      return Promise.resolve({ ok: false, error: 'external caller is not the registered Xallet Hub' });
    }
    return handleCymonkeyCall(message);
  });

  async function handleMessage(message: unknown, senderTabId?: number) {
    if (!isRecord(message)) return undefined;
    if (message.channel === 'jangolova.cymonkey.event') {
      const event = isRecord(message.event) ? message.event : {};
      return appendEvent(
        typeof event.type === 'string' ? event.type : 'cymonkey.event',
        isRecord(event.data) ? event.data : {},
        senderTabId,
      );
    }
    if (message.channel === 'jangolova.cymonkey.control') {
      return dispatchEngine(String(message.method || ''), isRecord(message.params) ? message.params : {});
    }
    if (message.type === 'GET_SPOKE_STATE') return state;
    if (message.type === 'CYMONKEY_CALL') return handleCymonkeyCall(message);
    if (message.type === 'JANGOLOVA_EXTENSION_CALL') return handleCymonkeyCall(message);
    return undefined;
  }

  function isExtensionCallMessage(message: Record<string, unknown>) {
    return message.type === 'CYMONKEY_CALL' || message.type === 'JANGOLOVA_EXTENSION_CALL';
  }

  async function handleCymonkeyCall(message: Record<string, unknown>) {
    const method = String(message.method || '');
    const params = isRecord(message.params) ? message.params : {};
    state = { ...state, status: 'running', lastAction: method, lastError: undefined };
    await spoke?.updateState(state);
    try {
      const result = await dispatchEngine(method, params);
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
