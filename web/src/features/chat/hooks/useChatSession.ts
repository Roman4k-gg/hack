import { useCallback, useState } from 'react';
import { useAuthStore } from '@/features/auth/authStore';
import { useChatStore } from '../chatStore';
import { parseChatEvent } from '../schemas';
import { useChatStream } from './useChatStream';
import { useMockChat } from './useMockChat';

const CHAT_URL = '/api/v1/chat/stream';

export type ChatMode = 'live' | 'demo';

export function useChatSession() {
  const token = useAuthStore((state) => state.token);
  const { start, stop: stopStream, error: streamError } = useChatStream();
  const mock = useMockChat();
  const [mode, setMode] = useState<ChatMode>('live');

  const send = useCallback(
    (content: string) => {
      const text = content.trim();

      if (!text) {
        return;
      }

      const store = useChatStore.getState();

      if (store.activeMessageId) {
        return;
      }

      useChatStore.getState().sendMessage(text);

      void start({
        url: CHAT_URL,
        body: { message: text },
        headers: token ? { Authorization: `Bearer ${token}` } : undefined,
        onEvent: (event) => {
          try {
            useChatStore.getState().applyEvent(parseChatEvent(event));
          } catch {
            return;
          }
        },
        onFailure: () => {
          setMode('demo');
          resetToLastQuestion();
          mock.send(text);
        },
      });
    },
    [mock, start, token],
  );

  const retry = useCallback(() => {
    const state = useChatStore.getState();

    if (state.status !== 'error') {
      return;
    }

    const index = state.messages.map((message) => message.role).lastIndexOf('user');

    if (index < 0) {
      return;
    }

    const question = state.messages[index].content;

    useChatStore.setState({
      messages: state.messages.slice(0, index),
      status: 'idle',
      error: null,
      activeMessageId: null,
    });

    send(question);
  }, [send]);

  const stop = useCallback(() => {
    stopStream();
    mock.stop();
  }, [mock, stopStream]);

  return {
    send,
    stop,
    retry,
    mode,
    error: mode === 'demo' ? null : streamError,
  };
}

function resetToLastQuestion() {
  const state = useChatStore.getState();
  const index = state.messages.map((message) => message.role).lastIndexOf('user');

  useChatStore.setState({
    messages: index >= 0 ? state.messages.slice(0, index) : [],
    status: 'idle',
    error: null,
    activeMessageId: null,
  });
}
