import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { EmptyState } from '@/components/ui/EmptyState';
import { Skeleton } from '@/components/ui/Skeleton';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { cn } from '@/lib/utils';
import { fetchPlan, updateItemStatus } from './api';
import type { ItemStatus, Plan } from './types';

const stageColors = [
  'bg-brand-soft',
  'bg-azure-soft',
  'bg-iris-soft',
  'bg-alert-soft',
];

export function OnboardingMap() {
  const queryClient = useQueryClient();

  const { data, isPending, isError, refetch } = useQuery({
    queryKey: ['onboarding-plan'],
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
      await queryClient.cancelQueries({ queryKey: ['onboarding-plan'] });

      const previous = queryClient.getQueryData<Plan>(['onboarding-plan']);

      queryClient.setQueryData<Plan>(['onboarding-plan'], (old) =>
        old ? applyStatus(old, itemId, status) : old,
      );

      return { previous };
    },
    onError: (_error, _variables, context) => {
      if (context?.previous) {
        queryClient.setQueryData(['onboarding-plan'], context.previous);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ['onboarding-plan'] });
    },
  });

  function changeStatus(
    itemId: string,
    status: ItemStatus,
    title: string,
  ) {
    let reason: string | undefined;

    if (status === 'blocked') {
      const input = window.prompt(
        `Почему пункт «${title}» заблокирован? Причина попадёт в карту и в аудит.`,
        'Ждём подтверждения от HR',
      );

      if (!input) {
        return;
      }

      reason = input;
    }

    mutation.mutate({ itemId, status, reason });
  }

  if (isPending) {
    return <MapSkeleton />;
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
      percent: allItems.length > 0 ? Math.round((done / allItems.length) * 100) : 0,
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
  onChangeStatus: (itemId: string, status: ItemStatus, title: string) => void;
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
        {plan.next_step && (
          <div className="bg-brand p-5 lg:flex-[2]">
            <p className="text-xs font-bold">Следующий шаг</p>
            <p className="mt-1 text-xl font-extrabold tracking-tight">
              {plan.next_step.title}
            </p>
            <p className="mt-1 text-sm">{plan.next_step.reason}</p>
          </div>
        )}

        <div className="rotate-[1.5deg] bg-azure p-5 lg:flex-1">
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
                <li
                  key={item.id}
                  className={cn(
                    'flex min-h-[8rem] flex-col bg-paper p-3',
                    item.status === 'blocked' && 'border-l-4 border-alert',
                  )}
                >
                  <p className="text-sm font-medium leading-snug">
                    {item.title}
                  </p>
                  <p className="mt-1 line-clamp-2 text-xs text-muted">
                    {item.description}
                  </p>
                  <div className="mt-auto flex flex-wrap items-center gap-x-2 gap-y-1 pt-2">
                    <StatusBadge status={item.status} />
                    {item.status === 'blocked' && item.blocked_reason && (
                      <span className="text-xs text-muted">
                        {item.blocked_reason}
                      </span>
                    )}
                  </div>
                  <div className="mt-2 flex flex-wrap gap-1 pt-1">
                    {item.status !== 'in_progress' && (
                      <StatusAction
                        label="В работу"
                        onClick={() =>
                          onChangeStatus(item.id, 'in_progress', item.title)
                        }
                      />
                    )}
                    {item.status !== 'done' && (
                      <StatusAction
                        label="Готово"
                        onClick={() => onChangeStatus(item.id, 'done', item.title)}
                      />
                    )}
                    {item.status !== 'blocked' && (
                      <StatusAction
                        label="Блокер"
                        onClick={() =>
                          onChangeStatus(item.id, 'blocked', item.title)
                        }
                      />
                    )}
                  </div>
                </li>
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
