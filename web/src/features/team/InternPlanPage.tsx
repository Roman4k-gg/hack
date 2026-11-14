import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { ArrowLeft } from 'lucide-react';
import { DemoBadge } from '@/components/ui/DemoBadge';
import { EmptyState } from '@/components/ui/EmptyState';
import { Skeleton } from '@/components/ui/Skeleton';
import { useAuthStore } from '@/features/auth/authStore';
import { PlanView } from '@/features/onboarding/OnboardingMap';
import { fetchTeamReport } from './api';
import { fetchInternPlan } from './internPlan';

const TEAM_SCOPE = 'onboarding:team:read';

export function InternPlanPage({ internId }: { internId: string }) {
  const scopes = useAuthStore((state) => state.user?.scopes ?? []);
  const userId = useAuthStore((state) => state.user?.id);
  const canView = scopes.includes(TEAM_SCOPE) || scopes.includes('*');

  const report = useQuery({
    queryKey: ['team-report', userId],
    queryFn: fetchTeamReport,
    enabled: canView,
  });

  const member = report.data?.interns.find((item) => item.id === internId);

  const plan = useQuery({
    queryKey: ['intern-plan', userId, internId],
    queryFn: () =>
      fetchInternPlan({
        id: internId,
        full_name: member?.full_name ?? '',
        team_name: member?.team.name ?? '',
      }),
    enabled: canView && Boolean(member?.plan),
  });

  if (!canView) {
    return (
      <EmptyState
        title="Нет доступа к картам стажёров"
        text="Раздел доступен наставникам, руководителям, HR и администраторам."
      />
    );
  }

  if (report.isPending || (Boolean(member?.plan) && plan.isPending)) {
    return (
      <div role="status" aria-label="Загружаем карту стажёра">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="mt-3 h-9 w-64" />
        <Skeleton className="mt-6 h-36" />
      </div>
    );
  }

  if (report.isError || plan.isError) {
    return (
      <div>
        <BackLink />
        <p className="mt-4">
          Не удалось загрузить карту стажёра. Проверьте подключение к API.
        </p>
        <button
          className="mt-3 bg-ink px-4 py-2 text-sm font-bold text-paper"
          onClick={() => {
            void report.refetch();
            void plan.refetch();
          }}
        >
          Повторить
        </button>
      </div>
    );
  }

  if (!member) {
    return (
      <div>
        <BackLink />
        <div className="mt-4">
          <EmptyState
            title="Стажёр не найден"
            text="Возможно, он не из вашей команды. Вернитесь к списку и выберите стажёра ещё раз."
          />
        </div>
      </div>
    );
  }

  if (!member.plan || !plan.data) {
    return (
      <div>
        <BackLink />
        <div className="mt-4">
          <EmptyState
            title={`${member.full_name}: план не назначен`}
            text="Как только HR или наставник назначат план, карта появится здесь."
          />
        </div>
      </div>
    );
  }

  return (
    <PlanView
      plan={plan.data.data}
      header={
        <>
          <BackLink />
          <p className="mt-3 text-xs text-muted">
            Команда {member.team.name} · {member.position || 'Стажёр'}
          </p>
          <h1 className="mt-1 text-3xl font-extrabold tracking-tight">
            {member.full_name}
          </h1>
          <p className="mt-1 flex items-center gap-2 text-sm text-muted">
            Карта адаптации · режим просмотра
            {plan.data.demo && <DemoBadge />}
          </p>
        </>
      }
    />
  );
}

function BackLink() {
  return (
    <Link
      to="/dashboard"
      className="inline-flex items-center gap-1 bg-mist px-3 py-1.5 text-xs font-bold hover:bg-line"
    >
      <ArrowLeft aria-hidden strokeWidth={3} className="h-4 w-4" />
      К команде
    </Link>
  );
}