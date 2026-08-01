# Native Interaction Engines

Jangolova does not launch native targets. Xallet or the native system launches
Unity, Unreal, or another application and supplies an interaction endpoint or
opaque handle.

## Cooperative bridge

The current native integration uses an authenticated loopback WebSocket and
the `jangolova.bridge/v1alpha1` methods. Jangolova owns the bridge host,
protocol, conformance validation, and Unity package. The target owner injects
the endpoint and short-lived token when it starts the application.

Passing a handle or endpoint never transfers target ownership. Disconnecting
Jangolova closes only its bridge connection and host; process and display
lifecycle remain with the target owner.

The next native adapters will wrap this bridge as provider-visible interaction
engines for Unity and Unreal without adding process launch back to Jangolova.
