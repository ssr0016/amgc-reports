const BASE_URL = process.env.NEXT_PUBLIC_API_URL;

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly details?: Record<string, string>,
    readonly retryAfter?: number,
  ) {
    super(message);
  }

  get isConflict() {
    return this.status === 409;
  }
  get isValidation() {
    return this.status === 422;
  }
  get isRateLimited() {
    return this.status === 429;
  }
  get isNetwork() {
    return this.status === 0;
  }
}

// Kept in memory only; the matching HttpOnly cookie is set by the API.
let csrfToken: string | null = null;

export function resetCsrf() {
  csrfToken = null;
}

let onUnauthorized: (() => void) | null = null;

// Called on any 401 except a failed login (wrong password). Registered by the auth UI.
export function setOnUnauthorized(handler: (() => void) | null) {
  onUnauthorized = handler;
}

async function request(path: string, init: RequestInit): Promise<Response> {
  try {
    return await fetch(`${BASE_URL}${path}`, {
      ...init,
      credentials: "include",
      signal: AbortSignal.timeout(15_000),
    });
  } catch {
    throw new ApiError(0, "NETWORK", "Cannot reach the server. Check your connection and try again.");
  }
}

async function fetchCsrf(): Promise<string> {
  const res = await request("/auth/csrf", {});
  if (!res.ok) throw new ApiError(res.status, "CSRF", "Could not get a CSRF token");
  csrfToken = (await res.json()).csrf_token;
  return csrfToken!;
}

// Routes that must never trigger a refresh: they are how a session starts or ends, or the refresh itself.
const NO_REFRESH = new Set(["/auth/login", "/auth/refresh", "/auth/logout"]);

// One request, with the CSRF token a write needs. A stale token is fetched again once (frontend plan section 5).
async function send(path: string, method: string, body: unknown): Promise<Response> {
  const write = method !== "GET";
  const attempt = async () => {
    const headers: Record<string, string> = {};
    const multipart = body instanceof FormData;
    if (body !== undefined && !multipart) headers["Content-Type"] = "application/json";
    if (write) headers["X-CSRF-Token"] = csrfToken ?? (await fetchCsrf());
    return request(path, {
      method,
      headers,
      body: body === undefined ? undefined : body instanceof FormData ? body : JSON.stringify(body),
    });
  };

  const res = await attempt();
  if (write && res.status === 403 && (await res.clone().text()).includes("CSRF")) {
    resetCsrf();
    return attempt();
  }
  return res;
}

// Single flight: every request that gets a 401 while a refresh is running waits for that one refresh
// instead of starting its own. The refresh token rotates, so a second concurrent call would be refused.
let refreshing: Promise<boolean> | null = null;

function refreshSession(): Promise<boolean> {
  refreshing ??= send("/auth/refresh", "POST", undefined)
    .then(
      (res) => res.ok,
      () => false, // a network error is not a reason to sign the user out here; the 401 below decides
    )
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

// Sends the request, refreshing the session once on a 401, and throws an ApiError for any non-2xx answer.
async function call(path: string, method: string, body: unknown, silent401: boolean): Promise<Response> {
  let res = await send(path, method, body);
  // The access token lives 10 minutes. On a 401, refresh once and replay the request once.
  if (res.status === 401 && !NO_REFRESH.has(path) && (await refreshSession())) {
    res = await send(path, method, body);
  }
  if (res.ok) return res;

  const data = await res.json().catch(() => null);
  if (res.status === 401 && !silent401 && path !== "/auth/login") onUnauthorized?.();
  const retry = Number(res.headers.get("Retry-After"));
  throw new ApiError(
    res.status,
    data?.error ?? "ERROR",
    data?.message ?? "Something went wrong. Please try again.",
    data?.details,
    retry > 0 ? retry : undefined,
  );
}

export async function api<T>(
  path: string,
  { method = "GET", body, silent401 = false }: { method?: string; body?: unknown; silent401?: boolean } = {},
): Promise<T> {
  const res = await call(path, method, body, silent401);
  if (res.status === 204) return undefined as T;
  return (await res.json().catch(() => null)) as T;
}

// A request that answers with a file (the Excel exports; POST unless the download writes nothing). The name comes from Content-Disposition, preferring the UTF-8 filename*.
export async function downloadFile(path: string, body?: unknown, method = "POST"): Promise<{ blob: Blob; filename: string }> {
  const res = await call(path, method, body, false);
  const disposition = res.headers.get("Content-Disposition") ?? "";
  const utf8 = /filename\*=UTF-8''([^;]+)/i.exec(disposition)?.[1];
  const plain = /filename="([^"]+)"/i.exec(disposition)?.[1];
  let filename = plain ?? "download";
  if (utf8) {
    try {
      filename = decodeURIComponent(utf8);
    } catch {
      // a malformed escape: keep the ASCII fallback
    }
  }
  return { blob: await res.blob(), filename };
}

// Hands a downloaded file to the browser as a normal download.
export function saveFile({ blob, filename }: { blob: Blob; filename: string }) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  // Some browsers start the download after click() returns, so revoke the URL a little later.
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
