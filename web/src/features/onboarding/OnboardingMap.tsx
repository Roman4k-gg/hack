import { useState, type FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useAuthStore } from '@/features/auth/authStore';
import { ApiError } from '@/lib/api';
import { EmptyState } from '@/components/ui/EmptyState';
import { Skeleton } from '@/components/ui/Skeleton';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { cn } from '@/lib/utils';
import { fetchPlan, updateItemStatus } from './api';
import type { ItemStatus, Plan, PlanItem } from './types';

const stageColors = [
  'bg-brand-soft',
  'bg-azure-soft',
  'bg-iris-soft',
  'bg-alert-soft',
];

export function OnboardingMap() {
  const queryClient = useQueryClient();
  const userId = useAuthStore((state) => state.user?.id);
  const planKey = ['onboarding-plan', userId];

  const { data, error, isPending, isError, refetch } = useQuery({
    queryKey: planKey,
    queryFn: fetchPlan,
  });

  const mutation = useMutation({
    mutationFn: ({
      itemId,
      status,
      reason,
    }: {
      itemId: string;
      status: ItemStatus;
      reason?: string;
    }) => updateItemStatus(itemId, status, reason),
    onMutate: async ({ itemId, status }) => {
      await queryClient.cancelQueries({ queryKey: planKey });

      const previous = queryClient.getQueryData<Plan>(planKey);

      queryClient.setQueryData<Plan>(planKey, (old) =>
        old ? applyStatus(old, itemId, status) : old,
      );

      return { previous };
    },
    onError: (_error, _variables, context) => {
      if (context?.previous) {
        queryClient.setQueryData(planKey, context.previous);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: planKey });
    },
  });

  function changeStatus(itemId: string, status: ItemStatus, reason?: string) {
    mutation.mutate({ itemId, status, reason });
  }

  if (isPending) {
    return <MapSkeleton />;
  }

  if (error instanceof ApiError && error.status === 404) {
    return (
      <EmptyState
        title="План адаптации не назначен"
        text="Как только HR или наставник назначат план, он появится здесь."
      />
    );
  }

  if (isError || !data) {
    return (
      <div>
        <p>Не удалось загрузить карту. Проверьте подключение к API.</p>
        <button
          className="mt-3 bg-ink px-4 py-2 text-sm font-bold text-paper"
          onClick={() => refetch()}
        >
          Повторить
        </button>
      </div>
    );
  }

  return <PlanView plan={data} onChangeStatus={changeStatus} />;
}

function applyStatus(plan: Plan, itemId: string, status: ItemStatus): Plan {
  const stages = plan.stages.map((stage) => ({
    ...stage,
    items: stage.items.map((item) =>
      item.id === itemId ? { ...item, status } : item,
    ),
  }));

  const allItems = stages.flatMap((stage) => stage.items);
  const done = allItems.filter((item) => item.status === 'done').length;

  return {
    ...plan,
    stages,
    progress: {
      done,
      total: allItems.length,
      percent:
        allItems.length > 0 ? Math.round((done / allItems.length) * 100) : 0,
    },
  };
}

function MapSkeleton() {
  return (
    <div role="status" aria-label="Загружаем карту">
      <Skeleton className="h-4 w-24" />
      <Skeleton className="mt-3 h-9 w-64" />
      <div className="mt-6 flex flex-col gap-5 lg:flex-row">
        <Skeleton className="h-36 lg:flex-[2]" />
        <Skeleton className="h-36 lg:flex-1" />
      </div>
      <div className="mt-8 grid gap-5 md:grid-cols-2 xl:grid-cols-4">
        {[0, 1, 2, 3].map((index) => (
          <Skeleton key={index} className="h-72" />
        ))}
      </div>
    </div>
  );
}

