import type { XalletSpookState } from './types';

const hubName = 'Xallet Hub';

export class XalletSpookClient {
  private hubId: string | null = null;
  private timer: ReturnType<typeof setInterval> | undefined;

  constructor(
    private readonly name: string,
    private state: XalletSpookState,
    private readonly uiPath: string,
    private readonly onStatus: (status: XalletSpookState['xalletSpook']) => void = () => undefined,
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

  async updateState(state: XalletSpookState) {
    this.state = state;
    if (!this.hubId) return;
    try {
      await browser.runtime.sendMessage(this.hubId, {
        type: 'UPDATE_SPOKE_STATE',
        payload: state,
      });
    } catch {
      this.hubId = null;
      this.setStatus('unavailable');
    }
  }

  private async discover() {
    try {
      const extensions = await browser.management.getAll();
      const hub = extensions.find((extension) => extension.name === hubName && extension.enabled);
      if (!hub?.id) {
        this.hubId = null;
        this.setStatus('unavailable');
        return;
      }
      if (hub.id === this.hubId) return;
      this.hubId = hub.id;
      this.setStatus('connected');
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
      this.setStatus('unavailable');
    }
  }

  private setStatus(status: XalletSpookState['xalletSpook']) {
    this.state = { ...this.state, xalletSpook: status };
    this.onStatus(status);
  }
}
