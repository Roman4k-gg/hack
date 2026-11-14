import { describe, expect, it } from 'vitest';
import { createSSEParser } from './parseSSE';
import type { SSEEvent } from './parseSSE';

describe('SSE parser', () => {
    it('собирает событие из отдельных сетевых фрагментов', () => {
        const events: SSEEvent[] = [];
        const parser = createSSEParser((event) => events.push(event));

        parser.feed('event: del');
        parser.feed('ta\ndata: {"text":"При');
        expect(events).toHaveLength(0);

        parser.feed('вет"}\n\n');

        expect(events).toEqual([
            {
                event: 'delta',
                data: '{"text":"Привет"}',
            },
        ]);
    });

    it('обрабатывает CRLF, разделённый между фрагментами', () => {
        const events: SSEEvent[] = [];
        const parser = createSSEParser((event) => events.push(event));

        parser.feed('event: delta\r');
        parser.feed('\ndata: {"text":"План"}\r');
        parser.feed('\n\r');
        parser.feed('\n');

        expect(events).toEqual([
            {
                event: 'delta',
                data: '{"text":"План"}',
            },
        ]);
    });

    it('игнорирует комментарии и объединяет строки data', () => {
        const events: SSEEvent[] = [];
        const parser = createSSEParser((event) => events.push(event));

        parser.feed(': heartbeat\nid: 7\ndata: первая\n');
        parser.feed('data: вторая\n\nevent: done\ndata: {}\n\n');

        expect(events).toEqual([
            {
                event: 'message',
                data: 'первая\nвторая',
                id: '7',
            },
            {
                event: 'done',
                data: '{}',
                id: '7',
            },
        ]);
    });

    it('не дублирует события при повторном flush', () => {
        const events: SSEEvent[] = [];
        const parser = createSSEParser((event) => events.push(event));

        parser.feed('event: done\ndata: {}');
        parser.flush();
        parser.flush();

        expect(events).toEqual([
            {
                event: 'done',
                data: '{}',
            },
        ]);
    });
});