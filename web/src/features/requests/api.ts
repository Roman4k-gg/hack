import { apiFetch } from '@/lib/api';
import { withDemoFallback } from '@/lib/demo';
import { mockRequests } from './mock';
import { requestListSchema } from './types';

export function fetchRequests() {
  return withDemoFallback(
    async () => requestListSchema.parse(await apiFetch<unknown>('/requests')),
    mockRequests,
  );
}