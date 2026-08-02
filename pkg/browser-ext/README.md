# Jangolova Browser Extension

This is the canonical WXT implementation of Jangolova's browser runtime. It
contains shared extension platform services plus two semantic subsystems:

- **Cymonkey** installs and manages augmentations, DOM operations, styles, and overlays.
- **Pacman** transports `jangolova.pacman/v1alpha1` calls to an explicitly installed
  browser presentation runtime such as `@jangolova/threejs-pacman`.

Jangolova owns extension authentication, policy, packaged script injection,
namespaced storage, declarative network rules, and the shared cursor event log.
Neither subsystem exposes raw `chrome.*` or `browser.*` APIs to the page.

Build the single artifact for each browser with:

```sh
npm install
npm run check
```

The page-safe `window.jangolova.cymonkey` API remains compatible. Every build
works standalone and also carries Xallet Spook support. When an enabled
`Xallet Hub` is discovered, the extension registers with it and accepts
privileged external calls only from that discovered hub ID. No separate Spook
artifact or installation exists.