function PlanView({
  plan,
  onChangeStatus,
}: {
  plan: Plan;
  onChangeStatus: (itemId: string, status: ItemStatus, reason?: string) => void;
}) {
  if (plan.stages.length === 0) {
    return (
      <EmptyState
        title="План онбординга пока не назначен"
        text="Как только HR или наставник подготовят план, он появится здесь."
      />
    );
  }

  const firstName = plan.intern.full_name.split(' ')[0];

  return (
    <section>
      <p className="text-xs text-muted">Команда {plan.intern.team_name}</p>
      <h1 className="mt-1 text-3xl font-extrabold tracking-tight">
        Привет, {firstName}!
      </h1>

      <div className="mt-6 flex flex-col gap-5 lg:flex-row">
        <div className="bg-brand p-5 lg:flex-[2]">
          <p className="text-xs font-bold">Следующий шаг</p>
          {plan.next_step ? (
            <>
              <p className="mt-1 text-xl font-extrabold tracking-tight">
                {plan.next_step.title}
              </p>
              <p className="mt-1 text-sm">{plan.next_step.reason}</p>
            </>
          ) : plan.progress.percent === 100 ? (
            <>
              <p className="mt-1 text-xl font-extrabold tracking-tight">
                Все пункты плана выполнены
              </p>
              <p className="mt-1 text-sm">
                Отличная работа! Если появятся вопросы — спроси наставника или
                AI-наставника в чате.
              </p>
            </>
          ) : (
            <>
              <p className="mt-1 text-xl font-extrabold tracking-tight">
                Пока нет доступных шагов
              </p>
              <p className="mt-1 text-sm">
                Сними блокеры или уточни план у наставника.
              </p>
            </>
          )}
        </div>

        <div
          className={cn(
            'bg-azure p-5 lg:flex-1',
            plan.progress.percent < 100 && 'rotate-[1.5deg]',
          )}
        >
          <p className="text-xs font-bold">Прогресс</p>
          <p className="mt-2 text-5xl font-extrabold tracking-tight">
            {plan.progress.percent}%
          </p>
          <p className="mt-1 text-xs">
            {plan.progress.done} из {plan.progress.total} пунктов
          </p>
        </div>
      </div>

      <div className="mt-8 grid gap-5 md:grid-cols-2 xl:grid-cols-4">
        {plan.stages.map((stage, index) => (
          <div
            key={stage.id}
            className={cn('p-4', stageColors[index % stageColors.length])}
          >
            <h2 className="text-base font-extrabold">{stage.title}</h2>

            <ul className="mt-3 flex flex-col gap-3">
              {stage.items.map((item) => (
                <ItemCard
                  key={item.id}
                  item={item}
                  onChangeStatus={onChangeStatus}
                />
              ))}
            </ul>
          </div>
        ))}
      </div>
    </section>
  );
}

function StatusAction({
  label,
  onClick,
}: {
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="bg-mist px-2 py-0.5 text-[11px] font-bold text-muted hover:bg-line"
    >
      {label}
    </button>
  );
}

function ItemCard({
  item,
  onChangeStatus,
}: {
  item: PlanItem;
  onChangeStatus: (itemId: string, status: ItemStatus, reason?: string) => void;
}) {
  const [blocking, setBlocking] = useState(false);
  const [reason, setReason] = useState('');
  const isBlocked = item.status === 'blocked';

  function submitBlock(event: FormEvent) {
    event.preventDefault();
    const trimmed = reason.trim();

    if (!trimmed) {
      return;
    }

    onChangeStatus(item.id, 'blocked', trimmed);
    setBlocking(false);
    setReason('');
  }

  function cancelBlock() {
    setBlocking(false);
    setReason('');
  }

  return (
    <li
      className={cn(
        'flex min-h-[8rem] flex-col bg-paper p-3',
        isBlocked && 'border-l-4 border-alert',
      )}
    >
      <p className="text-sm font-medium leading-snug">{item.title}</p>
      <p className="mt-1 line-clamp-2 text-xs text-muted">{item.description}</p>

      <div className="mt-auto flex flex-wrap items-center gap-x-2 gap-y-1 pt-2">
        <StatusBadge status={item.status} />
        {isBlocked && item.blocked_reason && (
          <span className="text-xs text-muted">{item.blocked_reason}</span>
        )}
      </div>

      {blocking ? (
        <form onSubmit={submitBlock} className="mt-2 flex flex-col gap-2">
          <input
            autoFocus
            value={reason}
            maxLength={200}
            onChange={(event) => setReason(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                cancelBlock();
              }
            }}
            placeholder="Что мешает? Например: нет доступа к VPN"
            aria-label={`Причина блокировки для «${item.title}»`}
            className="w-full border border-line bg-mist px-2 py-1 text-xs outline-none focus:border-ink"
          />
          <div className="flex gap-1">
            <button
              type="submit"
              disabled={!reason.trim()}
              className="bg-ink px-2 py-0.5 text-[11px] font-bold text-paper disabled:opacity-40"
            >
              Сохранить
            </button>
            <StatusAction label="Отмена" onClick={cancelBlock} />
          </div>
        </form>
      ) : (
        <div className="mt-2 flex flex-wrap gap-1 pt-1">
          {item.status !== 'in_progress' && (
            <StatusAction
              label={isBlocked ? 'Снять блок' : 'В работу'}
              onClick={() => onChangeStatus(item.id, 'in_progress')}
            />
          )}
          {item.status !== 'done' && (
            <StatusAction
              label="Готово"
              onClick={() => onChangeStatus(item.id, 'done')}
            />
          )}
          {!isBlocked && (
            <StatusAction
              label="Нужна помощь"
              onClick={() => setBlocking(true)}
            />
          )}
        </div>
      )}
    </li>
  );
}