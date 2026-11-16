import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { CitationChip } from './CitationChip';
import { ToolCallCard } from './ToolCallCard';

describe('ToolCallCard', () => {
    it('показывает человеческое имя инструмента', () => {
        render(
            <ToolCallCard
                toolCall={{ id: 'call-1', name: 'get_onboarding_plan', status: 'running' }}
            />,
        );

        expect(screen.getByText('Получение плана адаптации')).toBeInTheDocument();
        expect(screen.getByText('Выполняется')).toBeInTheDocument();
    });

    it('перечисляет статусы выполнения', () => {
        const { rerender } = render(
            <ToolCallCard toolCall={{ id: 'call-1', name: 'get_my_profile', status: 'running' }} />,
        );

        rerender(
            <ToolCallCard toolCall={{ id: 'call-1', name: 'get_my_profile', status: 'success' }} />,
        );

        expect(screen.getByText('Получение профиля стажёра')).toBeInTheDocument();
        expect(screen.getByText('Выполнено')).toBeInTheDocument();

        rerender(
            <ToolCallCard toolCall={{ id: 'call-1', name: 'get_my_profile', status: 'error' }} />,
        );

        expect(screen.getByText('Ошибка или остановка')).toBeInTheDocument();
    });

    it('для неизвестного инструмента показывает общий заголовок', () => {
        render(
            <ToolCallCard toolCall={{ id: 'call-2', name: 'unknown_tool', status: 'running' }} />,
        );

        expect(screen.getByText('Обработка запроса')).toBeInTheDocument();
    });
});

describe('CitationChip', () => {
    it('делает цитату ссылкой с безопасными атрибутами', () => {
        render(
            <CitationChip
                citation={{ id: 'doc-1', title: 'План первой недели', url: 'https://demo.local/docs/plan' }}
            />,
        );

        const link = screen.getByRole('link', { name: 'План первой недели' });

        expect(link).toHaveAttribute('href', 'https://demo.local/docs/plan');
        expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    });

    it('без ссылки показывает просто текст', () => {
        render(<CitationChip citation={{ id: 'doc-2', title: 'Правила отпуска' }} />);

        expect(screen.getByText('Правила отпуска')).toBeInTheDocument();
        expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });
});
