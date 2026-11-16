import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useChatStore } from './chatStore';
import {
    fetchConversationMessages,
    fetchConversations,
    type Conversation,
} from './history';

const fetchMock = vi.fn();

beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
    useChatStore.setState({
        messages: [],
        status: 'idle',
        error: null,
        activeMessageId: null,
        conversationId: null,
    });
});

afterEach(() => {
    vi.unstubAllGlobals();
});

function response(status: number, body: unknown): Response {
    return {
        ok: status >= 200 && status < 300,
        status,
        text: async () => JSON.stringify(body),
    } as Response;
}

const conversation: Conversation = {
    id: 'conv-1',
    title: 'Где взять доступ к Jira?',
    created_at: '2026-11-16T10:00:00Z',
    updated_at: '2026-11-16T10:05:00Z',
    message_count: 2,
};

describe('история диалогов', () => {
    it('загружает список диалогов', async () => {
        fetchMock.mockResolvedValue(response(200, [conversation]));

        const conversations = await fetchConversations();

        expect(conversations).toHaveLength(1);
        expect(conversations[0].title).toBe('Где взять доступ к Jira?');
    });

    it('понимает ответ в виде объекта с items', async () => {
        fetchMock.mockResolvedValue(response(200, { items: [conversation] }));

        const conversations = await fetchConversations();

        expect(conversations).toHaveLength(1);
    });

    it('загружает сообщения диалога с инструментами и цитатами', async () => {
        fetchMock.mockResolvedValue(
            response(200, [
                {
                    id: 'msg-1',
                    role: 'user',
                    content: 'Где взять доступ к Jira?',
                    tool_calls: [],
                    citations: [],
                    created_at: '2026-11-16T10:00:00Z',
                },
                {
                    id: 'msg-2',
                    role: 'assistant',
                    content: 'Спроси наставника',
                    tool_calls: [
                        { id: 'call-1', name: 'get_onboarding_plan', status: 'success' },
                    ],
                    citations: [
                        { id: 'doc-1', title: 'Доступ к Jira', url: 'https://demo.local/docs/jira-access' },
                    ],
                    created_at: '2026-11-16T10:00:05Z',
                },
            ]),
        );

        const messages = await fetchConversationMessages('conv-1');

        expect(messages).toHaveLength(2);
        expect(messages[1].tool_calls[0].name).toBe('get_onboarding_plan');
        expect(messages[1].citations[0].title).toBe('Доступ к Jira');
    });

    it('не подставляет мусорные данные', async () => {
        fetchMock.mockResolvedValue(response(200, [{ id: 'conv-1' }]));

        await expect(fetchConversations()).rejects.toThrow();
    });
});

describe('стор чата', () => {
    it('запоминает идентификатор диалога', () => {
        useChatStore.getState().setConversationId('conv-1');

        expect(useChatStore.getState().conversationId).toBe('conv-1');
    });

    it('загружает переписку выбранного диалога', () => {
        useChatStore.getState().loadConversation('conv-1', [
            { id: 'msg-1', role: 'user', content: 'Вопрос' },
            { id: 'msg-2', role: 'assistant', content: 'Ответ' },
        ]);

        const state = useChatStore.getState();

        expect(state.conversationId).toBe('conv-1');
        expect(state.messages).toHaveLength(2);
        expect(state.status).toBe('idle');
    });

    it('сбрасывает переписку и диалог при новом чате', () => {
        useChatStore.getState().loadConversation('conv-1', [
            { id: 'msg-1', role: 'user', content: 'Вопрос' },
        ]);
        useChatStore.getState().reset();

        const state = useChatStore.getState();

        expect(state.messages).toEqual([]);
        expect(state.conversationId).toBeNull();
    });
});
