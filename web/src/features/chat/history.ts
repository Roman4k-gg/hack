import { z } from 'zod';
import { apiFetch } from '@/lib/api';

export const conversationSchema = z.object({
    id: z.string(),
    title: z.string(),
    created_at: z.string(),
    updated_at: z.string(),
    message_count: z.number(),
});

export const historyMessageSchema = z.object({
    id: z.string(),
    role: z.enum(['user', 'assistant', 'system', 'tool']),
    content: z.string(),
    tool_calls: z
        .array(
            z.object({
                id: z.string(),
                name: z.string(),
                status: z.string(),
            }),
        )
        .default([]),
    citations: z
        .array(
            z.object({
                id: z.string(),
                title: z.string(),
                url: z.string().optional(),
            }),
        )
        .default([]),
    created_at: z.string(),
});

export type Conversation = z.infer<typeof conversationSchema>;
export type HistoryMessage = z.infer<typeof historyMessageSchema>;

export async function fetchConversations(): Promise<Conversation[]> {
    const payload = await apiFetch<unknown>('/conversations');
    const items = Array.isArray(payload)
        ? payload
        : ((payload as { items?: unknown[] })?.items ?? []);

    return z.array(conversationSchema).parse(items);
}

export async function fetchConversationMessages(
    conversationId: string,
): Promise<HistoryMessage[]> {
    const payload = await apiFetch<unknown>(
        `/conversations/${conversationId}/messages`,
    );
    const items = Array.isArray(payload)
        ? payload
        : ((payload as { items?: unknown[] })?.items ?? []);

    return z.array(historyMessageSchema).parse(items);
}
