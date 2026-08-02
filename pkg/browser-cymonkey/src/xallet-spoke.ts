import type { XalletSpokeState } from './types';

const hubName = 'Xallet Hub';

export class XalletSpokeClient {
  private hubId: string | null = null;
  private timer: ReturnType<typeof setInterval> | undefined;

  constructor(
    private readonly name: string,
    private state: XalletSpokeState,
    private readonly uiPath: string,
  ) {}

  start() {
    void this.discover();
    this.timer = setInterval(() => void this.discover(), 10_000);
  }

  stop() {
    if (this.timer) clearInterval(this.timer);
    this.timer = undefined;
  }

  acceptsExternalSender(senderId: string | undefined) {
    return Boolean(senderId && this.hubId && senderId === this.hubId);
  }

  async updateState(state: XalletSpokeState) {
    this.state = state;
    if (!this.hubId) return;
    try {
      await browser.runtime.sendMessage(this.hubId, {
        type: 'UPDATE_SPOKE_STATE',
        payload: state,
      });
    } catch {
      this.hubId = null;
    }
  }

  private async discover() {
    try {
      const extensions = await browser.management.getAll();
      const hub = extensions.find((extension) => extension.name === hubName && extension.enabled);
      if (!hub?.id) {
        this.hubId = null;
        return;
      }
      if (hub.id === this.hubId) return;
      this.hubId = hub.id;
      await browser.runtime.sendMessage(hub.id, {
        type: 'REGISTER_SPOKE',
        payload: {
          name: this.name,
          initialState: this.state,
          uiPath: this.uiPath,
        },
      });
    } catch {
      this.hubId = null;
    }
  }
}
