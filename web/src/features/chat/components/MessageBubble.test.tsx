import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { MessageBubble } from './MessageBubble';
import type { ChatMessage } from '../types';

const assistantMessage: ChatMessage = {
    id: 'msg-1',
    role: 'assistant',
    content: 'Познакомься с **наставником** и проверь доступы.',
    toolCalls: [{ id: 'call-1', name: 'get_onboarding_plan', status: 'success' }],
    citations: [{ id: 'doc-1', title: 'План первой недели', url: 'https://demo.local/docs/plan' }],
};

describe('MessageBubble', () => {
    it('рендерит текст ответа с markdown', () => {
        render(<MessageBubble message={assistantMessage} />);

        expect(screen.getByText('наставником').tagName).toBe('STRONG');
    });

    it('показывает карточку вызова инструмента', () => {
        const { container } = render(<MessageBubble message={assistantMessage} />);

        expect(container.querySelector('.tool-call-card__name')).toHaveTextContent(
            'Получение плана адаптации',
        );
        expect(container.querySelector('.tool-call-card__status')).toHaveTextContent(
            'Выполнено',
        );
    });

    it('показывает цитату со ссылкой на источник', () => {
        render(<MessageBubble message={assistantMessage} />);

        const link = screen.getByRole('link', { name: 'План первой недели' });

        expect(link).toHaveAttribute('href', 'https://demo.local/docs/plan');
        expect(link).toHaveAttribute('target', '_blank');
    });

    it('рендерит сообщение пользователя без markdown', () => {
        render(
            <MessageBubble
                message={{ id: 'msg-2', role: 'user', content: 'Что делать в первый день?' }}
            />,
        );

        const bubble = screen.getByText('Что делать в первый день?');

        expect(bubble).toBeInTheDocument();
        expect(bubble.closest('.message-bubble')).toHaveClass('message-bubble--user');
    });

    it('не показывает инструменты и цитаты, если их нет', () => {
        render(
            <MessageBubble message={{ id: 'msg-3', role: 'assistant', content: 'Пустой ответ' }} />,
        );

        expect(screen.queryByText('Получение плана адаптации')).not.toBeInTheDocument();
        expect(document.querySelector('.citation-chip')).toBeNull();
    });

    it('показывает блок «почему такой ответ» для ассистента', () => {
        render(<MessageBubble message={assistantMessage} />);

        expect(screen.getByText('Почему такой ответ?')).toBeInTheDocument();
    });
});
