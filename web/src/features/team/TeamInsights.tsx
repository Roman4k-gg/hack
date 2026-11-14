import { useQuery } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { DemoBadge } from '@/components/ui/DemoBadge';
import { Skeleton } from '@/components/ui/Skeleton';
import { useAuthStore } from '@/features/auth/authStore';
import { cn } from '@/lib/utils';
import {
  fetchContentGaps,
  fetchTopQuestions,
  gapReasonLabels,
} from './analytics';

export function TeamInsights() {
  const userId = useAuthStore((state) => state.user?.id);

  const questions = useQuery({
    queryKey: ['top-questions', userId],
    queryFn: fetchTopQuestions,
  });
  const gaps = useQuery({
    queryKey: ['content-gaps', userId],
    queryFn: fetchContentGaps,
  });

  const topCount = Math.max(
    1,
    ...(questions.data?.data.map((q) => q.count) ?? []),
  );

  return (
    <div className="mt-6 grid gap-4 lg:grid-cols-2">
      <InsightCard
        title="Топ-вопросы стажёров"
        hint="О чём чаще всего спрашивают AI-наставника"
        accent="border-azure"
        demo={questions.data?.demo}
      >
        {questions.isPending && <Skeleton className="h-40" />}
        {questions.isError && <ErrorText />}
        {questions.data?.data.length === 0 && (
          <p className="text-sm text-muted">Вопросов пока не было.</p>
        )}
        <ol className="flex flex-col gap-3">
          {questions.data?.data.slice(0, 5).map((item) => (
            <li key={item.question}>
              <div className="flex items-baseline justify-between gap-4 text-sm">
                <span className="font-medium">{item.question}</span>
                <span className="shrink-0 font-bold">{item.count}</span>
              </div>
              <div className="mt-1 h-1.5 w-full bg-mist">
                <div
                  className="h-1.5 bg-azure"
                  style={{ width: `${(item.count / topCount) * 100}%` }}
                />
              </div>
            </li>
          ))}
        </ol>
      </InsightCard>

      <InsightCard
        title="Пробелы в контенте"
        hint="Темы, по которым AI не нашёл ответ в базе знаний"
        accent="border-iris"
        demo={gaps.data?.demo}
      >
        {gaps.isPending && <Skeleton className="h-40" />}
        {gaps.isError && <ErrorText />}
        {gaps.data?.data.length === 0 && (
          <p className="text-sm text-muted">Пробелов не найдено.</p>
        )}
        <ul className="flex flex-col gap-3">
          {gaps.data?.data.slice(0, 5).map((item) => (
            <li
              key={item.topic}
              className="flex items-center justify-between gap-4 text-sm"
            >
              <div>
                <p className="font-medium">{item.topic}</p>
                <p className="text-muted">
                  {item.questions} вопросов без ответа
                </p>
              </div>
              {item.reason && (
                <span
                  className={cn(
                    'shrink-0 px-2 py-0.5 text-xs font-bold',
                    item.reason === 'outdated'
                      ? 'bg-alert-soft'
                      : 'bg-iris-soft',
                  )}
                >
                  {gapReasonLabels[item.reason] ?? item.reason}
                </span>
              )}
            </li>
          ))}
        </ul>
      </InsightCard>
    </div>
  );
}

function InsightCard({
  title,
  hint,
  accent,
  demo,
  children,
}: {
  title: string;
  hint: string;
  accent: string;
  demo?: boolean;
  children: ReactNode;
}) {
  return (
    <section
      className={cn('border border-line border-t-4 bg-paper p-5', accent)}
    >
      <div className="flex flex-wrap items-center gap-3">
        <h2 className="text-base font-extrabold">{title}</h2>
        {demo && <DemoBadge />}
      </div>
      <p className="mb-4 mt-1 text-sm text-muted">{hint}</p>
      {children}
    </section>
  );
}

function ErrorText() {
  return <p className="text-sm text-muted">Не удалось загрузить данные.</p>;
}