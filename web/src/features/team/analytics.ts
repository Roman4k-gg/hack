import { z } from 'zod';
import { apiFetch } from '@/lib/api';
import { listOf, withDemoFallback } from '@/lib/demo';

const topQuestionSchema = z.object({
  question: z.string(),
  count: z.number(),
});

const contentGapSchema = z.object({
  topic: z.string(),
  questions: z.number(),
  reason: z.string().nullable().optional(),
});

export type TopQuestion = z.infer<typeof topQuestionSchema>;
export type ContentGap = z.infer<typeof contentGapSchema>;

export const gapReasonLabels: Record<string, string> = {
  missing: 'Нет документа',
  outdated: 'Документ устарел',
};

const mockTopQuestions: TopQuestion[] = [
  { question: 'Где взять доступ к Jira?', count: 14 },
  { question: 'Как оформить отгул?', count: 11 },
  { question: 'Как устроен процесс релиза?', count: 8 },
  { question: 'Кто мой наставник и как с ним связаться?', count: 6 },
  { question: 'Как настроить VPN?', count: 5 },
];

const mockContentGaps: ContentGap[] = [
  { topic: 'Оформление командировки', questions: 5, reason: 'missing' },
  {
    topic: 'Правила код-ревью в команде Ads',
    questions: 4,
    reason: 'outdated',
  },
  { topic: 'Доступ к тестовым стендам', questions: 3, reason: 'missing' },
];

export function fetchTopQuestions() {
  return withDemoFallback(
    async () =>
      listOf(topQuestionSchema).parse(
        await apiFetch<unknown>('/analytics/top-questions'),
      ),
    mockTopQuestions,
  );
}

export function fetchContentGaps() {
  return withDemoFallback(
    async () =>
      listOf(contentGapSchema).parse(
        await apiFetch<unknown>('/analytics/content-gaps'),
      ),
    mockContentGaps,
  );
}