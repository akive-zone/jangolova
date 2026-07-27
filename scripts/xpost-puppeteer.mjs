#!/usr/bin/env node
import puppeteer from "puppeteer-core";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import process from "node:process";

const composeURL = "https://x.com/compose/post";
const options = parseArgs(process.argv.slice(2));

if (!options.text) {
  usage("missing post text; use --text or pass text as arguments");
}

const profileDir =
  options.profile ||
  process.env.PROFILE_DIR ||
  path.join(os.homedir(), ".local/share/chromium-xpost-profile");
const executablePath =
  options.executable ||
  process.env.PUPPETEER_BROWSER_PATH ||
  process.env.PLAYWRIGHT_BROWSER_PATH ||
  findChromium();

if (!executablePath) {
  usage("could not find chromium, chromium-browser, google-chrome, or google-chrome-stable");
}

if (envFlag("CHROMIUM_CLEAR_STALE_LOCKS")) {
  clearChromiumLocks(profileDir);
}

const launchArgs = [
  "--no-first-run",
  "--no-default-browser-check",
  "--disable-dev-shm-usage",
  "--password-store=basic",
  "--start-maximized",
];
if (typeof process.getuid === "function" && process.getuid() === 0) {
  launchArgs.push("--no-sandbox");
}

const browser = await puppeteer.launch({
  executablePath,
  userDataDir: profileDir,
  headless: envFlag("PUPPETEER_HEADLESS"),
  defaultViewport: null,
  args: launchArgs,
  timeout: options.timeout,
});

try {
  const pages = await browser.pages();
  const page = pages[0] || (await browser.newPage());
  page.setDefaultTimeout(options.timeout);

  await page.goto(options.url, { waitUntil: "domcontentloaded" });

  const composer = page
    .locator(
      [
        'div[data-testid="tweetTextarea_0"]',
        'div[data-testid^="tweetTextarea_"]',
        'div[role="textbox"][contenteditable="true"]',
      ].join(", "),
    )
    .setTimeout(options.timeout);
  await composer.fill(options.text);

  if (options.publish) {
    const postButton = page
      .locator(
        [
          '[data-testid="tweetButton"]',
          '[data-testid="tweetButtonInline"]',
          'button[data-testid="tweetButton"]',
          'button[data-testid="tweetButtonInline"]',
        ].join(", "),
      )
      .setTimeout(options.timeout);
    await postButton.click();
    console.log("post submitted");
  } else {
    console.log("composer filled; pass --publish to click Post");
  }

  if (options.screenshot) {
    await fs.promises.mkdir(path.dirname(options.screenshot), { recursive: true });
    await page.screenshot({ path: options.screenshot, fullPage: true });
    console.log(`screenshot saved to ${options.screenshot}`);
  }
} finally {
  await browser.close();
}

function parseArgs(args) {
  const out = {
    publish: false,
    screenshot: "",
    timeout: durationToMs(process.env.XPOST_TIMEOUT || "45s"),
    url: process.env.XPOST_URL || composeURL,
    text: "",
    profile: "",
    executable: "",
  };
  const rest = [];

  for (let index = 0; index < args.length; index += 1) {
    const arg = args[index];
    if (arg === "--publish") {
      out.publish = true;
    } else if (arg === "--text") {
      out.text = requireValue(args, ++index, arg);
    } else if (arg === "--url") {
      out.url = requireValue(args, ++index, arg);
    } else if (arg === "--screenshot") {
      out.screenshot = requireValue(args, ++index, arg);
    } else if (arg === "--timeout") {
      out.timeout = durationToMs(requireValue(args, ++index, arg));
    } else if (arg === "--profile") {
      out.profile = requireValue(args, ++index, arg);
    } else if (arg === "--executable") {
      out.executable = requireValue(args, ++index, arg);
    } else if (arg === "--help" || arg === "-h") {
      usage(null, 0);
    } else if (arg.startsWith("--")) {
      usage(`unknown option: ${arg}`);
    } else {
      rest.push(arg);
    }
  }

  if (!out.text) {
    out.text = rest.join(" ").trim();
  }
  return out;
}

function requireValue(args, index, flag) {
  if (index >= args.length || args[index].startsWith("--")) {
    usage(`${flag} requires a value`);
  }
  return args[index];
}

function durationToMs(value) {
  const raw = String(value).trim();
  const match = raw.match(/^(\d+(?:\.\d+)?)(ms|s|m)?$/);
  if (!match) {
    usage(`invalid duration: ${value}`);
  }
  const amount = Number(match[1]);
  const unit = match[2] || "ms";
  if (unit === "s") return amount * 1000;
  if (unit === "m") return amount * 60 * 1000;
  return amount;
}

function findChromium() {
  const candidates = [
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
    "/usr/bin/google-chrome",
    "/usr/bin/google-chrome-stable",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Chromium.app/Contents/MacOS/Chromium",
  ];
  return candidates.find((candidate) => fs.existsSync(candidate)) || "";
}

function envFlag(name) {
  return ["1", "true", "yes"].includes(
    String(process.env[name] || "").trim().toLowerCase(),
  );
}

function clearChromiumLocks(directory) {
  for (const name of ["SingletonLock", "SingletonCookie", "SingletonSocket"]) {
    try {
      fs.rmSync(path.join(directory, name), { force: true });
    } catch {
      // Chromium will report a profile lock failure if best-effort cleanup is insufficient.
    }
  }
}

function usage(message, code = 1) {
  if (message) {
    console.error(`xpost-puppeteer: ${message}`);
  }
  console.error(`Usage: scripts/xpost.sh --mode puppeteer [options] [text]

Options:
  --text VALUE          post text; if empty, remaining args are joined
  --url VALUE           page URL to open before filling the composer
  --publish             click the Post button after filling the composer
  --screenshot PATH     optional PNG screenshot path after the flow
  --timeout DURATION    maximum wait time, e.g. 45000ms, 45s, 1m
  --profile PATH        persistent browser profile directory
  --executable PATH     Chromium/Chrome executable path`);
  process.exit(code);
}
