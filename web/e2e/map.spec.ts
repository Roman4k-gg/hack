import { expect, test } from '@playwright/test';

const plan = {
    id: 'plan-1',
    title: 'Адаптация в команде Search',
    starts_on: '2026-11-10T00:00:00Z',
    stages: [
        {
            id: 'stage-1',
            title: 'Неделя 1 · Окружение',
            description: 'Доступы, ноутбук и рабочее окружение.',
            position: 1,
            items: [
                {
                    id: 'item-1',
                    title: 'Получить ноутбук и доступ к VPN',
                    description: 'Выдаёт IT-отдел.',
                    kind: 'access',
                    status: 'done',
                },
                {
                    id: 'item-2',
                    title: 'Получить доступ к Jira',
                    description: 'Нужен для работы с задачами команды Search.',
                    kind: 'access',
                    status: 'in_progress',
                },
            ],
        },
        {
            id: 'stage-2',
            title: 'Неделя 1 · Команда',
            description: 'Знакомство с командой и наставником.',
            position: 2,
            items: [],
        },
    ],
    progress: { done: 4, total: 10, percent: 40 },
    next_step: {
        item_id: 'item-2',
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

async function loginAsIntern(page: import('@playwright/test').Page) {
    await page.route('**/api/v1/auth/login', async (route) => {
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
                token: 'test-token',
                token_type: 'Bearer',
                expires_at: '2026-11-17T12:00:00Z',
                user: {
                    id: '22222222-2222-2222-2222-222222222224',
                    email: 'dasha@demo.local',
                    full_name: 'Дарья Соколова',
                    role: 'intern',
                    position: 'Стажёр-разработчик',
                    team_name: 'Search',
                    scopes: ['onboarding:plan:read', 'onboarding:plan:write', 'chat:use'],
                },
            }),
        });
    });
}

test('карта показывает этапы, прогресс и следующий шаг', async ({ page }) => {
    await loginAsIntern(page);

    await page.route('**/api/v1/onboarding/plan', async (route) => {
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(plan),
        });
    });

    await page.goto('/login');
    await page.getByLabel('Почта').fill('dasha@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();

    await expect(page.getByRole('heading', { name: 'Привет, Дарья!' })).toBeVisible();
    await expect(page.getByText('Следующий шаг')).toBeVisible();
    await expect(page.getByText('Получить доступ к Jira').first()).toBeVisible();
    await expect(page.getByText('40%')).toBeVisible();
    await expect(page.getByText('Неделя 1 · Команда')).toBeVisible();
});

test('статус пункта меняется и запрос уходит в api', async ({ page }) => {
    await loginAsIntern(page);

    await page.route('**/api/v1/onboarding/plan', async (route) => {
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(plan),
        });
    });

    let patched = false;
    let patchedBody = '';

    await page.route('**/api/v1/onboarding/items/*', async (route) => {
        patched = true;
        patchedBody = route.request().postData() ?? '';

        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
                ...plan.stages[0].items[1],
                status: 'done',
            }),
        });
    });

    await page.goto('/login');
    await page.getByLabel('Почта').fill('dasha@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();

    await expect(page.getByRole('heading', { name: 'Привет, Дарья!' })).toBeVisible();

    await page.getByRole('button', { name: 'Готово' }).first().click();

    await expect.poll(() => patched).toBe(true);
    expect(patchedBody).toContain('"status":"done"');
});

test('карта показывает ошибку, если api недоступен', async ({ page }) => {
    await loginAsIntern(page);

    await page.route('**/api/v1/onboarding/plan', async (route) => {
        await route.fulfill({
            status: 500,
            contentType: 'application/json',
            body: JSON.stringify({ error: { code: 'INTERNAL_ERROR', message: 'Ошибка' } }),
        });
    });

    await page.goto('/login');
    await page.getByLabel('Почта').fill('dasha@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();

    await expect(page.getByText('Не удалось загрузить карту')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Повторить' })).toBeVisible();
});
