import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const fixtureRoot = new URL("./unity-pacman-fixture/", import.meta.url);
const manifest = JSON.parse(await readFile(new URL("Packages/manifest.json", fixtureRoot)));
const projectVersion = await readFile(new URL("ProjectSettings/ProjectVersion.txt", fixtureRoot), "utf8");
const assembly = JSON.parse(await readFile(new URL("Assets/Editor/Jangolova.PacmanFixture.Editor.asmdef", fixtureRoot)));
const fixture = await readFile(new URL("Assets/Editor/HeadlessPacmanFixture.cs", fixtureRoot), "utf8");
const container = await readFile(new URL("../deploy/unity-pacman-fixture/Containerfile", import.meta.url), "utf8");
const runner = await readFile(new URL("../deploy/unity-pacman-fixture/run-fixture.sh", import.meta.url), "utf8");

assert.equal(manifest.dependencies["com.jangolova.pacman"], "file:../../../pkg/unity-pacman");
assert.match(projectVersion, /m_EditorVersion: 2022\.3\./);
assert.equal(assembly.name, "Jangolova.PacmanFixture.Editor");
assert.ok(assembly.references.includes("Jangolova.Pacman"));
assert.ok(assembly.references.includes("Unity.Newtonsoft.Json"));
for (const method of ["hello", "capabilities", "describe", "act", "events", "health"]) {
  assert.ok(fixture.includes(`Dispatch("${method}"`));
}
assert.match(fixture, /object:fixture/);
assert.match(fixture, /object\.active\.set/);
assert.match(fixture, /event:resource-changed/);
assert.match(fixture, /transport\.Disposed/);
assert.match(fixture, /disconnect destroyed the target/);
assert.doesNotMatch(fixture, /FindObjectsOfType|Resources\.FindObjects|Application\.Quit|Process\.Kill/);
assert.match(container, /ARG UNITY_EDITOR_IMAGE/);
assert.match(container, /ARG UNITY_CONTAINER_USER=root/);
assert.match(container, /FROM \$\{UNITY_EDITOR_IMAGE\}/);
assert.doesNotMatch(container, /jangolova\/engine-runtime/);
for (const argument of ["-batchmode", "-nographics", "-quit", "-executeMethod"]) {
  assert.ok(runner.includes(argument));
}
assert.match(runner, /HeadlessPacmanFixture\.Run/);
assert.doesNotMatch(`${container}\n${runner}`, /UNITY_(?:EMAIL|PASSWORD|SERIAL|LICENSE)\s*=/);
assert.doesNotMatch(`${container}\n${runner}`, /Xvfb|DISPLAY=/);
console.log("Unity Pacman headless fixture environment contract is valid.");
