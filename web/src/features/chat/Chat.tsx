import { useChatStore } from './chatStore';
import { useChatSession } from './hooks/useChatSession.ts';
import { MessageList } from './components/MessageList';
import { Composer } from './components/Composer';
import { StreamingIndicator } from './components/StreamingIndicator';
import './chat.css';

export function Chat() {
    const messages = useChatStore((state) => state.messages);
    const status = useChatStore((state) => state.status);
    const error = useChatStore((state) => state.error);
    const { send, stop, retry, mode } = useChatSession();

    const busy = status === 'loading' || status === 'streaming';

    const visibleMessages = messages.filter(
        (message) =>
            message.content ||
            message.toolCalls?.length ||
            message.citations?.length,
    );

    return (
        <section className="chat">
            {mode === 'demo' && (
                <div className="chat__notice" role="status">
                    AI-сервис недоступен — показан демо-режим без обращения к
                    бэкенду.
                </div>
            )}

            {messages.length === 0 ? (
                <div className="chat__empty">
                    <h2>Привет! Я AI-buddy</h2>
                    <p>Задай вопрос об адаптации или задачах на сегодня.</p>
                </div>
            ) : (
                <MessageList messages={visibleMessages} />
            )}

            {status === 'loading' && (
                <div className="streaming-indicator" role="status">
                    AI-buddy готовит ответ…
                </div>
            )}

            {status === 'streaming' && <StreamingIndicator />}

            {status === 'error' && (
                <div className="chat__error" role="alert">
                    <span>{error}</span>
                    <button
                        type="button"
                        className="composer__button"
                        onClick={retry}
                    >
                        Повторить
                    </button>
                </div>
            )}

            {busy && (
                <div className="chat__actions">
                    <button
                        type="button"
                        className="composer__button"
                        onClick={stop}
                    >
                        Остановить
                    </button>
                </div>
            )}

            <Composer onSend={send} disabled={busy} />
        </section>
    );
}
