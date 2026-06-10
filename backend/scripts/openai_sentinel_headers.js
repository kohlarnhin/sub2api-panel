"use strict";

const vm = require("vm");
const path = require("path");
const net = require("net");
const tls = require("tls");
const zlib = require("zlib");
const { spawnSync } = require("child_process");
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

function normalizeHeaders(headers) {
  const normalized = {};
  if (!headers) return normalized;
  if (typeof headers.forEach === "function") {
    headers.forEach((value, key) => {
      normalized[String(key).toLowerCase()] = String(value);
    });
    return normalized;
  }
  if (Array.isArray(headers)) {
    for (const [key, value] of headers) {
      normalized[String(key).toLowerCase()] = String(value);
    }
    return normalized;
  }
  for (const [key, value] of Object.entries(headers)) {
    normalized[String(key).toLowerCase()] = String(value);
  }
  return normalized;
}

function bodyToBuffer(body) {
  if (body == null) return Buffer.alloc(0);
  if (Buffer.isBuffer(body)) return body;
  if (body instanceof Uint8Array) return Buffer.from(body);
  return Buffer.from(String(body));
}

function fetchHelperPath(input) {
  return String(input.fetch_helper || process.env.SUB2API_SENTINEL_FETCH_HELPER || "").trim();
}

function proxyAuthHeader(proxyURL) {
  if (!proxyURL.username && !proxyURL.password) return "";
  const username = decodeURIComponent(proxyURL.username || "");
  const password = decodeURIComponent(proxyURL.password || "");
  return `Basic ${Buffer.from(`${username}:${password}`).toString("base64")}`;
}

function connectSocket(options, secure) {
  return new Promise((resolve, reject) => {
    const socket = secure ? tls.connect(options) : net.connect(options);
    const timer = setTimeout(() => {
      socket.destroy(new Error("proxy connect timeout"));
    }, 30000);
    const onReady = () => {
      clearTimeout(timer);
      socket.off("error", onError);
      resolve(socket);
    };
    const onError = (error) => {
      clearTimeout(timer);
      reject(error);
    };
    if (secure) {
      socket.once("secureConnect", onReady);
    } else {
      socket.once("connect", onReady);
    }
    socket.once("error", onError);
  });
}

function readUntilHeadersEnd(socket) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let total = 0;
    const cleanup = () => {
      socket.off("data", onData);
      socket.off("error", onError);
      socket.off("end", onEnd);
      socket.off("close", onEnd);
    };
    const onError = (error) => {
      cleanup();
      reject(error);
    };
    const onEnd = () => {
      cleanup();
      reject(new Error("proxy closed before CONNECT completed"));
    };
    const onData = (chunk) => {
      chunks.push(chunk);
      total += chunk.length;
      const buffer = Buffer.concat(chunks, total);
      const idx = buffer.indexOf("\r\n\r\n");
      if (idx === -1) return;
      cleanup();
      const head = buffer.slice(idx + 4);
      if (head.length) socket.unshift(head);
      resolve(buffer.slice(0, idx).toString("latin1"));
    };
    socket.on("data", onData);
    socket.once("error", onError);
    socket.once("end", onEnd);
    socket.once("close", onEnd);
  });
}

async function connectViaHTTPProxy(proxyURL, targetURL, useTunnel) {
  const proxyPort = Number(proxyURL.port || (proxyURL.protocol === "https:" ? 443 : 80));
  const socket = await connectSocket({
    host: proxyURL.hostname,
    port: proxyPort,
    servername: proxyURL.hostname,
  }, proxyURL.protocol === "https:");

  if (!useTunnel) return socket;

  const headers = [
    `CONNECT ${targetURL.hostname}:${targetURL.port || 443} HTTP/1.1`,
    `Host: ${targetURL.hostname}:${targetURL.port || 443}`,
    "Connection: close",
  ];
  const auth = proxyAuthHeader(proxyURL);
  if (auth) headers.push(`Proxy-Authorization: ${auth}`);
  socket.write(`${headers.join("\r\n")}\r\n\r\n`);

  const responseHead = await readUntilHeadersEnd(socket);
  const statusMatch = responseHead.match(/^HTTP\/\d(?:\.\d)?\s+(\d+)/i);
  const status = statusMatch ? Number(statusMatch[1]) : 0;
  if (status < 200 || status >= 300) {
    socket.destroy();
    throw new Error(`proxy CONNECT failed: ${status || "unknown"}`);
  }
  return socket;
}

function readExact(socket, length) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let total = 0;
    const cleanup = () => {
      socket.off("data", onData);
      socket.off("error", onError);
      socket.off("end", onEnd);
      socket.off("close", onEnd);
    };
    const onError = (error) => {
      cleanup();
      reject(error);
    };
    const onEnd = () => {
      cleanup();
      reject(new Error("socks5 proxy closed unexpectedly"));
    };
    const onData = (chunk) => {
      chunks.push(chunk);
      total += chunk.length;
      if (total < length) return;
      cleanup();
      const buffer = Buffer.concat(chunks, total);
      const rest = buffer.slice(length);
      if (rest.length) socket.unshift(rest);
      resolve(buffer.slice(0, length));
    };
    socket.on("data", onData);
    socket.once("error", onError);
    socket.once("end", onEnd);
    socket.once("close", onEnd);
  });
}

