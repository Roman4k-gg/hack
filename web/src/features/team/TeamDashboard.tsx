import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { EmptyState } from '@/components/ui/EmptyState';
import { Skeleton } from '@/components/ui/Skeleton';
import { useAuthStore } from '@/features/auth/authStore';
import { cn } from '@/lib/utils';
import { fetchTeamReport } from './api';
import { TeamInsights } from './TeamInsights';
import type { TeamMember } from './types';

const TEAM_SCOPE = 'onboarding:team:read';

export function TeamDashboard() {
  const scopes = useAuthStore((state) => state.user?.scopes ?? []);
  const userId = useAuthStore((state) => state.user?.id);
  const canView = scopes.includes(TEAM_SCOPE) || scopes.includes('*');
  const [teamFilter, setTeamFilter] = useState('all');

  const { data, isPending, isError, refetch } = useQuery({
    queryKey: ['team-report', userId],
    queryFn: fetchTeamReport,
    enabled: canView,
  });

  const teams = useMemo(() => {
    if (!data) {
      return [];
    }

    return data.overview.teams.map((team) => team.id).filter(Boolean);
  }, [data]);

  const members = useMemo(() => {
    if (!data) {
      return [];
    }

    const list =
      teamFilter === 'all'
        ? data.interns
        : data.interns.filter((member) => member.team.id === teamFilter);

    return [...list].sort(
      (a, b) =>
        Number(b.at_risk) - Number(a.at_risk) ||
        a.progress.percent - b.progress.percent,
    );
  }, [data, teamFilter]);

  if (!canView) {
    return (
      <EmptyState
        title="Нет доступа к метрикам команды"
        text="Раздел доступен наставникам, руководителям, HR и администраторам. Если нужно — обратитесь к HR-партнёру."
      />
    );
  }

  if (isPending) {
    return <DashboardSkeleton />;
  }

  if (isError || !data) {
    return (
      <div>
        <p>
          Не удалось загрузить метрики команды. Проверьте подключение к API.
        </p>
        <button
          className="mt-3 bg-ink px-4 py-2 text-sm font-bold text-paper"
          onClick={() => refetch()}
        >
          Повторить
        </button>
      </div>
    );
  }

  const { overview } = data;

  return (
    <section>
      <p className="text-xs text-muted">Наставникам и руководителям</p>
      <h1 className="mt-1 text-3xl font-extrabold tracking-tight">Команда</h1>
      <p className="mt-1 text-sm text-muted">
        Кому нужна помощь и как идёт адаптация каждого стажёра.
      </p>

      <div className="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          label="Стажёров"
          value={String(overview.interns)}
          hint="в команде"
          tone="bg-azure-soft"
        />
        <MetricCard
          label="Средний прогресс"
          value={`${overview.avg_progress}%`}
          hint="пунктов чек-листа"
          tone="bg-brand-soft"
        />
        <MetricCard
          label="Успеваемость"
          value={`${overview.avg_pace}%`}
          hint="выполнено к сроку"
          tone="bg-iris-soft"
        />
        <MetricCard
          label="В зоне риска"
          value={String(overview.at_risk)}
          hint={`заблокировано: ${overview.blocked}, просрочено: ${overview.overdue}`}
          tone="bg-alert-soft"
        />
      </div>

      <TeamInsights />

      {overview.teams.length > 1 && (
        <div className="mt-6 flex flex-wrap gap-2">
          <FilterChip
            active={teamFilter === 'all'}
            onClick={() => setTeamFilter('all')}
            label="Все команды"
          />
          {overview.teams.map((team) => (
            <FilterChip
              key={team.id}
              active={teamFilter === team.id}
              onClick={() => setTeamFilter(team.id)}
              label={`${team.name} · ${team.interns}`}
            />
          ))}
        </div>
      )}

      {members.length === 0 ? (
        <div className="mt-6">
          <EmptyState
            title="Стажёров пока нет"
            text="Как только стажёры получат план адаптации, они появятся здесь с метриками."
          />
        </div>
      ) : (
        <div className="mt-6">
          <div className="hidden grid-cols-[minmax(0,1.4fr)_minmax(0,1.4fr)_9.5rem_5rem] gap-6 px-5 pb-2 text-xs font-bold text-muted lg:grid">
            <span>Стажёр</span>
            <span>Прогресс</span>
            <span>Статус</span>
            <span className="text-right">Вопросов AI</span>
          </div>
          <ul className="flex flex-col gap-3">
            {members.map((member) => (
              <MemberRow key={member.id} member={member} />
            ))}
          </ul>
        </div>
      )}

      {teams.length === 0 && data.interns.length === 0 && (
        <p className="mt-4 text-xs text-muted">
          Команды без плана адаптации тоже видны в списке — у них нулевой
          прогресс.
        </p>
      )}
    </section>
  );
}

