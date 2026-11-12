import { useEffect, useRef } from 'react';
import { useChatStore } from '../chatStore';
import { parseChatEvent } from '../schemas';
import type { ChatEvent } from '../schemas';

export function useMockChat() {
    const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
    const version = useRef(0);

    function cancelTimer() {
        version.current += 1;

        if (timer.current !== null) {
            clearTimeout(timer.current);
            timer.current = null;
        }
    }

    function stop() {
        cancelTimer();
        useChatStore.getState().finish();
    }

    function run(content: string, retry = false) {
        const text = content.trim();
        const store = useChatStore.getState();

        if (!text || store.activeMessageId) return;

        cancelTimer();

        if (retry) {
            const index = store.messages
                .map((message) => message.role)
                .lastIndexOf('user');

            if (index < 0) return;

            useChatStore.setState({
                messages: store.messages.slice(0, index),
            });
        }

        useChatStore.getState().sendMessage(text);

        const currentVersion = version.current;
        const shouldFail = text.toLowerCase().includes('ошибка') && !retry;

        const answer =
            'Привет! Начни с карты адаптации: проверь задачи на сегодня, ' +
            'познакомься с наставником и запроси необходимые доступы. ' +
            'Если возникнет вопрос — напиши мне, и мы разберём его вместе.';

        const words = answer.match(/\S+\s*/g) ?? [];
        let position = 0;

        function emit(event: ChatEvent) {
            const parsed = parseChatEvent({
                event: event.type,
                data: JSON.stringify(event),
            });

            useChatStore.getState().applyEvent(parsed);
        }

        function tick() {
            if (version.current !== currentVersion) return;

            timer.current = null;

            try {
                if (shouldFail && position === 4) {
                    emit({
                        type: 'error',
                        message: 'Не удалось получить ответ. Попробуй ещё раз.',
                    });
                    return;
                }

                if (position >= words.length) {
                    emit({ type: 'done' });
                    return;
                }

                emit({ type: 'delta', text: words[position] });
                position += 1;
                timer.current = setTimeout(tick, 100);
            } catch (error) {
                useChatStore.getState().fail(
                    error instanceof Error
                        ? error.message
                        : 'Неизвестная ошибка',
                );
            }
        }

        timer.current = setTimeout(tick, 900);
    }

    function retry() {
        const state = useChatStore.getState();

        if (state.status !== 'error') return;

        const lastQuestion = [...state.messages]
            .reverse()
            .find((message) => message.role === 'user');

        if (lastQuestion) run(lastQuestion.content, true);
    }

    useEffect(() => {
        return () => {
            version.current += 1;

            if (timer.current !== null) {
                clearTimeout(timer.current);
            }

            useChatStore.getState().finish();
        };
    }, []);

    return {
        send: (content: string) => run(content),
        stop,
        retry,
    };
}