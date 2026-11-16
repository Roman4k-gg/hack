import type { ChatMessage, ChatToolCall } from '../types';

interface AnswerDetailsProps {
    message: ChatMessage;
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

export function AnswerDetails({ message }: AnswerDetailsProps) {
    const tools = message.toolCalls ?? [];
    const citations = message.citations ?? [];

    return (
        <details className="answer-details">
            <summary className="answer-details__summary">
                Почему такой ответ?
            </summary>

            <div className="answer-details__body">
                <p className="answer-details__intro">
                    Здесь показаны источники и действия, переданные вместе
                    с этим ответом.
                </p>

                <h3>Действия</h3>

                {tools.length > 0 ? (
                    <ul className="answer-details__list">
                        {tools.map((tool) => (
                            <li key={tool.id}>
                                <span>
                                    {toolLabels[tool.name] ?? tool.name}
                                </span>
                                <span
                                    className={`answer-details__status answer-details__status--${tool.status}`}
                                >
                                    {statusLabels[tool.status]}
                                </span>
                            </li>
                        ))}
                    </ul>
                ) : (
                    <p>Для этого ответа сведения о действиях не переданы.</p>
                )}

                <h3>Источники</h3>

                {citations.length > 0 ? (
                    <ul className="answer-details__sources">
                        {citations.map((citation) => (
                            <li key={citation.id}>
                                <span>{citation.title}</span>
                                <small>
                                    {citation.url
                                        ? 'Ссылка доступна под ответом.'
                                        : 'Ссылка на документ не передана.'}
                                </small>
                            </li>
                        ))}
                    </ul>
                ) : (
                    <p>
                        Источники не переданы. Уточни важные сведения
                        у наставника.
                    </p>
                )}
            </div>
        </details>
    );
}