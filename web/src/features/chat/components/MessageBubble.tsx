import Markdown from 'react-markdown';
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
            <div
                className={
                    isUser
                        ? 'message-bubble__content'
                        : 'message-bubble__markdown'
                }
            >
                {isUser ? (
                    message.content
                ) : (
                    <Markdown
                        skipHtml
                        components={{
                            a: ({ href, children }) => (
                                <a
                                    href={href}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    {children}
                                </a>
                            ),
                        }}
                    >
                        {message.content}
                    </Markdown>
                )}
            </div>

            {!!message.toolCalls?.length && (
                <div className="message-bubble__tools">
                    {message.toolCalls.map((toolCall) => (
                        <ToolCallCard key={toolCall.id} toolCall={toolCall} />
                    ))}
                </div>
            )}

            {!!message.citations?.length && (
                <div className="message-bubble__citations">
                    {message.citations.map((citation) => (
                        <CitationChip key={citation.id} citation={citation} />
                    ))}
                </div>
            )}
        </article>
    );
}