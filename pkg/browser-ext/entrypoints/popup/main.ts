const statusElement = requireElement('status');

void Promise.all([
  browser.runtime.sendMessage({ channel: 'jangolova.cymonkey.control', method: 'describe', params: {} }),
  browser.runtime.sendMessage({ channel: 'jangolova.extension.control', method: 'describe', params: {} }),
]).then(([cymonkeyValue, extensionValue]) => {
  const cymonkey = cymonkeyValue as {
    extension?: { browser?: string };
    registeredScripts?: string[];
    dynamicRuleIds?: number[];
  };
  const extension = extensionValue as {
    distribution?: string;
    integrations?: { xalletSpook?: { status?: string } };
  };
  statusElement.textContent = 'Ready';
  requireElement('distribution').textContent = extension.distribution || 'single-build';
  requireElement('spook').textContent = extension.integrations?.xalletSpook?.status || 'unavailable';
  requireElement('browser').textContent = cymonkey.extension?.browser || 'unknown';
  requireElement('scripts').textContent = String(cymonkey.registeredScripts?.length || 0);
  requireElement('rules').textContent = String(cymonkey.dynamicRuleIds?.length || 0);
}).catch((error) => {
  statusElement.textContent = error instanceof Error ? error.message : String(error);
});

function requireElement(id: string) {
  const element = document.getElementById(id);
  if (!element) throw new Error(`missing popup element ${id}`);
  return element;
}
