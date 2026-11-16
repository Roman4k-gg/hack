import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchTeamReport } from './api';
import { fetchInternPlan } from './internPlan';

const fetchMock = vi.fn();

beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
    vi.unstubAllGlobals();
});

function response(status: number, body: unknown): Response {
    return {
        ok: status >= 200 && status < 300,
        status,
        text: async () => JSON.stringify(body),
    } as Response;
}

const member = {
    id: '22222222-2222-2222-2222-222222222224',
    full_name: 'Дарья Соколова',
    team_name: 'Search',
};

describe('метрики команды', () => {
    it('разбирает отчёт с бэка', async () => {
        fetchMock.mockResolvedValue(
            response(200, {
                interns: [],
                overview: {
                    interns: 6,
                    avg_progress: 25,
                    avg_pace: 55,
                    blocked: 2,
                    overdue: 5,
                    at_risk: 3,
                    teams: [],
                },
            }),
        );

        const report = await fetchTeamReport();

        expect(report.overview.interns).toBe(6);
        expect(report.interns).toEqual([]);
    });

    it('не скрывает ошибку бэка под демо-режим', async () => {
        fetchMock.mockResolvedValue(response(500, { error: { code: 'INTERNAL_ERROR' } }));

        await expect(fetchTeamReport()).rejects.toThrow();
    });
});

describe('план стажёра для наставника', () => {
    it('берёт план с бэка', async () => {
        fetchMock.mockResolvedValue(
            response(200, {
                id: 'plan-1',
                title: 'Адаптация в команде Search',
                starts_on: '2026-11-10T00:00:00Z',
                stages: [],
                progress: { done: 4, total: 10, percent: 40 },
                next_step: {
                    item_id: 'item-2',
                    title: 'Получить доступ к Jira',
                    reason: 'Обязательный пункт',
                },
                intern: {
                    id: member.id,
                    full_name: member.full_name,
                    team_name: member.team_name,
                    position: 'Стажёр-разработчик',
                },
            }),
        );

        const result = await fetchInternPlan(member);

        expect(result.demo).toBe(false);
        expect(result.data.next_step?.title).toBe('Получить доступ к Jira');
    });

    it('уходит в демо-режим, если бэк не отдаёт эндпоинт', async () => {
        fetchMock.mockResolvedValue(response(404, { error: { code: 'NOT_FOUND' } }));

        const result = await fetchInternPlan(member);

        expect(result.demo).toBe(true);
        expect(result.data.intern.id).toBe(member.id);
        expect(result.data.intern.full_name).toBe(member.full_name);
    });
});
