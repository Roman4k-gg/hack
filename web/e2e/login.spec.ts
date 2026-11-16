import { expect, test } from '@playwright/test';

const intern = {
    id: '22222222-2222-2222-2222-222222222224',
    email: 'dasha@demo.local',
    full_name: 'Дарья Соколова',
    role: 'intern',
    position: 'Стажёр-разработчик',
    team_name: 'Search',
    scopes: ['onboarding:plan:read', 'onboarding:plan:write', 'chat:use'],
};

const mentor = {
    id: '22222222-2222-2222-2222-222222222221',
    email: 'petr@demo.local',
    full_name: 'Пётр Иванов',
    role: 'mentor',
    position: 'Ведущий разработчик',
    team_name: 'Search',
    scopes: [
        'onboarding:plan:read',
        'onboarding:team:read',
        'analytics:team',
        'chat:use',
    ],
};

async function mockLogin(page: import('@playwright/test').Page, user: typeof intern) {
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

test('стажёр после входа попадает на карту', async ({ page }) => {
    await mockLogin(page, intern);

    await page.goto('/login');

    await page.getByLabel('Почта').fill('dasha@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();

    await expect(page).toHaveURL(/\/map$/);
});

test('наставник после входа попадает на дашборд команды', async ({ page }) => {
    await mockLogin(page, mentor);

    await page.goto('/login');

    await page.getByLabel('Почта').fill('petr@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();

    await expect(page).toHaveURL(/\/dashboard$/);
});

test('неверная почта показывает ошибку', async ({ page }) => {
    await page.route('**/api/v1/auth/login', async (route) => {
        await route.fulfill({
            status: 401,
            contentType: 'application/json',
            body: JSON.stringify({
                error: { code: 'UNAUTHORIZED', message: 'Пользователь не найден' },
            }),
        });
    });

    await page.goto('/login');

    await page.getByLabel('Почта').fill('nobody@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();

    await expect(page.getByText('Не удалось войти. Проверьте почту.')).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
});

test('после выхода вход возвращает на страницу входа', async ({ page }) => {
    await mockLogin(page, intern);

    await page.goto('/login');
    await page.getByLabel('Почта').fill('dasha@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();
    await expect(page).toHaveURL(/\/map$/);

    await page.getByRole('button', { name: 'Выйти' }).click();

    await expect(page).toHaveURL(/\/login$/);
});
