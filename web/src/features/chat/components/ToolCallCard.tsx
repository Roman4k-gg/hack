import type { ChatToolCall } from '../types';

interface ToolCallCardProps {
    toolCall: ChatToolCall;
}

const toolLabels: Record<string, string> = {
    get_onboarding_plan: 'Получение плана адаптации',
    get_my_profile: 'Получение профиля стажёра',
    search_knowledge: 'Поиск в базе знаний',
    preview_absence_request: 'Черновик заявки на отсутствие',
    create_absence_request: 'Создание заявки на отсутствие',
    get_my_requests: 'Получение моих заявок',
    preview_access_bundle: 'Черновик заявок на доступы',
    create_access_requests: 'Создание заявок на доступы',
    preview_handoff: 'Черновик вопроса наставнику',
    create_handoff: 'Отправка вопроса наставнику',
};

const statusLabels: Record<ChatToolCall['status'], string> = {
    running: 'Выполняется',
    success: 'Выполнено',
    error: 'Ошибка или остановка',
};

export function ToolCallCard({ toolCall }: ToolCallCardProps) {
    return (
        <div className="tool-call-card">
            <span className="tool-call-card__name">
                {toolLabels[toolCall.name] ?? 'Обработка запроса'}
            </span>
            <span
                className={`tool-call-card__status tool-call-card__status--${toolCall.status}`}
            >
                {statusLabels[toolCall.status]}
            </span>
        </div>
    );
}