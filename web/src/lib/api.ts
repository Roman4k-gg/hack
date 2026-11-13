import { useAuthStore } from '@/features/auth/authStore';

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

const rawBase = import.meta.env.VITE_API_BASE_URL ?? '/api/v1';
const API_BASE = rawBase.endsWith('/api') ? `${rawBase}/v1` : rawBase;

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const token = useAuthStore.getState().token;
  const headers = new Headers(init.headers);

  headers.set('Accept', 'application/json');

  if (init.body) {
    headers.set('Content-Type', 'application/json');
  }

  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE}${path}`, { ...init, headers });

  const text = await response.text();
  const payload = text ? safeParse(text) : null;

  if (!response.ok) {
    const envelope = (payload as { error?: { code?: string; message?: string } })
      ?.error;

    throw new ApiError(
      response.status,
      envelope?.code ?? 'UNKNOWN_ERROR',
      envelope?.message ?? 'Не удалось выполнить запрос',
    );
  }

  return payload as T;
}

function safeParse(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}
