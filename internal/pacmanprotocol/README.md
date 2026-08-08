# Pacman v1 Protocol Bindings

Generated Go bindings for the `jangolova.pacman/v1` protocol.

## Regeneration

```bash
npm run generate:pacman-protocol   # regenerate from schema
npm run check:pacman-protocol      # verify generated files are up to date
```

Source schema: `protocol/pacman/v1/protocol.schema.json`

## Contents

- Protocol version constant: `ProtocolVersion`
- Typed enums: `ResourceKind`, `Effect`
- Message structs: `Hello`, `Capability`, `Resource`, `Description`, `Health`, `ActionRequest`, `Event`, `EventBatch`
- `Client` / `Transport` interface for typed dispatch

These bindings are consumed by the Pacman adapter and any external client
that needs version-pinned, schema-derived types without importing the full
adapter package.
