import { beforeEach, describe, expect, it } from 'vitest';
import { useAuthStore } from './authStore';
import { useChatStore } from '@/features/chat/chatStore';

beforeEach(() => {
    useAuthStore.setState({ token: null, user: null });
    useChatStore.setState({
        messages: [],
        status: 'idle',
        error: null,
        activeMessageId: null,
    });
});

const session = {
    token: 'token-mentor',
    token_type: 'Bearer',
    expires_at: '2026-11-15T20:00:00Z',
    user: {
        id: 'mentor-1',
        email: 'petr@demo.local',
        full_name: 'Пётр Иванов',
        role: 'mentor' as const,
        team_name: 'Search',
        scopes: ['onboarding:team:read'],
    },
};

describe('смена пользователя очищает историю чата', () => {
    it('вход стирает переписку предыдущего пользователя', () => {
        useChatStore.getState().sendMessage('Первый вопрос');
        expect(useChatStore.getState().messages).toHaveLength(2);

        useAuthStore.getState().setSession(session);

        expect(useChatStore.getState().messages).toEqual([]);
        expect(useChatStore.getState().activeMessageId).toBeNull();
    });

    it('выход стирает переписку', () => {
        useAuthStore.getState().setSession(session);
        useChatStore.getState().sendMessage('Вопрос про Jira');
        expect(useChatStore.getState().messages).toHaveLength(2);

        useAuthStore.getState().logout();

        expect(useChatStore.getState().messages).toEqual([]);
        expect(useChatStore.getState().status).toBe('idle');
    });

    it('вход другого профиля не показывает чужую историю', () => {
        useAuthStore.getState().setSession(session);
        useChatStore.getState().sendMessage('Вопрос от наставника');

        useAuthStore.getState().setSession({
            ...session,
            token: 'token-intern',
            user: {
                ...session.user,
                id: 'intern-1',
                email: 'dasha@demo.local',
                full_name: 'Дарья Соколова',
                role: 'intern' as const,
            },
        });

        expect(useChatStore.getState().messages).toEqual([]);
    });
});
