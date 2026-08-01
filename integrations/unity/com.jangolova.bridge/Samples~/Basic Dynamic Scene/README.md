# Basic Dynamic Scene

Add `JangolovaSampleBootstrap` to an empty GameObject. It creates a camera and
light when needed and attaches the bridge. Build the project as a desktop
player, then configure a Jangolova `native-process` engine to launch it with
`bridge.enabled` set to `true`. The display-runtime caller owns the control
side of the bridge.
