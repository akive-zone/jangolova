#!/usr/bin/env node
import { createServer, request as httpRequest } from "node:http";
import { connect as tcpConnect } from "node:net";
import process from "node:process";

const listenPort = Number.parseInt(process.env.AUTHENTICATED_CDP_PORT || "9333", 10);
const upstreamPort = Number.parseInt(process.env.CDP_UPSTREAM_PORT || "9224", 10);
const expectedAuthorization = process.env.CDP_AUTHORIZATION;

if (!expectedAuthorization) throw new Error("CDP_AUTHORIZATION is required");

function authorized(request) {
  return request.headers.authorization === expectedAuthorization;
}

const server = createServer((request, response) => {
  if (!authorized(request)) {
    response.writeHead(401, { "content-type": "text/plain", "www-authenticate": "Bearer" });
    response.end("authorization required\n");
    return;
  }
  const headers = { ...request.headers, host: `127.0.0.1:${upstreamPort}` };
  delete headers.authorization;
  const upstream = httpRequest({
    hostname: "127.0.0.1",
    port: upstreamPort,
    method: request.method,
    path: request.url,
    headers,
  }, (upstreamResponse) => {
    response.writeHead(upstreamResponse.statusCode || 502, upstreamResponse.headers);
    upstreamResponse.pipe(response);
  });
  upstream.on("error", () => {
    if (!response.headersSent) response.writeHead(502);
    response.end();
  });
  request.pipe(upstream);
});

server.on("upgrade", (request, socket, head) => {
  if (!authorized(request)) {
    socket.end("HTTP/1.1 401 Unauthorized\r\nConnection: close\r\n\r\n");
    return;
  }
  const upstream = tcpConnect(upstreamPort, "127.0.0.1", () => {
    const lines = [`${request.method} ${request.url} HTTP/${request.httpVersion}`];
    for (let index = 0; index < request.rawHeaders.length; index += 2) {
      const name = request.rawHeaders[index];
      if (name.toLowerCase() === "authorization" || name.toLowerCase() === "host") continue;
      lines.push(`${name}: ${request.rawHeaders[index + 1]}`);
    }
    lines.push(`Host: 127.0.0.1:${upstreamPort}`, "", "");
    upstream.write(lines.join("\r\n"));
    if (head.length) upstream.write(head);
    socket.pipe(upstream).pipe(socket);
  });
  upstream.on("error", () => socket.destroy());
  socket.on("error", () => upstream.destroy());
});

server.listen(listenPort, "127.0.0.1", () => {
  process.stdout.write(`authenticated CDP relay listening on 127.0.0.1:${listenPort}\n`);
});

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => server.close(() => process.exit(0)));
}
