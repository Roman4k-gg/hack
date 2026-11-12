import { cn } from '@/lib/utils';
import type { ItemStatus } from '@/features/onboarding/types';

const statusView: Record<ItemStatus, { label: string; className: string }> = {
  pending: { label: 'Не начато', className: 'bg-mist' },
  in_progress: { label: 'В процессе', className: 'bg-azure-soft' },
  done: { label: 'Готово', className: 'bg-brand-soft' },
  blocked: { label: 'Заблокировано', className: 'bg-alert-soft' },
  skipped: { label: 'Пропущено', className: 'bg-iris-soft' },
};

export function StatusBadge({ status }: { status: ItemStatus }) {
  const view = statusView[status];

  return (
    <span className={cn('inline-block px-2 py-0.5 text-[11px] font-bold', view.className)}>
      {view.label}
    </span>
  );
}