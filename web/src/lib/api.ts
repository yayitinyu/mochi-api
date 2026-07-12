export interface ApiEnvelope<T> {
  success: boolean;
  message?: string;
  data?: T;
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

type Options = {
  method?: string;
  body?: unknown;
};

// request wraps fetch with the {success, message, data} envelope.
// On 401 it dispatches a global event so the app can redirect to login.
export async function request<T>(path: string, opts: Options = {}): Promise<T> {
  const res = await fetch(path, {
    method: opts.method ?? 'GET',
    headers: opts.body ? { 'Content-Type': 'application/json' } : undefined,
    body: opts.body ? JSON.stringify(opts.body) : undefined,
    credentials: 'same-origin',
  });

  if (res.status === 401) {
    window.dispatchEvent(new CustomEvent('mochi:unauthorized'));
  }

  let payload: ApiEnvelope<T>;
  try {
    payload = await res.json();
  } catch {
    throw new ApiError(res.status, '服务器响应异常');
  }
  if (!res.ok || !payload.success) {
    throw new ApiError(res.status, payload.message ?? '请求失败');
  }
  return payload.data as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: 'POST', body }),
  put: <T>(path: string, body?: unknown) => request<T>(path, { method: 'PUT', body }),
  del: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
};
