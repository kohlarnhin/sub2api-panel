"use strict";

const vm = require("vm");
const path = require("path");
const { runFromInputs } = require(path.join(__dirname, "sdkvm.js"));

const DEFAULT_USER_AGENT =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
  "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36";
const DEFAULT_PAGE_URL = "https://auth.openai.com/about-you";
const DEFAULT_FLOW = "oauth_create_account";
const SENTINEL_BOOTSTRAP_URL = "https://sentinel.openai.com/backend-api/sentinel/sdk.js";

async function readJsonFromStdin() {
  let raw = "";
  for await (const chunk of process.stdin) {
    raw += chunk;
  }
  return raw.trim() ? JSON.parse(raw) : {};
}

async function fetchText(url) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`fetch ${url} failed: ${response.status}`);
  }
  return response.text();
}

function makeStorage() {
  const store = new Map();
  return {
    length: 0,
    getItem(key) {
      return store.has(String(key)) ? store.get(String(key)) : null;
    },
    setItem(key, value) {
      store.set(String(key), String(value));
      this.length = store.size;
    },
    removeItem(key) {
      store.delete(String(key));
      this.length = store.size;
    },
    clear() {
      store.clear();
      this.length = 0;
    },
    key(idx) {
      return [...store.keys()][idx] ?? null;
    },
  };
}

function makeElement(tag) {
  return {
    tagName: String(tag).toUpperCase(),
    style: {},
    children: [],
    hidden: false,
    visibility: "visible",
    ariaHidden: "false",
    innerText: "",
    textContent: "",
    appendChild(child) {
      this.children.push(child);
      return child;
    },
    removeChild(child) {
      this.children = this.children.filter((item) => item !== child);
    },
    getBoundingClientRect() {
      return { x: 0, y: 0, top: 0, left: 0, right: 84, bottom: 16, width: 84, height: 16 };
    },
    addEventListener() {},
    removeEventListener() {},
    contentWindow: { postMessage() {} },
  };
}

function createTurnstileRuntimeEnvironment(input) {
  const userAgent = input.user_agent || DEFAULT_USER_AGENT;
  const body = makeElement("body");
  body.clientWidth = 1280;
  body.clientHeight = 720;

  const documentRef = {
    body,
    head: makeElement("head"),
    documentElement: Object.assign(makeElement("html"), {
      clientWidth: 1280,
      clientHeight: 720,
    }),
    referrer: "",
    cookie: "",
    visibilityState: "visible",
    scripts: [],
    createElement(tag) {
      return makeElement(tag);
    },
    addEventListener() {},
    removeEventListener() {},
  };

  const windowRef = {
    Reflect,
    Object,
    Math,
    Date,
    JSON,
    document: documentRef,
    history: {
      length: 2,
      state: null,
      pushState() {},
      replaceState() {},
    },
    navigator: {
      userAgent,
      language: "zh-CN",
      languages: ["zh-CN", "zh", "en-US", "en"],
      platform: "Win32",
      vendor: "Google Inc.",
      deviceMemory: 8,
      maxTouchPoints: 0,
      webdriver: false,
    },
    screen: {
      width: 1920,
      height: 1080,
      availWidth: 1920,
      availHeight: 1040,
      availLeft: 0,
      availTop: 0,
      colorDepth: 24,
      pixelDepth: 24,
    },
    performance: {
      now() {
        return performance.now();
      },
    },
    localStorage: makeStorage(),
    __reactRouterContext: {
      state: {
        loaderData: {
          "routes/layouts/client-auth-session-layout/layout": {
            session: {
              session_id: "",
              auth_session_logging_id: "",
              openai_client_id: "",
              app_name_enum: "",
              promo: "",
              signup_source: "",
              country_code_hint: "SG",
              is_missing_session: false,
            },
            seedCacheEntry: null,
          },
        },
        actionData: null,
        errors: null,
      },
    },
    setTimeout,
    clearTimeout,
  };

  return { windowRef, documentRef };
}

