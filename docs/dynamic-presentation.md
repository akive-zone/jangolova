# Dynamic Presentation

Jangolova owns presentation integrations as well as imperative interaction.
Created presentations are not passive output: they expose semantic state,
actions, and events so agents can subsequently operate the interfaces they
created.

## Web and Three.js

The repository's Three.js experience demonstrates semantic scene description,
object and camera actions, pointer events, and dynamic rendering. A browser
target is still supplied by Xallet or the native system. Jangolova uses its
Playwright/Puppeteer interaction engine to navigate and operate that target;
it does not launch the browser.

The current `connect-engine --source URL` option navigates an attached browser
to a caller-reachable presentation URL. Packaging and serving reusable
Jangolova presentation bundles through the provider is the next web-specific
adapter slice.

## Unity and Unreal

The Unity package implements the common cooperative bridge in a caller-owned
player. Jangolova owns the package and semantic operations; Xallet or the
native system owns the player process, display, and placement. Unreal should
follow the same boundary with an engine-specific plugin.

Presentation capabilities are descriptive, not authorization. The calling
agent system decides which write or externally-effectful actions are allowed.

Presentation artifacts and behavior belong to Jangolova. Serving, player
launch, graphics/display allocation, and process lifecycle belong to Xallet or
another target provider.
