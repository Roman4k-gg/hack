import { useQuery } from '@tanstack/react-query';
import { StatusBadge } from '@/components/ui/StatusBadge';
import { EmptyState } from '@/components/ui/EmptyState';
import { Skeleton } from '@/components/ui/Skeleton';
import { cn } from '@/lib/utils';
import { fetchPlan } from './api';
import type { Plan } from './types';

const stageColors = [
  'bg-brand-soft',
  'bg-azure-soft',
  'bg-iris-soft',
  'bg-alert-soft',
];

export function OnboardingMap() {
  const { data, isPending, isError, refetch } = useQuery({
    queryKey: ['onboarding-plan'],
    queryFn: fetchPlan,
  });

  if (isPending) {
    return <MapSkeleton />;
  }

  if (isError) {
    return (
      <div>
        <p>Не удалось загрузить карту. Попробуйте ещё раз.</p>
        <button
          className="mt-3 bg-ink px-4 py-2 text-sm font-bold text-paper"
          onClick={() => refetch()}
        >
          Повторить
        </button>
      </div>
    );
  }

  return <PlanView plan={data} />;
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

function PlanView({ plan }: { plan: Plan }) {
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
        Привет, {firstName}
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
                    'bg-paper p-3',
                    item.status === 'blocked' && 'border-l-4 border-alert',
                  )}
                >
                  <p className="text-sm font-medium leading-snug">
                    {item.title}
                  </p>
                  <div className="mt-2">
                    <StatusBadge status={item.status} />
                  </div>
                  {item.status === 'blocked' && item.blocked_reason && (
                    <p className="mt-2 text-xs text-muted">
                      {item.blocked_reason}
                    </p>
                  )}
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </section>
  );
}
