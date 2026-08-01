# Vision

Jangolova gives agents one semantic way to interact with existing visual and
interactive systems, regardless of where those systems run.

> Given a caller-owned target, attach the appropriate interaction engine and
> expose what the agent can observe, do, and present.

The interaction engine may be Playwright, Puppeteer, a Three.js experience, a
Unity package, an Unreal plugin, or another integration. The target may be
Chromium, a native player, a physical machine, a VM, or an OCI workload.

Jangolova deliberately does not provision targets. Xallet can create the
browser, display, application, network, and credentials, then hand Jangolova a
private endpoint. Standalone users can supply the same endpoint directly.

The durable abstraction is semantic interaction—`capabilities`, `describe`,
`act`, and `events`—not a specific browser, display server, or container tool.