async function connectViaSocks5(proxyURL, targetURL) {
  const proxyPort = Number(proxyURL.port || 1080);
  const socket = await connectSocket({ host: proxyURL.hostname, port: proxyPort }, false);
  const username = decodeURIComponent(proxyURL.username || "");
  const password = decodeURIComponent(proxyURL.password || "");
  const needsAuth = username !== "" || password !== "";

  socket.write(needsAuth ? Buffer.from([0x05, 0x02, 0x00, 0x02]) : Buffer.from([0x05, 0x01, 0x00]));
  let response = await readExact(socket, 2);
  if (response[0] !== 0x05) {
    socket.destroy();
    throw new Error("socks5 proxy returned invalid version");
  }
  if (response[1] === 0xff) {
    socket.destroy();
    throw new Error("socks5 proxy has no acceptable auth method");
  }
  if (response[1] === 0x02) {
    const user = Buffer.from(username);
    const pass = Buffer.from(password);
    if (user.length > 255 || pass.length > 255) {
      socket.destroy();
      throw new Error("socks5 proxy credentials are too long");
    }
    socket.write(Buffer.concat([Buffer.from([0x01, user.length]), user, Buffer.from([pass.length]), pass]));
    response = await readExact(socket, 2);
    if (response[1] !== 0x00) {
      socket.destroy();
      throw new Error("socks5 proxy authentication failed");
    }
  }

  const host = Buffer.from(targetURL.hostname);
  const port = Number(targetURL.port || (targetURL.protocol === "https:" ? 443 : 80));
  if (host.length > 255) {
    socket.destroy();
    throw new Error("target host is too long for socks5");
  }
  socket.write(Buffer.concat([
    Buffer.from([0x05, 0x01, 0x00, 0x03, host.length]),
    host,
    Buffer.from([(port >> 8) & 0xff, port & 0xff]),
  ]));
  response = await readExact(socket, 4);
  if (response[1] !== 0x00) {
    socket.destroy();
    throw new Error(`socks5 proxy CONNECT failed: ${response[1]}`);
  }
  if (response[3] === 0x01) {
    await readExact(socket, 6);
  } else if (response[3] === 0x03) {
    const len = await readExact(socket, 1);
    await readExact(socket, len[0] + 2);
  } else if (response[3] === 0x04) {
    await readExact(socket, 18);
  } else {
    socket.destroy();
    throw new Error("socks5 proxy returned invalid address type");
  }
  return socket;
}

function secureTargetSocket(socket, targetURL) {
  return new Promise((resolve, reject) => {
    const secure = tls.connect({
      socket,
      servername: targetURL.hostname,
    });
    secure.once("secureConnect", () => resolve(secure));
    secure.once("error", reject);
  });
}

async function openProxySocket(proxyURL, targetURL) {
  const scheme = proxyURL.protocol.replace(":", "").toLowerCase();
  const targetIsHTTPS = targetURL.protocol === "https:";
  if (scheme === "http" || scheme === "https") {
    const socket = await connectViaHTTPProxy(proxyURL, targetURL, targetIsHTTPS);
    return targetIsHTTPS ? secureTargetSocket(socket, targetURL) : { socket, absolutePath: true };
  }
  if (scheme === "socks5") {
    const socket = await connectViaSocks5(proxyURL, targetURL);
    return targetIsHTTPS ? secureTargetSocket(socket, targetURL) : socket;
  }
  throw new Error(`unsupported proxy scheme: ${scheme}`);
}

function decodeChunkedBody(buffer) {
  const chunks = [];
  let offset = 0;
  while (offset < buffer.length) {
    const lineEnd = buffer.indexOf("\r\n", offset);
    if (lineEnd === -1) break;
    const sizeText = buffer.slice(offset, lineEnd).toString("ascii").split(";")[0].trim();
    const size = parseInt(sizeText, 16);
    if (!Number.isFinite(size)) break;
    offset = lineEnd + 2;
    if (size === 0) break;
    chunks.push(buffer.slice(offset, offset + size));
    offset += size + 2;
  }
  return Buffer.concat(chunks);
}

function decompressBody(buffer, encoding) {
  const value = String(encoding || "").toLowerCase();
  if (value.includes("br")) return zlib.brotliDecompressSync(buffer);
  if (value.includes("gzip")) return zlib.gunzipSync(buffer);
  if (value.includes("deflate")) return zlib.inflateSync(buffer);
  return buffer;
}

