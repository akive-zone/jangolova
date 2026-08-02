import assert from "node:assert/strict";
import test from "node:test";
import { PACMAN_PROTOCOL_VERSION, PACMAN_RUNTIME_SYMBOL, ThreeJSPacman } from "../pkg/threejs-pacman/dist/index.js";

test("Three.js Pacman exposes only explicitly registered resources", async () => {
  const runtime = new ThreeJSPacman();
  const hidden = { name: "hidden", visible: true };
  const hero = { name: "hero", visible: true, position: vector(), rotation: vector(), scale: vector(1, 1, 1) };
  runtime.register({ id: "object:hero", kind: "object", target: hero, actions: ["object.visibility.set", "object.transform.set"] });
  const description = runtime.describe();
  assert.deepEqual(description.resources.map((resource) => resource.id), ["object:hero"]);
  assert.equal(description.resources.some((resource) => resource.properties?.name === hidden.name), false);
  assert.equal(runtime.hello().protocolVersion, PACMAN_PROTOCOL_VERSION);
});

test("Three.js Pacman enforces target and action allowlists", async () => {
  const runtime = new ThreeJSPacman();
  const hero = { visible: true, position: vector(), rotation: vector(), scale: vector(1, 1, 1) };
  runtime.register({ id: "object:hero", kind: "object", target: hero, actions: ["object.visibility.set"] });
  const accepted = await runtime.dispatch({ id: 1, method: "act", params: { name: "object.visibility.set", targetId: "object:hero", input: { visible: false } } });
  assert.equal(accepted.error, undefined);
  assert.equal(hero.visible, false);
  const denied = await runtime.dispatch({ id: 2, method: "act", params: { name: "object.transform.set", targetId: "object:hero", input: {} } });
  assert.equal(denied.error.code, "action_not_allowlisted");
});

test("Three.js Pacman installs a private symbol runtime", () => {
  const runtime = new ThreeJSPacman();
  const target = {};
  const uninstall = runtime.installGlobal(target);
  assert.equal(target[PACMAN_RUNTIME_SYMBOL], runtime);
  assert.equal(Object.keys(target).length, 0);
  uninstall();
  assert.equal(target[PACMAN_RUNTIME_SYMBOL], undefined);
});

function vector(x = 0, y = 0, z = 0) {
  return { x, y, z, set(nextX, nextY, nextZ) { this.x = nextX; this.y = nextY; this.z = nextZ; } };
}
