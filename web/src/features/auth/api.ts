import { apiFetch } from '@/lib/api';
import { loginResponseSchema, type LoginResponse } from './types';

export async function login(email: string): Promise<LoginResponse> {
  const payload = await apiFetch<unknown>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email: email.trim().toLowerCase() }),
  });

  return loginResponseSchema.parse(payload);
}
