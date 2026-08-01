# Xallet Boundary

Jangolova owns display engines. Xallet owns the display runtime.

The authoritative migration and ownership plan lives in Xallet at
`docs/plans/jangolova-display-runtime-transfer.md`.

Jangolova retains Chromium, WebKit, Gecko, SpiderMonkey, Unity, Unreal, native
process, and web-project engine lifecycle. CDP, VNC, surfaces, input,
observation, capture, policy, display sessions, and external-agent interfaces
move to Xallet.

The integration is a versioned engine-provider protocol: Xallet supplies a
surface environment or caller-owned opaque handle with an engine launch
request, and Jangolova returns an engine instance plus typed control endpoints
and cursor-addressed lifecycle events.

Xallet now implements native-host, managed host-X11, and managed OCI-X11
surfaces. The OCI surface is run by Xallet's Docker, Podman, or Apple Container
control plane; Jangolova receives only its resolved `DISPLAY` environment.
