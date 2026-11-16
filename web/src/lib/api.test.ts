import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useAuthStore } from '@/features/auth/authStore';
import { ApiError, apiFetch } from './api';

const fetchMock = vi.fn();

beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
    useAuthStore.setState({ token: null, user: null });
});

afterEach(() => {
    vi.unstubAllGlobals();
});

function jsonResponse(body: unknown, status = 200): Response {
    return {
        ok: status >= 200 && status < 300,
        status,
        text: async () => JSON.stringify(body),
    } as Response;
}

describe('apiFetch', () => {
    it('добавляет токен из auth-стора', async () => {
        useAuthStore.setState({ token: 'token-1', user: null });

        fetchMock.mockResolvedValue(jsonResponse({ ok: true }));

        await apiFetch('/me');

        const [, init] = fetchMock.mock.calls[0];
        const headers = init.headers as Headers;

        expect(headers.get('Authorization')).toBe('Bearer token-1');
        expect(headers.get('Accept')).toBe('application/json');
    });

    it('не добавляет заголовок, если токена нет', async () => {
        fetchMock.mockResolvedValue(jsonResponse({ ok: true }));

        await apiFetch('/me');

        const [, init] = fetchMock.mock.calls[0];
        const headers = init.headers as Headers;

        expect(headers.get('Authorization')).toBeNull();
    });

    it('ставит Content-Type только при наличии тела', async () => {
        fetchMock.mockResolvedValue(jsonResponse({ ok: true }));

        await apiFetch('/me');
        await apiFetch('/requests', { method: 'POST', body: JSON.stringify({ a: 1 }) });

        const [, first] = fetchMock.mock.calls[0];
        const [, second] = fetchMock.mock.calls[1];

        expect((first.headers as Headers).get('Content-Type')).toBeNull();
        expect((second.headers as Headers).get('Content-Type')).toBe('application/json');
    });

    it('возвращает распарсенный ответ', async () => {
        fetchMock.mockResolvedValue(jsonResponse({ intern: { id: 'user-1' } }));

        const result = await apiFetch<{ intern: { id: string } }>('/me');

        expect(result.intern.id).toBe('user-1');
    });

    it('превращает конверт ошибки в ApiError', async () => {
        fetchMock.mockResolvedValue(
            jsonResponse(
                {
                    error: {
                        code: 'FORBIDDEN',
                        message: 'Недостаточно прав',
                        trace_id: 'trace-1',
                    },
                },
                403,
            ),
        );

        await expect(apiFetch('/team/interns')).rejects.toMatchObject({
            name: 'ApiError',
            status: 403,
            code: 'FORBIDDEN',
            message: 'Недостаточно прав',
        });
    });

    it('подставляет дефолтный код ошибки, если конверта нет', async () => {
        fetchMock.mockResolvedValue(jsonResponse({}, 500));

        await expect(apiFetch('/team/interns')).rejects.toMatchObject({
            status: 500,
            code: 'UNKNOWN_ERROR',
        });
    });

    it('ApiError хранит статус и код', () => {
        const error = new ApiError(404, 'NOT_FOUND', 'Не найдено');

        expect(error.status).toBe(404);
        expect(error.code).toBe('NOT_FOUND');
        expect(error.message).toBe('Не найдено');
        expect(error).toBeInstanceOf(Error);
    });
});