function readHTTPResponse(socket) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    socket.on("data", (chunk) => chunks.push(chunk));
    socket.once("error", reject);
    socket.once("end", () => {
      const buffer = Buffer.concat(chunks);
      const idx = buffer.indexOf("\r\n\r\n");
      if (idx === -1) {
        reject(new Error("invalid proxy fetch response"));
        return;
      }
      const head = buffer.slice(0, idx).toString("latin1");
      const lines = head.split("\r\n");
      const statusMatch = lines[0].match(/^HTTP\/\d(?:\.\d)?\s+(\d+)\s*(.*)$/i);
      const status = statusMatch ? Number(statusMatch[1]) : 0;
      const statusText = statusMatch ? statusMatch[2] : "";
      const headers = {};
      for (const line of lines.slice(1)) {
        const colon = line.indexOf(":");
        if (colon <= 0) continue;
        headers[line.slice(0, colon).trim().toLowerCase()] = line.slice(colon + 1).trim();
      }
      let body = buffer.slice(idx + 4);
      if (String(headers["transfer-encoding"] || "").toLowerCase().includes("chunked")) {
        body = decodeChunkedBody(body);
      }
      body = decompressBody(body, headers["content-encoding"]);
      resolve({ status, statusText, headers, body });
    });
  });
}

class ProxyFetchResponse {
  constructor(result) {
    this.status = result.status;
    this.statusText = result.statusText;
    this.ok = result.status >= 200 && result.status < 300;
    this.headers = {
      get(name) {
        return result.headers[String(name).toLowerCase()] || null;
      },
    };
    this.body = result.body;
  }

  async text() {
    return this.body.toString("utf8");
  }

  async json() {
    return JSON.parse(await this.text());
  }
}

async function helperFetch(input, url, options = {}) {
  const helper = fetchHelperPath(input);
  if (!helper) return null;
  const body = bodyToBuffer(options.body);
  const payload = {
    proxy: String(input.proxy || "").trim(),
    url: String(url),
    method: String(options.method || (body.length > 0 ? "POST" : "GET")).toUpperCase(),
    headers: normalizeHeaders(options.headers),
    body_base64: body.toString("base64"),
  };
  const result = spawnSync(helper, ["sentinel-fetch"], {
    input: JSON.stringify(payload),
    encoding: "utf8",
    maxBuffer: 30 * 1024 * 1024,
    timeout: 45000,
  });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    const detail = String(result.stderr || result.stdout || `exit ${result.status}`).trim();
    throw new Error(`sentinel fetch helper failed: ${detail}`);
  }
  let parsed;
  try {
    parsed = JSON.parse(String(result.stdout || "{}"));
  } catch (error) {
    throw new Error(`sentinel fetch helper returned invalid JSON: ${String(error && error.message || error)}`);
  }
  return new ProxyFetchResponse({
    status: Number(parsed.status || 0),
    statusText: String(parsed.status_text || ""),
    headers: parsed.headers || {},
    body: Buffer.from(String(parsed.body_base64 || ""), "base64"),
  });
}

async function directProxyFetch(proxy, url, options = {}) {
  const targetURL = new URL(String(url));
  const proxyURL = new URL(proxy);
  let socketRef = await openProxySocket(proxyURL, targetURL);
  let absolutePath = false;
  if (socketRef && socketRef.socket) {
    absolutePath = Boolean(socketRef.absolutePath);
    socketRef = socketRef.socket;
  }
  const socket = socketRef;
  const body = bodyToBuffer(options.body);
  const headers = normalizeHeaders(options.headers);
  headers.host = targetURL.host;
  headers.connection = "close";
  if (!headers["accept-encoding"]) headers["accept-encoding"] = "identity";
  if (body.length > 0 && !headers["content-length"]) headers["content-length"] = String(body.length);
  const method = String(options.method || (body.length > 0 ? "POST" : "GET")).toUpperCase();
  const requestPath = absolutePath ? targetURL.href : `${targetURL.pathname || "/"}${targetURL.search || ""}`;
  const head = [
    `${method} ${requestPath} HTTP/1.1`,
    ...Object.entries(headers).map(([key, value]) => `${key}: ${value}`),
    "",
    "",
  ].join("\r\n");
  const responsePromise = readHTTPResponse(socket);
  socket.write(head);
  if (body.length > 0) socket.write(body);
  socket.end();
  return new ProxyFetchResponse(await responsePromise);
}

function makeFetch(input) {
  const proxy = String(input.proxy || "").trim();
  if (!proxy) return fetch;
  return async (url, options) => {
    const helperResponse = await helperFetch(input, url, options);
    if (helperResponse) return helperResponse;
    return directProxyFetch(proxy, url, options);
  };
}

async function fetchText(input, url) {
  const response = await makeFetch(input)(url);
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
  const bootstrap = await fetchText(input, SENTINEL_BOOTSTRAP_URL);
  const sdkUrlMatch = bootstrap.match(/src = '([^']+sdk\.js)'/);
  if (!sdkUrlMatch) {
    throw new Error("未解析到 Sentinel SDK 地址");
  }

  let sdkSource = await fetchText(input, sdkUrlMatch[1]);
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
    fetch: makeFetch(input),
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
  const response = await makeFetch(input)("https://sentinel.openai.com/backend-api/sentinel/req", {
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
