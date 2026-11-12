export type ChatRole = 'user' | 'assistant';

export interface ChatCitation {
    id: string;
    title: string;
    url?: string;
}

export interface ChatToolCall {
    id: string;
    name: string;
    status: 'running' | 'success' | 'error';
}

export interface ChatMessage {
    id: string;
    role: ChatRole;
    content: string;
    citations?: ChatCitation[];
    toolCalls?: ChatToolCall[];
}