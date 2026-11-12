import { useEffect, useRef } from 'react';
import { useChatStore } from '../chatStore';
import { parseChatEvent } from '../schemas';
import type { ChatEvent } from '../schemas';

function getScenario(question: string) {
    const text = question.toLowerCase();

    if (/доступ|jira|vpn/.test(text)) {
        return {
            tool: 'search_knowledge',
            citation: {
                id: 'demo-access',
                title: 'Демо-инструкция: рабочие доступы',
            },
            answer: [
                '## Как получить рабочие доступы',
                '',
                'Для старта тебе понадобятся **VPN, Jira и база знаний**.',
                '',
                '1. Уточни у наставника, какие системы нужны твоей команде.',
                '2. Подготовь рабочую почту и название команды.',
                '3. Создай заявку в службу поддержки.',
                '',
                'В заявку можно вставить такой текст:',
                '',
                '> Я прохожу стажировку. Прошу выдать доступ к Jira и базе знаний моей команды. Наставник: укажи имя.',
                '',
                '**Не передавай пароль или коды подтверждения в чат.**',
                '',
                'Это демонстрационный сценарий: реальные доступы выдаются по правилам компании.',
            ].join('\n'),
        };
    }

    if (/запуск|проект|код|репозитор/.test(text)) {
        return {
            tool: 'search_knowledge',
            citation: {
                id: 'demo-project',
                title: 'Демо-инструкция: запуск учебного проекта',
            },
            answer: [
                '## Первый запуск учебного проекта',
                '',
                'Перед запуском попроси наставника предоставить **ссылку на репозиторий** и доступ к нему.',
                '',
                'Для нашего демо AI-buddy открой PowerShell и выполни:',
                '',
                '```powershell',
                'cd D:\\Desktop\\AI-buddy\\hack\\web',
                'npm.cmd install',
                'npm.cmd run dev',
                '```',
                '',
                'Открой адрес из строки `Local` в терминале.',
                '',
                '### Если запуск не получился',
                '',
                '- Проверь, что терминал открыт в папке `web`.',
                '- Скопируй текст ошибки.',
                '- Отправь его наставнику вместе с описанием своих действий.',
                '',
                '> Команды выше относятся к демо AI-buddy. Для рабочего проекта используй его README.',
            ].join('\n'),
        };
    }

    return {
        tool: 'get_onboarding_plan',
        citation: {
            id: 'demo-onboarding',
            title: 'Демо-план: первая неделя стажировки',
        },
        answer: [
            '## Твой первый день',
            '',
            'Привет! Начнём с **трёх небольших шагов**.',
            '',
            '1. Познакомься с наставником и договорись о первой встрече.',
            '2. Проверь доступы к рабочей почте, Jira и базе знаний.',
            '3. Открой карту адаптации и выбери первую задачу.',
            '',
            '### Что обсудить с наставником',
            '',
            '- Чем занимается команда?',
            '- Какой результат ожидается к концу первой недели?',
            '- К кому обращаться, если задача заблокирована?',
            '',
            '> Необязательно разобраться во всём за один день. Записывай вопросы и обсуждай их с наставником.',
            '',
            '**Следующий шаг:** назначь знакомство с наставником.',
            '',
            'Это пример плана для демо. Персональный план появится после подключения данных.',
        ].join('\n'),
    };
}

export function useMockChat() {
    const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
    const version = useRef(0);

    function cancelTimer() {
        version.current += 1;

        if (timer.current !== null) {
            clearTimeout(timer.current);
            timer.current = null;
        }
    }

    function closeRunningTools() {
        const state = useChatStore.getState();
        const activeId = state.activeMessageId;

        useChatStore.setState({
            messages: state.messages.map((message) =>
                message.id !== activeId
                    ? message
                    : {
                        ...message,
                        toolCalls: message.toolCalls?.map((tool) =>
                            tool.status === 'running'
                                ? { ...tool, status: 'error' as const }
                                : tool,
                        ),
                    },
            ),
        });
    }

    function stop() {
        cancelTimer();
        closeRunningTools();
        useChatStore.getState().finish();
    }

    function run(content: string, isRetry = false) {
        const text = content.trim();
        const store = useChatStore.getState();

        if (!text || store.activeMessageId) return;

        cancelTimer();

        if (isRetry) {
            const index = store.messages
                .map((message) => message.role)
                .lastIndexOf('user');

            if (index < 0) return;

            useChatStore.setState({
                messages: store.messages.slice(0, index),
            });
        }

        useChatStore.getState().sendMessage(text);

        const currentVersion = version.current;
        const shouldFail = text.toLowerCase().includes('ошибка') && !isRetry;
        const scenario = getScenario(text);
        const toolId = crypto.randomUUID();
        const chunks = scenario.answer.match(/[\s\S]{1,12}/g) ?? [];

        const events: ChatEvent[] = [
            {
                type: 'tool_call',
                toolCall: {
                    id: toolId,
                    name: scenario.tool,
                    status: 'running',
                },
            },
            {
                type: 'tool_call',
                toolCall: {
                    id: toolId,
                    name: scenario.tool,
                    status: shouldFail ? 'error' : 'success',
                },
            },
        ];

        if (shouldFail) {
            events.push({
                type: 'error',
                message: 'Не удалось загрузить данные адаптации. Попробуй ещё раз.',
            });
        } else {
            events.push(
                ...chunks.map((text): ChatEvent => ({
                    type: 'delta',
                    text,
                })),
                {
                    type: 'citation',
                    citation: scenario.citation,
                },
                { type: 'done' },
            );
        }

        let position = 0;

        function tick() {
            if (version.current !== currentVersion) return;

            timer.current = null;
            const event = events[position];

            if (!event) return;

            try {
                const parsed = parseChatEvent({
                    event: event.type,
                    data: JSON.stringify(event),
                });

                useChatStore.getState().applyEvent(parsed);
                position += 1;

                if (event.type !== 'done' && event.type !== 'error') {
                    timer.current = setTimeout(
                        tick,
                        event.type === 'tool_call' ? 700 : 65,
                    );
                }
            } catch (error) {
                closeRunningTools();
                useChatStore.getState().fail(
                    error instanceof Error
                        ? error.message
                        : 'Не удалось получить ответ AI-buddy',
                );
            }
        }

        timer.current = setTimeout(tick, 900);
    }

    function retry() {
        const state = useChatStore.getState();

        if (state.status !== 'error') return;

        const question = [...state.messages]
            .reverse()
            .find((message) => message.role === 'user');

        if (question) run(question.content, true);
    }

    useEffect(() => {
        return () => {
            version.current += 1;

            if (timer.current !== null) {
                clearTimeout(timer.current);
            }

            closeRunningTools();
            useChatStore.getState().finish();
        };
    }, []);

    return {
        send: (content: string) => run(content),
        stop,
        retry,
    };
}