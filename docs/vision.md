# Vision

## Purpose

Jangolova makes an interactive engine available wherever it needs to run and
wherever its output needs to be seen.

An engine may run as a native process, inside a browser, inside a WebAssembly
runtime, on the current device, or on a remote host. A display may be a local
window, a virtual desktop, a browser canvas, a captured frame stream, or a
remote interactive client. Jangolova owns the session that connects them.

The product promise is:

> Start or attach to an engine, connect it to one or more display systems,
> provide control and input, and manage the complete lifecycle as one session.

## Principles

### Engines and displays are independent

An engine should not need to understand whether its output is viewed through a
local window, VNC, WebRTC, recording, or a remote agent. Display and transport
decisions belong to the session.

### Local and remote are deployment choices

The same session description should work on one device or across several
devices. Placement and transport may change without changing the workload's
intent.

### Adapters expose capabilities

Unity, Unreal Engine, Phaser, Three.js, Babylon.js, Playwright, and Puppeteer
do not share identical APIs. Jangolova will expose common lifecycle and
capability contracts without pretending all engines are interchangeable.

### Safe defaults

Attaching is preferred over replacing existing state. External actions require
explicit intent. Credentials and persistent profiles remain outside session
manifests.

### The prototype remains executable

Architecture is proven through running vertical slices. The existing browser,
Xvfb, VNC, CDP, Playwright, and Puppeteer workflow remains a regression target
while generalized components replace its hard-coded orchestration.

## Initial use cases

1. Launch a browser-hosted Three.js, Babylon.js, or Phaser application on an
   Xvfb surface and view it locally or over VNC.
2. Attach Puppeteer, Playwright, or CDP control to that browser session.
3. Launch a native engine on a local or virtual display and expose its output
   through a connector.
4. Run an engine on a remote Jangolova agent while controlling and viewing it
   from another device.
5. Capture screenshots, frame streams, recordings, logs, and health events
   from a session through consistent APIs.

## Non-goals for the foundation

- Replacing Unity, Unreal Engine, or browser rendering APIs.
- Hiding every engine-specific feature behind a lowest-common-denominator API.
- Building a cloud scheduler before one-host sessions are dependable.
- Storing account credentials inside source-controlled configuration.
