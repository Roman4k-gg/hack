import type { RequestItem } from './types';

export const mockRequests: RequestItem[] = [
  {
    id: 'ABS-0003',
    type: 'absence',
    title: 'Отгул 21 ноября',
    summary: 'Один день за свой счёт.',
    status: 'pending',
    created_at: '2026-11-14T09:20:00Z',
  },
  {
    id: 'ACC-0002',
    type: 'access',
    title: 'Доступ к GitLab (группа Search)',
    summary: 'Роль Developer, выдаёт IT-отдел.',
    status: 'done',
    created_at: '2026-11-11T12:05:00Z',
  },
  {
    id: 'ABS-0001',
    type: 'absence',
    title: 'Отгул 3 ноября',
    summary: 'Один день, личные дела.',
    status: 'approved',
    created_at: '2026-10-30T15:40:00Z',
  },
];