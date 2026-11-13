import { z } from 'zod';
import { listOf } from '@/lib/demo';

export const requestSchema = z.object({
  id: z.string(),
  number: z.string().nullable().optional(),
  type: z.string().nullable().optional(),
  title: z.string().nullable().optional(),
  summary: z.string().nullable().optional(),
  status: z.string(),
  created_at: z.string(),
});

export const requestListSchema = listOf(requestSchema);

export type RequestItem = z.infer<typeof requestSchema>;

export const requestTypeLabels: Record<string, string> = {
  absence: 'Отгул',
  access: 'Доступ',
};

export const requestStatusMeta: Record<
  string,
  { label: string; tone: string }
> = {
  pending: { label: 'На рассмотрении', tone: 'bg-azure-soft' },
  approved: { label: 'Одобрена', tone: 'bg-brand-soft' },
  done: { label: 'Выполнена', tone: 'bg-brand-soft' },
  rejected: { label: 'Отклонена', tone: 'bg-alert-soft' },
  cancelled: { label: 'Отменена', tone: 'bg-mist' },
};