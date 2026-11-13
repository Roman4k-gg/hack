import { apiFetch } from '@/lib/api';
import { planItemSchema, planSchema, type ItemStatus, type Plan, type PlanItem } from './types';

export async function fetchPlan(): Promise<Plan> {
  const payload = await apiFetch<unknown>('/onboarding/plan');

  return planSchema.parse(payload);
}

export async function updateItemStatus(
  itemId: string,
  status: ItemStatus,
  blockedReason?: string,
): Promise<PlanItem> {
  const payload = await apiFetch<unknown>(`/onboarding/items/${itemId}`, {
    method: 'PATCH',
    body: JSON.stringify({
      status,
      blocked_reason: status === 'blocked' ? (blockedReason ?? '') : '',
    }),
  });

  return planItemSchema.parse(payload);
}
