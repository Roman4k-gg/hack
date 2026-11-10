import type { ChatMessage } from '../types';
import { CitationChip } from './CitationChip';
import { ToolCallCard } from './ToolCallCard';

interface MessageBubbleProps {
    message: ChatMessage;
}

export function MessageBubble({ message }: MessageBubbleProps) {
    const isUser = message.role === 'user';

    return (
        <article
            className={`message-bubble ${isUser ? 'message-bubble--user' : 'message-bubble--assistant'}`}
        >
            <div className="message-bubble__content">
                {message.content}
            </div>

            {message.toolCalls && message.toolCalls.length > 0 && (
                <div className="message-bubble__tools">
                    {message.toolCalls.map((toolCall) => (
                        <ToolCallCard key={toolCall.id} toolCall={toolCall} />
                    ))}
                </div>
            )}

            {message.citations && message.citations.length > 0 && (
                <div className="message-bubble__citations">
                    {message.citations.map((citation) => (
                        <CitationChip key={citation.id} citation={citation} />
                    ))}
                </div>
            )}
        </article>
    );
}