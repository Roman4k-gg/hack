import { useState } from 'react';
import { useChatStore } from './chatStore';
import {
    fetchConversationMessages,
    fetchConversations,
    type Conversation,
} from './history';
import { useChatSession } from './hooks/useChatSession.ts';
import { MessageList } from './components/MessageList';
import { Composer } from './components/Composer';
import { StreamingIndicator } from './components/StreamingIndicator';
import './chat.css';

export function Chat() {
    const messages = useChatStore((state) => state.messages);
    const status = useChatStore((state) => state.status);
    const error = useChatStore((state) => state.error);
    const loadConversation = useChatStore((state) => state.loadConversation);
    const reset = useChatStore((state) => state.reset);
    const { send, stop, retry, mode } = useChatSession();

    const [historyOpen, setHistoryOpen] = useState(false);
    const [conversations, setConversations] = useState<Conversation[]>([]);
    const [historyLoading, setHistoryLoading] = useState(false);

    const busy = status === 'loading' || status === 'streaming';

    const visibleMessages = messages.filter(
        (message) =>
            message.content ||
            message.toolCalls?.length ||
            message.citations?.length,
    );

    async function toggleHistory() {
        if (historyOpen) {
            setHistoryOpen(false);
            return;
        }

        setHistoryOpen(true);
        setHistoryLoading(true);

        try {
            setConversations(await fetchConversations());
        } catch {
            setConversations([]);
        } finally {
            setHistoryLoading(false);
        }
    }

    async function openConversation(conversation: Conversation) {
        try {
            const history = await fetchConversationMessages(conversation.id);

            loadConversation(
                conversation.id,
                history
                    .filter((message) => message.role === 'user' || message.role === 'assistant')
                    .map((message) => ({
                        id: message.id,
                        role: message.role as 'user' | 'assistant',
                        content: message.content,
                        toolCalls: message.tool_calls.map((tool) => ({
                            id: tool.id,
                            name: tool.name,
                            status:
                                tool.status === 'running' || tool.status === 'success'
                                    ? tool.status
                                    : 'error',
                        })),
                        citations: message.citations.map((citation) => ({
                            id: citation.id,
                            title: citation.title,
                            url: citation.url,
                        })),
                    })),
            );

            setHistoryOpen(false);
        } catch {
            setHistoryOpen(false);
        }
    }

    function startNewConversation() {
        reset();
        setHistoryOpen(false);
    }

    return (
        <section className="chat">
            <div className="chat__toolbar">
                <button
                    type="button"
                    className="composer__button"
                    onClick={() => void toggleHistory()}
                >
                    {historyOpen ? 'Скрыть историю' : 'История диалогов'}
                </button>
                <button
                    type="button"
                    className="composer__button"
                    onClick={startNewConversation}
                >
                    Новый диалог
                </button>
            </div>

            {historyOpen && (
                <div className="chat__history">
                    {historyLoading && (
                        <p className="chat__history-empty">Загружаем историю…</p>
                    )}

                    {!historyLoading && conversations.length === 0 && (
                        <p className="chat__history-empty">
                            Пока нет сохранённых диалогов
                        </p>
                    )}

                    {conversations.map((conversation) => (
                        <button
                            key={conversation.id}
                            type="button"
                            className="chat__history-item"
                            onClick={() => void openConversation(conversation)}
                        >
                            <span className="chat__history-title">
                                {conversation.title || 'Без названия'}
                            </span>
                            <span className="chat__history-meta">
                                {conversation.message_count} сообщений
                            </span>
                        </button>
                    ))}
                </div>
            )}

            {mode === 'demo' && (
                <div className="chat__notice" role="status">
                    AI-сервис недоступен — показан демо-режим без обращения к
                    бэкенду.
                </div>
            )}

            {messages.length === 0 ? (
                <div className="chat__empty">
                    <h2>С чего начнём?</h2>
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

            <Composer onSend={send} onStop={stop} disabled={busy} />
        </section>
    );
}
