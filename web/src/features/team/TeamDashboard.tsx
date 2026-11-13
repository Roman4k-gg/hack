import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { EmptyState } from '@/components/ui/EmptyState';
import { Skeleton } from '@/components/ui/Skeleton';
import { useAuthStore } from '@/features/auth/authStore';
import { cn } from '@/lib/utils';
import { fetchTeamReport } from './api';
import type { TeamMember } from './types';

const TEAM_SCOPE = 'onboarding:team:read';

export function TeamDashboard() {
  const scopes = useAuthStore((state) => state.user?.scopes ?? []);
  const canView = scopes.includes(TEAM_SCOPE) || scopes.includes('*');
  const [teamFilter, setTeamFilter] = useState('all');

  const { data, isPending, isError, refetch } = useQuery({
    queryKey: ['team-report'],
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

    if (teamFilter === 'all') {
      return data.interns;
    }

    return data.interns.filter((member) => member.team.id === teamFilter);
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
        <p>Не удалось загрузить метрики команды. Проверьте подключение к API.</p>
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
        Прогресс, успеваемость и риски по каждому стажёру — из PostgreSQL.
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
          hint="дедлайны не просрочены"
          tone="bg-iris-soft"
        />
        <MetricCard
          label="В зоне риска"
          value={String(overview.at_risk)}
          hint={`блокеров: ${overview.blocked}, просрочено: ${overview.overdue}`}
          tone="bg-alert-soft"
        />
      </div>

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
        <div className="mt-6 grid gap-4 lg:grid-cols-2">
          {members.map((member) => (
            <MemberCard key={member.id} member={member} />
          ))}
        </div>
      )}

      {teams.length === 0 && data.interns.length === 0 && (
        <p className="mt-4 text-xs text-muted">
          Команды без плана адаптации тоже видны в списке — у них нулевой прогресс.
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
    <div className={cn('p-4', tone)}>
      <p className="text-xs font-bold">{label}</p>
      <p className="mt-1 text-4xl font-extrabold tracking-tight">{value}</p>
      <p className="mt-1 text-xs text-muted">{hint}</p>
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

function MemberCard({ member }: { member: TeamMember }) {
  const hasPlan = Boolean(member.plan);
  const stage = member.plan?.current_stage ?? 'План не назначен';

  return (
    <article
      className={cn(
        'border border-line bg-paper p-4',
        member.at_risk && 'border-l-4 border-l-alert',
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-base font-extrabold">{member.full_name}</h2>
          <p className="text-xs text-muted">
            {member.position || 'Стажёр'} · {member.team.name || 'без команды'}
            {member.mentor ? ` · наставник ${member.mentor.full_name}` : ''}
          </p>
        </div>
        {member.at_risk && (
          <span className="bg-alert-soft px-2 py-0.5 text-[11px] font-bold">
            риск
          </span>
        )}
      </div>

      <div className="mt-4">
        <div className="flex items-baseline justify-between text-xs font-bold">
          <span>Прогресс</span>
          <span>
            {member.progress.done} из {member.progress.total} ·{' '}
            {member.progress.percent}%
          </span>
        </div>
        <div className="mt-1 h-2 w-full bg-mist">
          <div
            className="h-2 bg-brand"
            style={{ width: `${Math.min(member.progress.percent, 100)}%` }}
          />
        </div>
      </div>

      <div className="mt-4 grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
        <Row label="Успеваемость" value={`${member.pace.percent}%`} />
        <Row label="Этап" value={stage} />
        <Row
          label="Статусы"
          value={`в работе ${member.progress.in_progress}, ждут ${member.progress.pending}`}
        />
        <Row label="Вопросов AI" value={String(member.activity.questions)} />
      </div>

      {member.blockers.length > 0 && (
        <div className="mt-4 border-l-4 border-alert bg-alert-soft/40 p-3">
          <p className="text-xs font-bold">Блокеры</p>
          <ul className="mt-1 flex flex-col gap-1 text-xs">
            {member.blockers.map((blocker) => (
              <li key={blocker.item_id}>
                <span className="font-bold">{blocker.title}</span>
                {blocker.reason ? ` — ${blocker.reason}` : ''}
              </li>
            ))}
          </ul>
        </div>
      )}

      {member.overdue.length > 0 && (
        <div className="mt-2 border-l-4 border-l-iris bg-iris-soft/40 p-3">
          <p className="text-xs font-bold">Просрочено</p>
          <ul className="mt-1 flex flex-col gap-1 text-xs">
            {member.overdue.map((item) => (
              <li key={item.item_id}>
                {item.title} — на {item.days_overdue} дн. позже срока
              </li>
            ))}
          </ul>
        </div>
      )}

      {!hasPlan && (
        <p className="mt-3 text-xs text-muted">
          План адаптации ещё не назначен — прогресс не считается.
        </p>
      )}
    </article>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-2">
      <span className="text-muted">{label}</span>
      <span className="text-right font-bold">{value}</span>
    </div>
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
      <div className="mt-6 grid gap-4 lg:grid-cols-2">
        {[0, 1].map((index) => (
          <Skeleton key={index} className="h-56" />
        ))}
      </div>
    </div>
  );
}
