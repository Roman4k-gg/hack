import { z } from 'zod';
import type { SSEEvent } from '../../lib/sse/parseSSE';

export const citationSchema = z.object({
    id: z.string(),
    title: z.string(),
    url: z.string().optional(),
});

export const toolCallSchema = z.object({
    id: z.string(),
    name: z.string(),
    status: z.enum(['running', 'success', 'error']),
});

export const chatEventSchema = z.discriminatedUnion('type', [
    z.object({
        type: z.literal('delta'),
        text: z.string(),
    }),
    z.object({
        type: z.literal('citation'),
        citation: citationSchema,
    }),
    z.object({
        type: z.literal('tool_call'),
        toolCall: toolCallSchema,
    }),
    z.object({
        type: z.literal('done'),
    }),
    z.object({
        type: z.literal('error'),
        message: z.string(),
    }),
]);

export type ChatEvent = z.infer<typeof chatEventSchema>;

export function parseChatEvent(event: SSEEvent): ChatEvent {
    const data: unknown = JSON.parse(event.data);

    if (typeof data !== 'object' || data === null || Array.isArray(data)) {
        throw new Error('Некорректное событие чата');
    }

    return chatEventSchema.parse({
        ...data,
        type: event.event === 'message'
            ? ('type' in data ? data.type : undefined)
            : event.event,
    });
}