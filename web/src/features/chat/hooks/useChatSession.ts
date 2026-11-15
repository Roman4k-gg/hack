import { useState } from 'react';
import { useAuthStore } from '@/features/auth/authStore';
import { useChatStore } from '../chatStore';
import { parseChatEvent } from '../schemas';
import { useChatStream } from './useChatStream';
import { useMockChat } from './useMockChat';

const CHAT_URL = '/api/v1/chat/stream';
const DEMO_STATUSES = new Set([502, 503, 504]);

export type ChatMode = 'live' | 'demo';

export function useChatSession() {
  const token = useAuthStore((state) => state.token);
  const { start, stop: stopStream, error: streamError } = useChatStream();
  const mock = useMockChat();
  const [mode, setMode] = useState<ChatMode>('live');

  function send(content: string) {
    const text = content.trim();
    const store = useChatStore.getState();

    if (!text || store.activeMessageId) return;

    if (import.meta.env.VITE_CHAT_DEMO === 'true') {
      setMode('demo');
      mock.send(text);
      return;
    }

    setMode('live');
    store.sendMessage(text);

    const messageId = useChatStore.getState().activeMessageId;
    let terminal = false;
    let receivedEvent = false;

    function isCurrent() {
      return useChatStore.getState().activeMessageId === messageId;
    }

    void start({
      url: CHAT_URL,
      body: { message: text },
      headers: token
        ? { Authorization: `Bearer ${token}` }
        : undefined,
      onEvent: (event) => {
        if (!isCurrent()) return true;

        let parsed;

        try {
          parsed = parseChatEvent(event);
        } catch {
          throw new Error('Получено некорректное событие ответа');
        }

        receivedEvent = true;
        useChatStore.getState().applyEvent(parsed);

        terminal = parsed.type === 'done' || parsed.type === 'error';
        return terminal;
      },
      onComplete: () => {
        if (!terminal && isCurrent()) {
          failCurrentAnswer(
            'Ответ прервался. Уже полученный текст сохранён. Попробуй ещё раз.',
          );
        }
      },
      onFailure: (error, httpStatus) => {
        if (!isCurrent()) return;

        if (
          !receivedEvent &&
          httpStatus !== undefined &&
          DEMO_STATUSES.has(httpStatus)
        ) {
          resetToLastQuestion();
          setMode('demo');
          mock.send(text);
          return;
        }

        failCurrentAnswer(error.message);
      },
    });
  }

  function retry() {
    const state = useChatStore.getState();

    if (state.status !== 'error') return;

    const index = state.messages
      .map((message) => message.role)
      .lastIndexOf('user');

    if (index < 0) return;

    const question = state.messages[index].content;

    resetToLastQuestion();
    send(question);
  }

  function stop() {
    stopStream();
    mock.stop();
  }

  return {
    send,
    stop,
    retry,
    mode,
    error: mode === 'demo' ? null : streamError,
  };
}

function failCurrentAnswer(message: string) {
  const state = useChatStore.getState();

  useChatStore.setState({
    messages: state.messages.map((item) =>
      item.id !== state.activeMessageId
        ? item
        : {
          ...item,
          toolCalls: item.toolCalls?.map((tool) =>
            tool.status === 'running'
              ? { ...tool, status: 'error' as const }
              : tool,
          ),
        },
    ),
  });

  useChatStore.getState().fail(message);
}

function resetToLastQuestion() {
  const state = useChatStore.getState();
  const index = state.messages
    .map((message) => message.role)
    .lastIndexOf('user');

  useChatStore.setState({
    messages: index >= 0 ? state.messages.slice(0, index) : [],
    status: 'idle',
    error: null,
    activeMessageId: null,
  });
}