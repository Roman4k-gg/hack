import { ArrowUpRight, MessageCircle } from 'lucide-react';
import { useChatStore } from './chatStore';
import { useChatSession } from './hooks/useChatSession';
import { MessageList } from './components/MessageList';
import { Composer } from './components/Composer';
import { StreamingIndicator } from './components/StreamingIndicator';
import './chat.css';

const suggestions = [
    'Что делать в первый день?',
    'Как получить доступ к Jira?',
    'Как запустить проект?',
];

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
        <section className="chat" aria-label="Чат">
            {mode === 'demo' && (
                <div className="chat__notice" role="status">
                    Демо-режим · ответы на примерах адаптации
                </div>
            )}

            {messages.length === 0 ? (
                <div className="chat__empty">
                    <span className="chat__eyebrow">НАЧНЁМ ЗНАКОМСТВО</span>

                    <div className="chat__welcome-icon" aria-hidden="true">
                        <MessageCircle size={28} strokeWidth={1.5} />
                    </div>

                    <h2>С чего начнём?</h2>
                    <p>
                        Помогу освоиться в команде, разобраться с доступами
                        и найти следующий шаг.
                    </p>

                    <div className="chat__suggestions">
                        {suggestions.map((question) => (
                            <button
                                key={question}
                                type="button"
                                className="chat__suggestion"
                                onClick={() => send(question)}
                                disabled={busy}
                            >
                                <span>{question}</span>
                                <ArrowUpRight size={17} aria-hidden="true" />
                            </button>
                        ))}
                    </div>

                    <p className="chat__footnote">
                        Можно начать с подсказки или написать свой вопрос.
                    </p>
                </div>
            ) : (
                <MessageList messages={visibleMessages} />
            )}

            {status === 'loading' && (
                <div className="streaming-indicator" role="status">
                    Ищу полезное для тебя…
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
                        className="chat__stop"
                        onClick={stop}
                    >
                        <span aria-hidden="true">■</span>
                        Остановить
                        <kbd>Esc</kbd>
                    </button>
                </div>
            )}

            <Composer onSend={send} onStop={stop} disabled={busy} />
        </section>
    );
}