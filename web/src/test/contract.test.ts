import { describe, expect, it } from 'vitest';
import { planSchema } from '@/features/onboarding/types';
import { requestListSchema } from '@/features/requests/types';
import { reportSchema } from '@/features/team/types';

const teamReport = {
    interns: [
        {
            id: '22222222-2222-2222-2222-222222222224',
            full_name: 'Дарья Соколова',
            email: 'dasha@demo.local',
            position: 'Стажёр-разработчик',
            team: { id: '11111111-1111-1111-1111-111111111111', name: 'Search', slug: 'search' },
            mentor: { id: '22222222-2222-2222-2222-222222222221', full_name: 'Пётр Иванов' },
            plan: {
                id: '33333333-3333-3333-3333-333333333331',
                title: 'Адаптация в команде Search',
                starts_on: '2026-11-10',
                current_stage: 'Неделя 1 · Окружение',
            },
            progress: {
                done: 4,
                total: 10,
                pending: 3,
                in_progress: 2,
                blocked: 1,
                skipped: 0,
                percent: 40,
            },
            pace: { expected: 3, completed: 3, percent: 100 },
            blockers: [
                {
                    item_id: '44444444-4444-4444-4444-444444444407',
                    title: 'Пройти курс по безопасности',
                    reason: 'Ждём подтверждения от HR',
                },
            ],
            overdue: [],
            activity: { questions: 3, last_event_at: '2026-11-14T09:52:12Z' },
            at_risk: true,
        },
    ],
    overview: {
        interns: 6,
        avg_progress: 25,
        avg_pace: 55,
        blocked: 2,
        overdue: 5,
        at_risk: 3,
        teams: [
            {
                id: '11111111-1111-1111-1111-111111111111',
                name: 'Search',
                interns: 3,
                avg_progress: 33,
                avg_pace: 66,
                blocked: 1,
                overdue: 0,
            },
        ],
    },
};

const plan = {
    id: '33333333-3333-3333-3333-333333333331',
    title: 'Адаптация в команде Search',
    starts_on: '2026-11-10T00:00:00Z',
    stages: [
        {
            id: '33333333-3333-3333-3333-333333333341',
            title: 'Неделя 1 · Окружение',
            description: 'Доступы, ноутбук и рабочее окружение.',
            position: 1,
            items: [
                {
                    id: '44444444-4444-4444-4444-444444444401',
                    title: 'Получить ноутбук и доступ к VPN',
                    description: 'Выдаёт IT-отдел, доступ приходит на рабочую почту.',
                    kind: 'access',
                    status: 'done',
                    blocked_reason: null,
                },
            ],
        },
    ],
    progress: { done: 4, total: 10, percent: 40 },
    next_step: {
        item_id: '44444444-4444-4444-4444-444444444403',
        title: 'Получить доступ к Jira',
        reason: 'Обязательный пункт этапа «Неделя 1 · Окружение»',
    },
    intern: {
        id: '22222222-2222-2222-2222-222222222224',
        full_name: 'Дарья Соколова',
        team_name: 'Search',
        position: 'Стажёр-разработчик',
    },
};

const requests = {
    items: [
        {
            request_id: '9d2e1f4a-2b3c-4d5e-8f9a-1b2c3d4e5f60',
            id: '9d2e1f4a-2b3c-4d5e-8f9a-1b2c3d4e5f60',
            kind: 'absence',
            type: 'absence',
            title: 'Отгул 20.11.2026',
            status: 'pending',
            external_id: 'ABS-1025',
            number: 'ABS-1025',
            approver: { id: '22222222-2222-2222-2222-222222222226', full_name: 'Ольга Кузнецова' },
            created_at: '2026-11-14T09:40:34Z',
            updated_at: '2026-11-14T09:40:34Z',
        },
    ],
};

describe('схемы фронта против ответов бэка', () => {
    it('парсит /team/interns', () => {
        const parsed = reportSchema.parse(teamReport);

        expect(parsed.interns).toHaveLength(1);
        expect(parsed.interns[0].progress.percent).toBe(40);
        expect(parsed.interns[0].pace.percent).toBe(100);
        expect(parsed.interns[0].at_risk).toBe(true);
        expect(parsed.overview.interns).toBe(6);
    });

    it('парсит /onboarding/plan', () => {
        const parsed = planSchema.parse(plan);

        expect(parsed.intern.full_name).toBe('Дарья Соколова');
        expect(parsed.next_step?.title).toBe('Получить доступ к Jira');
        expect(parsed.stages[0].items[0].status).toBe('done');
        expect(parsed.progress.percent).toBe(40);
    });

    it('парсит /requests', () => {
        const parsed = requestListSchema.parse(requests);

        expect(parsed).toHaveLength(1);
        expect(parsed[0].id).toBe('9d2e1f4a-2b3c-4d5e-8f9a-1b2c3d4e5f60');
        expect(parsed[0].type).toBe('absence');
        expect(parsed[0].number).toBe('ABS-1025');
        expect(parsed[0].status).toBe('pending');
    });

    it('отклоняет чужой формат заявки', () => {
        expect(() =>
            requestListSchema.parse({
                items: [{ request_id: 'x', status: 'pending', created_at: '2026-11-14T09:40:34Z' }],
            }),
        ).toThrow();
    });

    it('отклоняет неизвестный статус пункта', () => {
        expect(() =>
            planSchema.parse({
                ...plan,
                stages: [
                    {
                        ...plan.stages[0],
                        items: [{ ...plan.stages[0].items[0], status: 'archived' }],
                    },
                ],
            }),
        ).toThrow();
    });
});
