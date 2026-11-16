import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useAuthStore } from '@/features/auth/authStore';
import { useChatStore } from '../chatStore';
import { useChatSession } from './useChatSession';

const fetchMock = vi.fn();

beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
    useAuthStore.setState({ token: 'token-1', user: null });
    useChatStore.setState({
        messages: [],
        status: 'idle',
        error: null,
        activeMessageId: null,
    });
});

afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
});

function errorResponse(status: number): Response {
    return {
        ok: false,
        status,
        text: async () =>
            JSON.stringify({
                error: { code: 'SERVICE_UNAVAILABLE', message: 'Ассистент недоступен' },
            }),
    } as Response;
}

function streamResponse(events: string[]): Response {
    const encoder = new TextEncoder();

    return {
        ok: true,
        status: 200,
        body: {
            getReader: () => {
                let index = 0;

                return {
                    read: async () => {
                        if (index >= events.length) {
                            return { done: true, value: undefined };
                        }

                        const chunk = encoder.encode(events[index]);
                        index += 1;

                        return { done: false, value: chunk };
                    },
                };
            },
        },
        text: async () => '',
    } as unknown as Response;
}

describe('useChatSession', () => {
    it('переключается в демо-режим, когда AI-сервис недоступен', async () => {
        fetchMock.mockResolvedValue(errorResponse(503));

        const { result } = renderHook(() => useChatSession());

        act(() => {
            result.current.send('Где взять доступ к Jira?');
        });

        await waitFor(() => {
            expect(result.current.mode).toBe('demo');
        });
    });

    it('не маскирует ошибку 500 под демо-режим', async () => {
        fetchMock.mockResolvedValue(errorResponse(500));

        const { result } = renderHook(() => useChatSession());

        act(() => {
            result.current.send('Где взять доступ к Jira?');
        });

        await waitFor(() => {
            expect(useChatStore.getState().status).toBe('error');
        });

        expect(result.current.mode).toBe('live');
        expect(useChatStore.getState().error).toContain('500');
    });

    it('применяет события живого потока', async () => {
        fetchMock.mockResolvedValue(
            streamResponse([
                'event: message\ndata: {"type":"delta","text":"Спроси наставника"}\n\n',
                'event: message\ndata: {"type":"done"}\n\n',
            ]),
        );

        const { result } = renderHook(() => useChatSession());

        act(() => {
            result.current.send('Что делать дальше?');
        });

        await waitFor(() => {
            const messages = useChatStore.getState().messages;

            expect(messages).toHaveLength(2);
            expect(messages[1].content).toBe('Спроси наставника');
        });

        expect(result.current.mode).toBe('live');
        expect(useChatStore.getState().status).toBe('idle');
    });

    it('сообщает об оборванном ответе, если не пришло done', async () => {
        fetchMock.mockResolvedValue(
            streamResponse([
                'event: message\ndata: {"type":"delta","text":"Начало ответа"}\n\n',
            ]),
        );

        const { result } = renderHook(() => useChatSession());

        act(() => {
            result.current.send('Что делать дальше?');
        });

        await waitFor(() => {
            expect(useChatStore.getState().status).toBe('error');
        });

        expect(useChatStore.getState().error).toContain('прервался');
        expect(result.current.mode).toBe('live');
    });

    it('не отправляет запрос на пустое сообщение', () => {
        const { result } = renderHook(() => useChatSession());

        act(() => {
            result.current.send('   ');
        });

        expect(fetchMock).not.toHaveBeenCalled();
        expect(useChatStore.getState().messages).toHaveLength(0);
    });
});
