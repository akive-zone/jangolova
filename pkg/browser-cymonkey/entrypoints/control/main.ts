declare global {
  // The CDP worker calls this function inside the extension-origin control page.
  // It remains unavailable to ordinary website scripts.
  // eslint-disable-next-line no-var
  var cymonkeyDispatch: (method: string, params?: Record<string, unknown>) => Promise<unknown>;
}

globalThis.cymonkeyDispatch = (method, params = {}) => {
  return browser.runtime.sendMessage({
    channel: 'jangolova.cymonkey.control',
    method,
    params,
  });
};

document.documentElement.dataset.cymonkeyControlReady = 'true';

export {};
