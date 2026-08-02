# Jangolova Browser Extension backend (legacy package path)

This directory is the legacy package path for the Jangolova Browser Extension
backend. Cymonkey itself is the transport-neutral Jangolova subsystem; CDP and
WebDriver BiDi work without this package. A single TypeScript source produces
ordinary standalone extensions and Xallet-compatible spoke builds.

`pkg/browser-jangolova` is the preferred package path.

## Build matrix

| Mode | Chrome | Edge | Firefox |
| --- | --- | --- | --- |
| Standalone | `.output/chrome-mv3` | `.output/edge-mv3` | `.output/firefox-mv3` |
| Xallet spoke | `.output/chrome-mv3-spoke` | `.output/edge-mv3-spoke` | `.output/firefox-mv3-spoke` |

```sh
npm install
npm run compile
npm run build:standalone
npm run build:spoke
```

Load the appropriate output directory as an unpacked extension during
development. `npm run zip`, `npm run zip:firefox`, and `npm run zip:spoke`
create distributable archives.

## Runtime shapes

Every build works without Xallet and exposes the page-safe API at
`window.jangolova.cymonkey`. Privileged calls stay in the extension background
and are available to the caller-owned CDP control page.

The target owner installs the extension; Jangolova only detects and handshakes
with it. Spoke mode adds the `management` permission and discovers an enabled extension
named `Xallet Hub`. It registers itself with `REGISTER_SPOKE`, publishes state
with `UPDATE_SPOKE_STATE`, and accepts external `CYMONKEY_CALL` (or
`JANGOLOVA_EXTENSION_CALL`) messages only from the discovered hub ID. If Xallet
Hub is absent, the extension continues to operate as standalone Cymonkey.

Packaged augmentation code belongs under `public/augmentations/<id>/`. Runtime
script operations accept only those packaged paths; they do not download or
evaluate arbitrary source text.
