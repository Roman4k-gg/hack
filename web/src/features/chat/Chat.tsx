import { useState } from 'react';
import type { ChatMessage } from './types';
import { MessageList } from './components/MessageList';
import { Composer } from './components/Composer';
import { StreamingIndicator } from './components/StreamingIndicator';
import './chat.css';

const initialMessages: ChatMessage[] = [
    {
        id: '1',
        role: 'assistant',
        content: 'Привет! Я AI-buddy. Чем могу помочь?',
    },
];

export function Chat() {
    const [messages, setMessages] = useState<ChatMessage[]>(initialMessages);

    function handleSend(content: string) {
        const newMessage: ChatMessage = {
            id: crypto.randomUUID(),
            role: 'user',
            content,
        };

        setMessages((previous) => [...previous, newMessage]);
    }

    return (
        <section className="chat">
            <MessageList messages={messages} />
            <Composer onSend={handleSend} />
        </section>
    );
}