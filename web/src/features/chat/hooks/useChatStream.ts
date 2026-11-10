
import { useCallback, useEffect, useRef, useState } from 'react';
import { createSSEParser } from '../../../lib/sse/parseSSE';
import type { SSEEvent } from '../../../lib/sse/parseSSE';

interface ChatStreamOptions {
    url: string;
    body: unknown;
    headers?: Record<string, string>;
    onEvent: (event: SSEEvent) => void;
}

type StreamStatus = 'idle' | 'streaming' | 'error';

export function useChatStream() {
    const controllerRef = useRef<AbortController | null>(null);

    const [status, setStatus] = useState<StreamStatus>('idle');
    const [error, setError] = useState<string | null>(null);

    const stop = useCallback(() => {
        controllerRef.current?.abort();
        controllerRef.current = null;
        setStatus('idle');
    }, []);

    const start = useCallback(async ({
        url,
        body,
        headers,
        onEvent,
    }: ChatStreamOptions) => {
        controllerRef.current?.abort();

        const controller = new AbortController();
        controllerRef.current = controller;

        setStatus('streaming');
        setError(null);

        try {
            const response = await fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Accept: 'text/event-stream',
                    ...headers,
                },
                body: JSON.stringify(body),
                signal: controller.signal,
            });

            if (!response.ok) {
                throw new Error(`Ошибка HTTP: ${response.status}`);
            }

            if (!response.body) {
                throw new Error('Сервер не вернул поток данных');
            }

            const reader = response.body.getReader();
            const decoder = new TextDecoder();

            const parser = createSSEParser((event) => {
                if (!controller.signal.aborted) {
                    onEvent(event);
                }
            });

            while (!controller.signal.aborted) {
                const { done, value } = await reader.read();

                if (done) {
                    break;
                }

                parser.feed(decoder.decode(value, { stream: true }));
            }

            if (!controller.signal.aborted) {
                parser.feed(decoder.decode());
                parser.flush();
            }
        } catch (err) {
            if (controller.signal.aborted) {
                return;
            }

            if (controllerRef.current === controller) {
                setError(
                    err instanceof Error
                        ? err.message
                        : 'Неизвестная ошибка'
                );
                setStatus('error');
            }
        } finally {
            if (controllerRef.current === controller) {
                controllerRef.current = null;

                setStatus((current) =>
                    current === 'streaming' ? 'idle' : current
                );
            }
        }
    }, []);

    useEffect(() => {
        return () => {
            controllerRef.current?.abort();
            controllerRef.current = null;
        };
    }, []);

    return {
        start,
        stop,
        status,
        error,
        isStreaming: status === 'streaming',
    };
}
