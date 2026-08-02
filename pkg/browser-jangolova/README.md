# Jangolova Browser Extension

This is the canonical package path for the Jangolova Browser Extension backend.

The implementation is currently shared with `pkg/browser-cymonkey` while the
product naming and migration work are phased in. During migration:

- Prefer `pkg/browser-jangolova` in new references.
- Keep `pkg/browser-cymonkey` working as the compatibility path.
- Keep protocol identifiers stable (`jangolova.cymonkey/*`) so page APIs do not
  break.

Build from this path and delegate to the shared source:

```sh
npm --prefix pkg/browser-jangolova run check
```