async function loadSentinelSdkInternals(input) {
  const bootstrap = await fetchText(SENTINEL_BOOTSTRAP_URL);
  const sdkUrlMatch = bootstrap.match(/src = '([^']+sdk\.js)'/);
  if (!sdkUrlMatch) {
    throw new Error("未解析到 Sentinel SDK 地址");
  }

  let sdkSource = await fetchText(sdkUrlMatch[1]);
  const exposeAnchor = "t.init=we,t.sessionObserverToken=";
  if (!sdkSource.includes(exposeAnchor)) {
    throw new Error("Sentinel SDK 结构已变化，未找到暴露锚点");
  }
  sdkSource = sdkSource.replace(
    exposeAnchor,
    "globalThis.__codexSentinel={P,ce,_n,Nt,D};" + exposeAnchor,
  );

  const userAgent = input.user_agent || DEFAULT_USER_AGENT;
  const sdkUrl = sdkUrlMatch[1];

  const document = {
    currentScript: { src: sdkUrl },
    head: { appendChild() {} },
    body: { appendChild() {}, clientWidth: 1280, clientHeight: 720 },
    documentElement: {
      getAttribute(attr) {
        if (attr === "data-build") return null;
        return null;
      },
      clientWidth: 1280,
      clientHeight: 720,
    },
    scripts: [{ src: sdkUrl }],
    cookie: input.cookie_header || `oai-did=${encodeURIComponent(input.did || "")}`,
    referrer: "",
    visibilityState: "visible",
    createElement(tag) {
      return makeElement(tag);
    },
    addEventListener() {},
    removeEventListener() {},
  };

  const navigatorProto = {
    getBattery() { return Promise.resolve({ charging: true, chargingTime: 0, dischargingTime: Infinity, level: 1 }); },
    getGamepads() { return []; },
    javaEnabled() { return false; },
    sendBeacon() { return true; },
    vibrate() { return true; },
  };
  const navigator = Object.create(navigatorProto, {
    userAgent: { value: userAgent, enumerable: true },
    language: { value: "zh-CN", enumerable: true },
    languages: { value: ["zh-CN", "zh", "en-US", "en"], enumerable: true },
    hardwareConcurrency: { value: 8, enumerable: true },
    platform: { value: "Win32", enumerable: true },
    vendor: { value: "Google Inc.", enumerable: true },
    deviceMemory: { value: 8, enumerable: true },
    maxTouchPoints: { value: 0, enumerable: true },
    webdriver: { value: false, enumerable: true },
    cookieEnabled: { value: true, enumerable: true },
    onLine: { value: true, enumerable: true },
    pdfViewerEnabled: { value: true, enumerable: true },
    product: { value: "Gecko", enumerable: true },
    productSub: { value: "20030107", enumerable: true },
    appCodeName: { value: "Mozilla", enumerable: true },
    appName: { value: "Netscape", enumerable: true },
    appVersion: { value: userAgent.replace("Mozilla/", ""), enumerable: true },
  });

  const context = {
    console,
    setTimeout,
    clearTimeout,
    setInterval,
    clearInterval,
    TextEncoder,
    TextDecoder,
    crypto,
    Reflect,
    performance: {
      now: () => performance.now(),
      timeOrigin: performance.timeOrigin || Date.now() - performance.now(),
      memory: {
        jsHeapSizeLimit: 2172649472,
        totalJSHeapSize: 35000000 + Math.floor(Math.random() * 5000000),
        usedJSHeapSize: 25000000 + Math.floor(Math.random() * 5000000),
      },
    },
    fetch,
    URL,
    URLSearchParams,
    Array,
    Object,
    Map,
    Set,
    WeakMap,
    WeakSet,
    Promise,
    Proxy,
    Number,
    String,
    Boolean,
    RegExp,
    Error,
    TypeError,
    RangeError,
    SyntaxError,
    Symbol,
    parseInt,
    parseFloat,
    isNaN,
    isFinite,
    encodeURIComponent,
    decodeURIComponent,
    encodeURI,
    decodeURI,
    screen: {
      width: 1920,
      height: 1080,
      availWidth: 1920,
      availHeight: 1040,
      availLeft: 0,
      availTop: 0,
      colorDepth: 24,
      pixelDepth: 24,
    },
    navigator,
    location: {
      href: input.page_url || DEFAULT_PAGE_URL,
      origin: "https://auth.openai.com",
      protocol: "https:",
      host: "auth.openai.com",
      hostname: "auth.openai.com",
      pathname: "/about-you",
      search: "",
      hash: "",
    },
    document,
    history: {
      length: 2,
      state: null,
      pushState() {},
      replaceState() {},
      back() {},
      forward() {},
      go() {},
    },
    localStorage: makeStorage(),
    sessionStorage: makeStorage(),
    __reactRouterContext: {
      state: {
        loaderData: {
          "routes/layouts/client-auth-session-layout/layout": {
            session: {
              session_id: "",
              auth_session_logging_id: "",
              openai_client_id: "",
              app_name_enum: "",
              promo: "",
              signup_source: "",
              country_code_hint: "SG",
              is_missing_session: false,
            },
            seedCacheEntry: null,
          },
        },
        actionData: null,
        errors: null,
      },
    },
    btoa: (value) => Buffer.from(value, "binary").toString("base64"),
    atob: (value) => Buffer.from(value, "base64").toString("binary"),
  };
  context.window = context;
  context.self = context;
  context.top = context;
  context.globalThis = context;
  context.addEventListener = () => {};
  context.removeEventListener = () => {};
  context.postMessage = () => {};
  context.requestAnimationFrame = (cb) => setTimeout(cb, 16);
  context.cancelAnimationFrame = (id) => clearTimeout(id);
  context.Math = Math;
  context.Date = Date;
  context.JSON = JSON;

  vm.createContext(context);
  vm.runInContext(sdkSource, context, { timeout: 20000 });

  if (!context.__codexSentinel) {
    throw new Error("Sentinel SDK 内部方法暴露失败");
  }

  return context.__codexSentinel;
}

