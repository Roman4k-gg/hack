import { create } from 'zustand';
import type { ChatMessage } from './types';
import type { ChatEvent } from './schemas';

type ChatStatus = 'idle' | 'loading' | 'streaming' | 'error';

interface ChatState {
    messages: ChatMessage[];
    status: ChatStatus;
    error: string | null;
    activeMessageId: string | null;
    sendMessage: (content: string) => void;
    applyEvent: (event: ChatEvent) => void;
    fail: (message: string) => void;
    finish: () => void;
}

export const useChatStore = create<ChatState>((set) => ({
    messages: [
        {
            id: 'welcome',
            role: 'assistant',
            content: 'Привет! Я AI-buddy. Чем могу помочь?',
        },
    ],
    status: 'idle',
    error: null,
    activeMessageId: null,

    sendMessage: (content) => {
        const text = content.trim();
        if (!text) return;

        set((state) => {
            if (state.activeMessageId) return state;

            const assistantId = crypto.randomUUID();

            return {
                messages: [
                    ...state.messages,
                    {
                        id: crypto.randomUUID(),
                        role: 'user',
                        content: text,
                    },
                    {
                        id: assistantId,
                        role: 'assistant',
                        content: '',
                    },
                ],
                activeMessageId: assistantId,
                status: 'loading',
                error: null,
            };
        });
    },

    applyEvent: (event) => set((state) => {
        if (!state.activeMessageId) return state;

        if (event.type === 'error') {
            return {
                status: 'error',
                error: event.message,
                activeMessageId: null,
            };
        }

        if (event.type === 'done') {
            return { status: 'idle', activeMessageId: null };
        }

        return {
            status: 'streaming',
            messages: state.messages.map((message) => {
                if (message.id !== state.activeMessageId) return message;

                switch (event.type) {
                    case 'delta':
                        return {
                            ...message,
                            content: message.content + event.text,
                        };
                    case 'citation':
                        return {
                            ...message,
                            citations: [
                                ...(message.citations ?? []).filter(
                                    (item) => item.id !== event.citation.id,
                                ),
                                event.citation,
                            ],
                        };
                    case 'tool_call':
                        return {
                            ...message,
                            toolCalls: [
                                ...(message.toolCalls ?? []).filter(
                                    (item) => item.id !== event.toolCall.id,
                                ),
                                event.toolCall,
                            ],
                        };
                }
            }),
        };
    }),

    fail: (message) => set({
        status: 'error',
        error: message,
        activeMessageId: null,
    }),

    finish: () => set({
        status: 'idle',
        activeMessageId: null,
    }),
}));