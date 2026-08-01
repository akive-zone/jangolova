#!/usr/bin/env node
import readline from "node:readline";
import process from "node:process";

const lines = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
let reconnectAttempts = 0;
lines.on("line", (line) => {
  const request = JSON.parse(line);
  if (request.method === "connect") {
    if (request.params?.endpoint !== "wss://browser.remote.example/devtools/browser/42") {
      respond(request.id, undefined, "unexpected remote CDP endpoint");
      return;
    }
    if (request.params?.headers?.Authorization !== "Bearer fixture-secret") {
      respond(request.id, undefined, "authenticated CDP header was not supplied");
      return;
    }
    respond(request.id, { capabilities: ["fixture.connected"] });
    return;
  }
  if (request.method === "reconnect") {
    if (request.params?.headers?.Authorization !== "Bearer rotated-secret") {
      respond(request.id, undefined, "rotated authenticated CDP header was not supplied");
      return;
    }
    reconnectAttempts += 1;
    if (reconnectAttempts === 1) {
      respond(request.id, undefined, "temporary rejection of Bearer rotated-secret");
      return;
    }
    respond(request.id, { reconnected: true });
    return;
  }
  if (request.method === "disconnect") {
    respond(request.id, { disconnected: true });
    setImmediate(() => process.exit(0));
    return;
  }
  if (request.method === "health") {
    respond(request.id, { connected: true });
    return;
  }
  respond(request.id, undefined, `unsupported fixture method ${request.method}`);
});

function respond(id, result, error) {
  process.stdout.write(`${JSON.stringify({ id, ...(error ? { error } : { result }) })}\n`);
}
