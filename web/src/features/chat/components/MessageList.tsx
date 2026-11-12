import { useLayoutEffect, useRef } from 'react';
import type { ChatMessage } from '../types';
import { MessageBubble } from './MessageBubble';

interface MessageListProps {
    messages: ChatMessage[];
}

export function MessageList({ messages }: MessageListProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const followRef = useRef(true);
    const lastUserIdRef = useRef<string | undefined>(undefined);

    useLayoutEffect(() => {
        const container = containerRef.current;
        if (!container) return;

        const lastUserId = [...messages]
            .reverse()
            .find((message) => message.role === 'user')?.id;

        if (lastUserId !== lastUserIdRef.current) {
            followRef.current = true;
            lastUserIdRef.current = lastUserId;
        }

        if (followRef.current) {
            container.scrollTop = container.scrollHeight;
        }
    }, [messages]);

    return (
        <div
            ref={containerRef}
            className="message-list"
            aria-label="История сообщений"
            onScroll={() => {
                const container = containerRef.current;
                if (!container) return;

                const distance =
                    container.scrollHeight -
                    container.scrollTop -
                    container.clientHeight;

                followRef.current = distance < 80;
            }}
        >
            {messages.map((message) => (
                <MessageBubble key={message.id} message={message} />
            ))}
        </div>
    );
}