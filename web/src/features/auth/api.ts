import { demoUsers } from './demoUsers';
import { loginResponseSchema, type LoginResponse } from './types';

export async function login(email: string): Promise<LoginResponse> {
  await new Promise((resolve) => setTimeout(resolve, 300));

  const user = demoUsers.find(
    (item) => item.email === email.trim().toLowerCase(),
  );
  if (!user) {
    throw new Error('Пользователь не найден');
  }

  return loginResponseSchema.parse({
    token: `demo-token-${user.id}`,
    token_type: 'Bearer',
    expires_at: new Date(Date.now() + 8 * 60 * 60 * 1000).toISOString(),
    user,
  });
}
