const statusElement = requireElement('status');

void browser.runtime.sendMessage({
  channel: 'jangolova.cymonkey.control',
  method: 'describe',
  params: {},
}).then((value) => {
  const description = value as {
    extension?: { mode?: string; browser?: string };
    registeredScripts?: string[];
    dynamicRuleIds?: number[];
  };
  statusElement.textContent = 'Ready';
  requireElement('mode').textContent = description.extension?.mode || 'standalone';
  requireElement('browser').textContent = description.extension?.browser || 'unknown';
  requireElement('scripts').textContent = String(description.registeredScripts?.length || 0);
  requireElement('rules').textContent = String(description.dynamicRuleIds?.length || 0);
}).catch((error) => {
  statusElement.textContent = error instanceof Error ? error.message : String(error);
});

function requireElement(id: string) {
  const element = document.getElementById(id);
  if (!element) throw new Error(`missing popup element ${id}`);
  return element;
}
