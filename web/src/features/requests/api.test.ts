import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchRequests } from './api';
import { mockRequests } from './mock';

const fetchMock = vi.fn();

beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
});

function response(status: number, body: unknown): Response {
    return {
        ok: status >= 200 && status < 300,
        status,
        text: async () => JSON.stringify(body),
    } as Response;
}

describe('заявки', () => {
    it('берёт данные с API, когда бэк ответил', async () => {
        fetchMock.mockResolvedValue(
            response(200, {
                items: [
                    {
                        request_id: 'req-1',
                        id: 'req-1',
                        kind: 'absence',
                        type: 'absence',
                        title: 'Отгул 20.11.2026',
                        status: 'pending',
                        external_id: 'ABS-1025',
                        number: 'ABS-1025',
                        approver: { id: '1', full_name: 'Ольга Кузнецова' },
                        created_at: '2026-11-14T09:40:34Z',
                        updated_at: '2026-11-14T09:40:34Z',
                    },
                ],
            }),
        );

        const result = await fetchRequests();

        expect(result.demo).toBe(false);
        expect(result.data).toHaveLength(1);
        expect(result.data[0].title).toBe('Отгул 20.11.2026');
    });

    it('уходит в демо-режим, если бэк ещё не отдаёт эндпоинт', async () => {
        fetchMock.mockResolvedValue(response(404, { error: { code: 'NOT_FOUND' } }));

        const result = await fetchRequests();

        expect(result.demo).toBe(true);
        expect(result.data).toEqual(mockRequests);
    });

    it('не прячет настоящую ошибку под демо-режим', async () => {
        fetchMock.mockResolvedValue(response(500, { error: { code: 'INTERNAL_ERROR' } }));

        await expect(fetchRequests()).rejects.toThrow();
    });
});
