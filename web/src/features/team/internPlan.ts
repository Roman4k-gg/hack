import { apiFetch } from '@/lib/api';
import { withDemoFallback } from '@/lib/demo';
import { mockPlan } from '@/features/onboarding/mockPlan';
import { planSchema, type Plan } from '@/features/onboarding/types';

const demoPlan: Plan = planSchema.parse(mockPlan);

export function fetchInternPlan(member: {
  id: string;
  full_name: string;
  team_name: string;
}) {
  return withDemoFallback(
    async () =>
      planSchema.parse(await apiFetch<unknown>(`/team/interns/${member.id}/plan`)),
    {
      ...demoPlan,
      intern: {
        id: member.id,
        full_name: member.full_name,
        team_name: member.team_name,
      },
    },
  );
}