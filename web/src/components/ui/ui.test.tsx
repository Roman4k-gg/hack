import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { EmptyState } from './EmptyState';
import { Skeleton } from './Skeleton';
import { StatusBadge } from './StatusBadge';

describe('EmptyState', () => {
    it('показывает заголовок, текст и действие', () => {
        render(
            <EmptyState
                title="Заявок пока нет"
                text="Создай заявку — она появится здесь"
                action={<button type="button">Создать</button>}
            />,
        );

        expect(screen.getByText('Заявок пока нет')).toBeInTheDocument();
        expect(screen.getByText('Создай заявку — она появится здесь')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Создать' })).toBeInTheDocument();
    });

    it('работает без текста и действия', () => {
        render(<EmptyState title="Пока пусто" />);

        expect(screen.getByText('Пока пусто')).toBeInTheDocument();
    });
});

describe('Skeleton', () => {
    it('отрисовывает заглушку с классами', () => {
        const { container } = render(<Skeleton className="h-4 w-24" />);

        const skeleton = container.firstElementChild;

        expect(skeleton).toHaveClass('animate-pulse');
        expect(skeleton).toHaveClass('h-4');
        expect(skeleton).toHaveClass('w-24');
    });
});

describe('StatusBadge', () => {
    it('переводит статусы на русский', () => {
        const cases: Record<string, string> = {
            pending: 'Не начато',
            in_progress: 'В процессе',
            done: 'Готово',
            blocked: 'Заблокировано',
            skipped: 'Пропущено',
        };

        for (const [status, label] of Object.entries(cases)) {
            const { unmount } = render(<StatusBadge status={status as never} />);

            expect(screen.getByText(label)).toBeInTheDocument();

            unmount();
        }
    });
});
