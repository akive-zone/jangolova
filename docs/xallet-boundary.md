# Xallet Boundary

Jangolova owns interaction and presentation engines. Xallet owns target
runtimes and display infrastructure.

Jangolova retains Playwright, Puppeteer, Three.js, Unity and Unreal plugins,
their semantic interaction protocol, and adapter-local workers. Xallet owns
Chromium/WebKit/Gecko processes, browser-driver processes, native applications,
surfaces, VNC, CDP/BiDi/WebDriver exposure, OCI placement, networking, secrets,
policy, and target-session lifecycle.

For Safari MCP, Xallet also owns `safaridriver --mcp` and the private
stdio-to-Streamable-HTTP relay. Jangolova owns only the MCP client and semantic
capability mapping.

The integration is target-in:

1. Xallet creates and starts a target.
2. Xallet resolves a private endpoint or native handle.
3. Xallet asks Jangolova to connect an interaction engine to that target.
4. Agents call Jangolova's capabilities through the authenticated provider.
5. Disconnecting Jangolova does not terminate the Xallet-owned target.

The same sequence works without Xallet when a native system supplies the
target coordinates directly.

The exact browser target shapes, ownership invariants, cleanup sequence, and
Xallet implementation checklist are in
[Browser target protocols](browser-target-protocols.md).