async function requestSentinelChallenge(input) {
  const response = await fetch("https://sentinel.openai.com/backend-api/sentinel/req", {
    method: "POST",
    headers: {
      origin: "https://sentinel.openai.com",
      referer: "https://sentinel.openai.com/backend-api/sentinel/frame.html?sv=20260219f9f6",
      "content-type": "text/plain;charset=UTF-8",
      "user-agent": input.user_agent || DEFAULT_USER_AGENT,
      "accept-language": "zh-CN,zh;q=0.9",
    },
    body: JSON.stringify({
      p: input.proof,
      id: input.did,
      flow: input.flow || DEFAULT_FLOW,
    }),
  });
  if (!response.ok) {
    throw new Error(`Sentinel req 失败: ${response.status}`);
  }
  return response.json();
}

async function mintHeaders(input) {
  const flow = input.flow || DEFAULT_FLOW;
  if (!input.did) {
    throw new Error("缺少 did");
  }

  const sdk = await loadSentinelSdkInternals(input);
  const proof = await sdk.P.getRequirementsToken();
  const chatRequest = await requestSentinelChallenge({
    ...input,
    proof,
    flow,
  });
  sdk.D(chatRequest, proof);

  const enforcementToken = await sdk.P.getEnforcementToken(chatRequest);
  let turnstileToken = null;
  if (chatRequest?.turnstile?.dx) {
    try {
      const { windowRef, documentRef } = createTurnstileRuntimeEnvironment(input);
      const vmResult = await runFromInputs(proof, chatRequest.turnstile.dx, {
        windowRef,
        documentRef,
        timeoutMs: 2000,
      });
      turnstileToken = vmResult.encodedValue;
    } catch (error) {
      turnstileToken = `turnstile_error:${String(error && error.message || error)}`;
    }
  }

  const headers = {
    "OpenAI-Sentinel-Token": sdk.ce(
      { p: enforcementToken, t: turnstileToken, c: chatRequest.token },
      flow,
    ),
  };

  if (chatRequest?.so?.required && typeof chatRequest.so.snapshot_dx === "string") {
    try {
      const soToken = await sdk.Nt(chatRequest.so.snapshot_dx);
      if (soToken) {
        headers["OpenAI-Sentinel-SO-Token"] = sdk.ce(
          { so: soToken, c: chatRequest.token },
          flow,
        );
      }
    } catch (error) {
      headers.sentinel_so_error = String(error && error.message || error);
    }
  }

  return headers;
}

async function main() {
  const input = await readJsonFromStdin();
  const headers = await mintHeaders(input);
  process.stdout.write(JSON.stringify({ headers }));
}

main().catch((error) => {
  process.stderr.write(String(error && error.stack || error));
  process.exit(1);
});
