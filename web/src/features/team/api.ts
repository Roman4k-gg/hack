import { apiFetch } from '@/lib/api';
import { reportSchema, type TeamReport } from './types';

export async function fetchTeamReport(): Promise<TeamReport> {
  const payload = await apiFetch<unknown>('/team/interns');

  return reportSchema.parse(payload);
}
