import { mockPlan } from './mockPlan';
import { planSchema, type Plan } from './types';

export async function fetchPlan(): Promise<Plan> {
  await new Promise((resolve) => setTimeout(resolve, 300));
  return planSchema.parse(mockPlan);
}
