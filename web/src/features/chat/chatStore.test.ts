import { beforeEach, describe, expect, it } from 'vitest';
import { useChatStore } from './chatStore';

beforeEach(() => {
    useChatStore.setState({
        messages: [],
        status: 'idle',
        error: null,
        activeMessageId: null,
    });
});

describe('chatStore', () => {
    it('не создаёт сообщения для пустого вопроса', () => {
        useChatStore.getState().sendMessage('   ');

        expect(useChatStore.getState().messages).toEqual([]);
        expect(useChatStore.getState().status).toBe('idle');
    });

    it('создаёт вопрос и ответ, блокируя повторную отправку', () => {
        useChatStore.getState().sendMessage('  Что делать в первый день?  ');
        useChatStore.getState().sendMessage('Второй вопрос');

        const state = useChatStore.getState();

        expect(state.messages).toHaveLength(2);
        expect(state.messages[0]).toMatchObject({
            role: 'user',
            content: 'Что делать в первый день?',
        });
        expect(state.messages[1]).toMatchObject({
            role: 'assistant',
            content: '',
        });
        expect(state.messages[0].id).not.toBe(state.messages[1].id);
        expect(state.activeMessageId).toBe(state.messages[1].id);
        expect(state.status).toBe('loading');
    });

    it('собирает ответ из фрагментов и завершает поток', () => {
        useChatStore.getState().sendMessage('Мой план');

        useChatStore.getState().applyEvent({
            type: 'delta',
            text: 'Познакомься ',
        });
        useChatStore.getState().applyEvent({
            type: 'delta',
            text: 'с наставником.',
        });

        expect(useChatStore.getState().status).toBe('streaming');
        expect(useChatStore.getState().messages[1].content)
            .toBe('Познакомься с наставником.');

        useChatStore.getState().applyEvent({ type: 'done' });
        useChatStore.getState().applyEvent({
            type: 'delta',
            text: 'Лишний фрагмент',
        });

        expect(useChatStore.getState().status).toBe('idle');
        expect(useChatStore.getState().activeMessageId).toBeNull();
        expect(useChatStore.getState().messages[1].content)
            .toBe('Познакомься с наставником.');
    });

    it('обновляет инструмент и источник без дубликатов', () => {
        useChatStore.getState().sendMessage('Покажи план');

        for (const status of ['running', 'success'] as const) {
            useChatStore.getState().applyEvent({
                type: 'tool_call',
                toolCall: {
                    id: 'tool-1',
                    name: 'get_onboarding_plan',
                    status,
                },
            });
        }

        for (const title of ['План', 'План первой недели']) {
            useChatStore.getState().applyEvent({
                type: 'citation',
                citation: {
                    id: 'source-1',
                    title,
                },
            });
        }

        const message = useChatStore.getState().messages[1];

        expect(message.toolCalls).toEqual([
            {
                id: 'tool-1',
                name: 'get_onboarding_plan',
                status: 'success',
            },
        ]);
        expect(message.citations).toEqual([
            {
                id: 'source-1',
                title: 'План первой недели',
            },
        ]);
    });

    it('сохраняет частичный ответ при ошибке', () => {
        useChatStore.getState().sendMessage('Покажи план');
        useChatStore.getState().applyEvent({
            type: 'delta',
            text: 'Первый шаг',
        });
        useChatStore.getState().applyEvent({
            type: 'error',
            message: 'Сервис недоступен',
        });

        const state = useChatStore.getState();

        expect(state.status).toBe('error');
        expect(state.error).toBe('Сервис недоступен');
        expect(state.activeMessageId).toBeNull();
        expect(state.messages[1].content).toBe('Первый шаг');
    });

    it('после остановки игнорирует запоздалые события', () => {
        useChatStore.getState().sendMessage('Покажи план');
        useChatStore.getState().applyEvent({
            type: 'delta',
            text: 'Первый шаг',
        });

        useChatStore.getState().finish();
        useChatStore.getState().applyEvent({
            type: 'delta',
            text: 'Запоздалый текст',
        });

        expect(useChatStore.getState().status).toBe('idle');
        expect(useChatStore.getState().activeMessageId).toBeNull();
        expect(useChatStore.getState().messages[1].content).toBe('Первый шаг');
    });

    it('показывает сетевую ошибку и очищает её при новой отправке', () => {
        useChatStore.getState().sendMessage('Покажи план');
        useChatStore.getState().fail('Соединение потеряно');

        expect(useChatStore.getState().status).toBe('error');
        expect(useChatStore.getState().error).toBe('Соединение потеряно');
        expect(useChatStore.getState().activeMessageId).toBeNull();

        useChatStore.getState().sendMessage('Как получить доступ?');

        expect(useChatStore.getState().status).toBe('loading');
        expect(useChatStore.getState().error).toBeNull();
    });
});
describe('сброс истории при смене пользователя', () => {
    it('очищает сообщения и статус', () => {
        useChatStore.getState().sendMessage('Где взять доступ к Jira?');
        useChatStore.getState().applyEvent({
            type: 'delta',
            text: 'Спроси наставника',
        });

        expect(useChatStore.getState().messages.length).toBeGreaterThan(0);

        useChatStore.getState().reset();

        const state = useChatStore.getState();

        expect(state.messages).toEqual([]);
        expect(state.status).toBe('idle');
        expect(state.error).toBeNull();
        expect(state.activeMessageId).toBeNull();
    });

    it('сбрасывает и ошибку стрима', () => {
        useChatStore.getState().fail('Ассистент недоступен');
        expect(useChatStore.getState().status).toBe('error');

        useChatStore.getState().reset();

        expect(useChatStore.getState().status).toBe('idle');
        expect(useChatStore.getState().error).toBeNull();
    });
});
