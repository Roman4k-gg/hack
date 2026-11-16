import { expect, test } from '@playwright/test';

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
                id: 'plan-1',
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
                    item_id: 'item-7',
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

async function login(page: import('@playwright/test').Page, user: Record<string, unknown>) {
    await page.route('**/api/v1/auth/login', async (route) => {
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
                token: 'test-token',
                token_type: 'Bearer',
                expires_at: '2026-11-17T12:00:00Z',
                user,
            }),
        });
    });
}

test('наставник видит стажёров с метриками', async ({ page }) => {
    await login(page, {
        id: '22222222-2222-2222-2222-222222222221',
        email: 'petr@demo.local',
        full_name: 'Пётр Иванов',
        role: 'mentor',
        position: 'Ведущий разработчик',
        team_name: 'Search',
        scopes: ['onboarding:team:read', 'analytics:team'],
    });

    await page.route('**/api/v1/team/interns', async (route) => {
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(teamReport),
        });
    });

    await page.goto('/login');
    await page.getByLabel('Почта').fill('petr@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();

    await expect(page.getByRole('heading', { name: 'Команда' })).toBeVisible();
    await expect(page.getByText('Дарья Соколова')).toBeVisible();
    await expect(page.getByText('Пройти курс по безопасности')).toBeVisible();
    await expect(page.getByText('Ждём подтверждения от HR')).toBeVisible();
    await expect(page.getByText('40%').first()).toBeVisible();
});

test('стажёру раздел команды недоступен', async ({ page }) => {
    await login(page, {
        id: '22222222-2222-2222-2222-222222222224',
        email: 'dasha@demo.local',
        full_name: 'Дарья Соколова',
        role: 'intern',
        position: 'Стажёр-разработчик',
        team_name: 'Search',
        scopes: ['onboarding:plan:read', 'chat:use'],
    });

    await page.goto('/login');
    await page.getByLabel('Почта').fill('dasha@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();

    await expect(page).toHaveURL(/\/map$/);
    await expect(page.getByRole('link', { name: 'Команда' })).toHaveCount(0);
});
