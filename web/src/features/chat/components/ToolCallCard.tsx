
import type { ChatToolCall } from '../types';

interface ToolCallCardProps {
    toolCall: ChatToolCall;
}

const statusLabels: Record<ChatToolCall['status'], string> = {
    running: 'Выполняется',
    success: 'Выполнено',
    error: 'Ошибка',
};

export function ToolCallCard({ toolCall }: ToolCallCardProps) {
    return (
        <div className="tool-call-card">
            <span className="tool-call-card__name">{toolCall.name}</span>
            <span
                className={`tool-call-card__status tool-call-card__status--${toolCall.status}`}
            >
                {statusLabels[toolCall.status]}
            </span>
        </div>
    );
}
