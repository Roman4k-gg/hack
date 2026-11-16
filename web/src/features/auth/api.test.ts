import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useAuthStore } from '@/features/auth/authStore';
import { login } from './api';

const fetchMock = vi.fn();

beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
    useAuthStore.setState({ token: null, user: null });
});

afterEach(() => {
    vi.unstubAllGlobals();
});

describe('login', () => {
    it('нормализует почту и разбирает ответ', async () => {
        fetchMock.mockResolvedValue({
            ok: true,
            status: 200,
            text: async () =>
                JSON.stringify({
                    token: 'token-1',
                    token_type: 'Bearer',
                    expires_at: '2026-11-17T12:00:00Z',
                    user: {
                        id: 'user-1',
                        email: 'dasha@demo.local',
                        full_name: 'Дарья Соколова',
                        role: 'intern',
                        position: 'Стажёр-разработчик',
                        team_name: 'Search',
                        scopes: ['onboarding:plan:read'],
                    },
                }),
        });

        const session = await login('  Dasha@Demo.Local ');

        const [, init] = fetchMock.mock.calls[0];

        expect(init.method).toBe('POST');
        expect(init.body).toBe(JSON.stringify({ email: 'dasha@demo.local' }));
        expect(session.token).toBe('token-1');
        expect(session.user.role).toBe('intern');
        expect(session.user.scopes).toContain('onboarding:plan:read');
    });

    it('пробрасывает ошибку бэка', async () => {
        fetchMock.mockResolvedValue({
            ok: false,
            status: 401,
            text: async () =>
                JSON.stringify({
                    error: { code: 'UNAUTHORIZED', message: 'Пользователь не найден' },
                }),
        });

        await expect(login('nobody@demo.local')).rejects.toMatchObject({
            status: 401,
            code: 'UNAUTHORIZED',
        });
    });

    it('не пропускает ответ без скоупов', async () => {
        fetchMock.mockResolvedValue({
            ok: true,
            status: 200,
            text: async () =>
                JSON.stringify({
                    token: 'token-1',
                    token_type: 'Bearer',
                    expires_at: '2026-11-17T12:00:00Z',
                    user: {
                        id: 'user-1',
                        email: 'dasha@demo.local',
                        full_name: 'Дарья Соколова',
                        role: 'intern',
                    },
                }),
        });

        await expect(login('dasha@demo.local')).resolves.toMatchObject({
            user: { scopes: [] },
        });
    });
});
