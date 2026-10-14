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
}

// Kept in memory only; the matching HttpOnly cookie is set by the API.
let csrfToken: string | null = null;

export function resetCsrf() {
  csrfToken = null;
}

async function request(path: string, init: RequestInit): Promise<Response> {
  try {
    return await fetch(`${BASE_URL}${path}`, { ...init, credentials: "include" });
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

export async function api<T>(
  path: string,
  { method = "GET", body }: { method?: string; body?: unknown } = {},
): Promise<T> {
  const write = method !== "GET";
  const send = async () => {
    const headers: Record<string, string> = {};
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (write) headers["X-CSRF-Token"] = csrfToken ?? (await fetchCsrf());
    return request(path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  };

  let res = await send();
  // Stale CSRF token: refetch once and retry (frontend plan section 5).
  if (write && res.status === 403 && (await res.clone().text()).includes("CSRF")) {
    resetCsrf();
    res = await send();
  }

  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    const retry = Number(res.headers.get("Retry-After"));
    throw new ApiError(
      res.status,
      data?.error ?? "ERROR",
      data?.message ?? "Something went wrong. Please try again.",
      data?.details,
      retry > 0 ? retry : undefined,
    );
  }
  return data as T;
}
