import { describe, expect, it } from 'vitest';
import { parseChatEvent } from './schemas';

describe('parseChatEvent', () => {
    it('приводит тип из имени события', () => {
        const event = parseChatEvent({
            event: 'delta',
            data: JSON.stringify({ text: 'Твой следующий шаг — ' }),
        });

        expect(event).toEqual({
            type: 'delta',
            text: 'Твой следующий шаг — ',
        });
    });

    it('берёт тип из data, если он есть', () => {
        const event = parseChatEvent({
            event: 'message',
            data: JSON.stringify({ type: 'citation', citation: { id: 'doc-1', title: 'Доступ к Jira' } }),
        });

        expect(event.type).toBe('citation');
    });

    it('разбирает событие вызова инструмента', () => {
        const event = parseChatEvent({
            event: 'message',
            data: JSON.stringify({
                type: 'tool_call',
                toolCall: { id: 'call-1', name: 'get_onboarding_plan', status: 'running' },
            }),
        });

        expect(event).toMatchObject({
            type: 'tool_call',
            toolCall: { name: 'get_onboarding_plan', status: 'running' },
        });
    });

    it('разбирает завершение и ошибку', () => {
        expect(parseChatEvent({ event: 'message', data: '{"type":"done"}' })).toEqual({
            type: 'done',
        });

        expect(
            parseChatEvent({
                event: 'message',
                data: '{"type":"error","message":"Ассистент недоступен"}',
            }),
        ).toEqual({ type: 'error', message: 'Ассистент недоступен' });
    });

    it('падает на невалидном JSON', () => {
        expect(() => parseChatEvent({ event: 'message', data: '{не json' })).toThrow();
    });

    it('падает на неизвестном типе', () => {
        expect(() =>
            parseChatEvent({ event: 'message', data: '{"type":"unknown"}' }),
        ).toThrow();
    });

    it('падает на событии без типа', () => {
        expect(() => parseChatEvent({ event: 'message', data: '{}' })).toThrow();
    });

    it('падает, если текст не строка', () => {
        expect(() =>
            parseChatEvent({ event: 'message', data: '{"type":"delta","text":42}' }),
        ).toThrow();
    });

    it('падает, если у цитаты нет заголовка', () => {
        expect(() =>
            parseChatEvent({
                event: 'message',
                data: '{"type":"citation","citation":{"id":"doc-1"}}',
            }),
        ).toThrow();
    });
});