function MetricCard({
  label,
  value,
  hint,
  tone,
}: {
  label: string;
  value: string;
  hint: string;
  tone: string;
}) {
  return (
    <div className={cn('p-5', tone)}>
      <p className="text-sm font-bold">{label}</p>
      <p className="mt-1 text-5xl font-extrabold tracking-tight">{value}</p>
      <p className="mt-1 text-sm text-muted">{hint}</p>
    </div>
  );
}

function FilterChip({
  active,
  onClick,
  label,
}: {
  active: boolean;
  onClick: () => void;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'px-3 py-1.5 text-xs font-bold',
        active ? 'bg-ink text-paper' : 'bg-mist text-muted hover:bg-line',
      )}
    >
      {label}
    </button>
  );
}

function MemberRow({ member }: { member: TeamMember }) {
  const hasPlan = Boolean(member.plan);
  const stage = member.plan?.current_stage;

  const status = !hasPlan
    ? { label: 'План не назначен', tone: 'bg-mist text-muted' }
    : member.at_risk
      ? { label: 'Нужна помощь', tone: 'bg-alert-soft text-ink' }
      : { label: 'Идёт по плану', tone: 'bg-brand-soft text-ink' };

  return (
    <li
      className={cn(
        'border border-line bg-paper p-5',
        member.at_risk && 'border-l-4 border-l-alert',
      )}
    >
      <div className="grid items-center gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1.4fr)_9.5rem_5rem] lg:gap-6">
        <div className="min-w-0">
          <p className="truncate text-base font-extrabold">
            {member.full_name}
          </p>
          <p className="truncate text-sm text-muted">
            {member.position || 'Стажёр'} · {member.team.name || 'без команды'}
          </p>
        </div>

        <div>
          {hasPlan ? (
            <>
              <div className="flex items-baseline justify-between text-sm font-bold">
                <span>
                  {member.progress.done} из {member.progress.total}
                </span>
                <span>{member.progress.percent}%</span>
              </div>
              <div className="mt-1 h-2.5 w-full bg-mist">
                <div
                  className="h-2.5 bg-brand"
                  style={{
                    width: `${Math.min(member.progress.percent, 100)}%`,
                  }}
                />
              </div>
              {stage && <p className="mt-1 text-xs text-muted">{stage}</p>}
            </>
          ) : (
            <p className="text-sm text-muted">Прогресс не считается</p>
          )}
        </div>

        <span
          className={cn(
            'w-fit px-2.5 py-1 text-xs font-bold lg:w-full lg:text-center',
            status.tone,
          )}
        >
          {status.label}
        </span>

        <p className="text-sm font-bold lg:text-right">
          <span className="font-normal text-muted lg:hidden">
            Вопросов AI:{' '}
          </span>
          {member.activity.questions}
        </p>
      </div>

      {(member.blockers.length > 0 || member.overdue.length > 0) && (
        <ul className="mt-4 flex flex-col gap-1 border-t border-line pt-3 text-sm">
          {member.blockers.map((blocker) => (
            <li key={blocker.item_id}>
              <span className="mr-2 bg-alert-soft px-1.5 py-0.5 text-xs font-bold">
                Заблокировано
              </span>
              <span className="font-bold">{blocker.title}</span>
              {blocker.reason ? ` — ${blocker.reason}` : ''}
            </li>
          ))}
          {member.overdue.map((item) => (
            <li key={item.item_id}>
              <span className="mr-2 bg-iris-soft px-1.5 py-0.5 text-xs font-bold">
                Просрочено
              </span>
              <span className="font-bold">{item.title}</span> — на{' '}
              {item.days_overdue} дн. позже срока
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}

function DashboardSkeleton() {
  return (
    <div role="status" aria-label="Загружаем метрики команды">
      <Skeleton className="h-4 w-24" />
      <Skeleton className="mt-3 h-9 w-64" />
      <div className="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {[0, 1, 2, 3].map((index) => (
          <Skeleton key={index} className="h-24" />
        ))}
      </div>
      <div className="mt-6 flex flex-col gap-3">
        {[0, 1, 2].map((index) => (
          <Skeleton key={index} className="h-24" />
        ))}
      </div>
    </div>
  );
}