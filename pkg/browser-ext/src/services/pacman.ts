import { publishPacmanEvent } from './events';
import { requireTabID, targetTab } from './tabs';

export async function callPacman(request: unknown, target: unknown = {}) {
  const tabId = requireTabID(await targetTab(target));
  const results = await browser.scripting.executeScript({
    target: { tabId },
    world: 'MAIN',
    func: async (pacmanRequest: unknown) => {
      const symbol = Symbol.for('jangolova.pacman.runtime');
      const runtime = (globalThis as Record<PropertyKey, unknown>)[symbol] as { dispatch?(request: unknown): unknown } | undefined;
      if (!runtime?.dispatch) throw new Error('no explicitly installed Three.js Pacman runtime was found');
      return runtime.dispatch(pacmanRequest);
    },
    args: [request],
  } as unknown as Parameters<typeof browser.scripting.executeScript>[0]);
  const value = results[0]?.result;
  await publishPacmanEvent('request.completed', { tabId }, tabId);
  return value;
}
