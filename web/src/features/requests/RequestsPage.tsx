import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { DemoBadge } from '@/components/ui/DemoBadge';
import { EmptyState } from '@/components/ui/EmptyState';
import { Skeleton } from '@/components/ui/Skeleton';
import { useAuthStore } from '@/features/auth/authStore';
import { cn } from '@/lib/utils';
import { fetchRequests } from './api';
import {
  requestStatusMeta,
  requestTypeLabels,
  type RequestItem,
} from './types';

const rowGrid = 'lg:grid-cols-[8rem_minmax(0,1fr)_9rem_11rem]';

export function RequestsPage() {
  const userId = useAuthStore((state) => state.user?.id);

  const { data, isPending, isError, refetch } = useQuery({
    queryKey: ['requests', userId],
    queryFn: fetchRequests,
  });

  return (
    <section>
      <p className="text-xs text-muted">Заявки</p>
      <div className="mt-1 flex flex-wrap items-center gap-3">
        <h1 className="text-3xl font-extrabold tracking-tight">Мои заявки</h1>
        {data?.demo && <DemoBadge />}
      </div>
      <p className="mt-1 text-sm text-muted">
        Отгулы и доступы, которые оформлены через AI-наставника.
      </p>

      <div className="mt-6 flex flex-col gap-3 bg-azure-soft p-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-sm font-bold">Как создать заявку</p>
          <p className="mt-1 text-sm">
            Напиши в чате, например: «хочу отгул в пятницу». AI-наставник
            покажет черновик и отправит заявку только после твоего
            подтверждения.
          </p>
        </div>
        <Link
          to="/"
          className="w-fit shrink-0 bg-ink px-4 py-2 text-sm font-bold text-paper"
        >
          Открыть чат
        </Link>
      </div>

      <div className="mt-6">
        {isPending && <RequestsSkeleton />}

        {isError && (
          <div>
            <p>Не удалось загрузить заявки. Проверьте подключение к API.</p>
            <button
              className="mt-3 bg-ink px-4 py-2 text-sm font-bold text-paper"
              onClick={() => refetch()}
            >
              Повторить
            </button>
          </div>
        )}

        {data && data.data.length === 0 && (
          <EmptyState
            title="Заявок пока нет"
            text="Когда ты оформишь отгул или доступ через чат, заявка появится здесь со статусом."
          />
        )}

        {data && data.data.length > 0 && (
          <>
            <div
              className={cn(
                'hidden gap-6 px-5 pb-2 text-xs font-bold text-muted lg:grid',
                rowGrid,
              )}
            >
              <span>Номер</span>
              <span>Заявка</span>
              <span>Создана</span>
              <span>Статус</span>
            </div>
            <ul className="flex flex-col gap-3">
              {data.data.map((item) => (
                <RequestRow key={item.id} item={item} />
              ))}
            </ul>
          </>
        )}
      </div>
    </section>
  );
}

function RequestRow({ item }: { item: RequestItem }) {
  const status = requestStatusMeta[item.status] ?? {
    label: item.status,
    tone: 'bg-mist',
  };
  const type = item.type ? (requestTypeLabels[item.type] ?? item.type) : null;

  return (
    <li className="border border-line bg-paper p-5">
      <div className={cn('grid items-center gap-3 lg:gap-6', rowGrid)}>
        <div>
          <p className="text-base font-extrabold">{item.number ?? item.id}</p>
          {type && <p className="text-sm text-muted">{type}</p>}
        </div>

        <div className="min-w-0">
          <p className="text-base font-medium">
            {item.title ?? type ?? 'Заявка'}
          </p>
          {item.summary && (
            <p className="mt-0.5 text-sm text-muted">{item.summary}</p>
          )}
        </div>

        <p className="text-sm text-muted">{formatDate(item.created_at)}</p>

        <span
          className={cn(
            'w-fit px-2.5 py-1 text-xs font-bold lg:w-full lg:text-center',
            status.tone,
          )}
        >
          {status.label}
        </span>
      </div>
    </li>
  );
}

function formatDate(value: string) {
  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
}

function RequestsSkeleton() {
  return (
    <div
      role="status"
      aria-label="Загружаем заявки"
      className="flex flex-col gap-3"
    >
      {[0, 1, 2].map((index) => (
        <Skeleton key={index} className="h-20" />
      ))}
    </div>
  );
}