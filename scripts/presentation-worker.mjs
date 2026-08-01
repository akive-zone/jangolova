#!/usr/bin/env node
import readline from "node:readline";
import process from "node:process";

let browser;
let page;
let disconnected = false;
const lines = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
let chain = Promise.resolve();
lines.on("line", (line) => { chain = chain.then(() => handleLine(line)).catch((error) => console.error(error?.stack || String(error))); });

async function handleLine(line) {
  let request;
  try {
    request = JSON.parse(line);
    const result = await dispatch(request.method, request.params || {});
    respond({ id: request.id, result });
    if (request.method === "disconnect") setImmediate(() => process.exit(0));
  } catch (error) { respond({ id: request?.id || 0, error: error?.message || String(error) }); }
}

async function dispatch(method, params) {
  if (method === "connect") return connect(params.endpoint, params.source);
  if (method === "disconnect") return disconnect();
  if (method === "health") return { connected: isConnected() && Boolean(page) };
  requireConnection();
  if (method === "hello") return page.evaluate(() => window.jangolova?.hello?.() || null);
  if (method === "capabilities") return page.evaluate(() => window.jangolova?.capabilities?.() || []);
  if (method === "describe") return page.evaluate(() => window.jangolova?.describe?.() || null);
  if (method === "act") {
    if (params.name === "presentation.capture") {
      return { pngBase64: await page.screenshot({ encoding: "base64", fullPage: Boolean(params.input?.fullPage) }) };
    }
    return page.evaluate((request) => window.jangolova?.act?.(request.name, request.input || {}), params);
  }
  if (method === "events") return page.evaluate((query) => window.jangolova?.events?.(query || {}), params);
  throw new Error(`unsupported interaction method ${method}`);
}

async function connect(endpoint, source) {
  if (typeof endpoint !== "string" || endpoint.length === 0) throw new Error("CDP endpoint is required");
  const { default: puppeteer } = await import("puppeteer-core");
  browser = await puppeteer.connect(endpoint.startsWith("ws") ? { browserWSEndpoint: endpoint, protocol: "cdp" } : { browserURL: endpoint, protocol: "cdp" });
  browser.on("disconnected", () => { disconnected = true; });
  const pages = await browser.pages();
  page = pages.at(-1) || await browser.newPage();
  if (typeof source === "string" && source.trim()) await page.goto(source, { waitUntil: "domcontentloaded" });
  const ready = await page.evaluate(() => Boolean(window.jangolova && typeof window.jangolova.act === "function"));
  if (!ready) throw new Error("active page does not expose window.jangolova presentation bridge");
  const capabilities = await page.evaluate(() => (window.jangolova.capabilities?.() || []).map((item) => item.name));
  return { capabilities };
}

async function disconnect() { if (browser?.disconnect) browser.disconnect(); disconnected = true; return { disconnected: true }; }
function isConnected() { return Boolean(browser) && !disconnected && browser.connected; }
function requireConnection() { if (!isConnected() || !page) throw new Error("presentation target is disconnected"); }
function respond(value) { process.stdout.write(`${JSON.stringify(value)}\n`); }
