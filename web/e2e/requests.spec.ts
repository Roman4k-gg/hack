import { expect, test } from '@playwright/test';

const requestItem = {
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
                    scopes: ['onboarding:plan:read', 'request:self', 'chat:use'],
                },
            }),
        });
    });
}

test('список заявок показывает заявку со статусом', async ({ page }) => {
    await loginAsIntern(page);

    await page.route('**/api/v1/requests', async (route) => {
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ items: [requestItem] }),
        });
    });

    await page.goto('/login');
    await page.getByLabel('Почта').fill('dasha@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();

    await page.getByRole('link', { name: 'Заявки' }).click();

    await expect(page.getByRole('heading', { name: 'Мои заявки' })).toBeVisible();
    await expect(page.getByText('Отгул 20.11.2026')).toBeVisible();
    await expect(page.getByText('На рассмотрении')).toBeVisible();
    await expect(page.getByText('ABS-1025')).toBeVisible();
});

test('пустой список заявок показывает подсказку', async ({ page }) => {
    await loginAsIntern(page);

    await page.route('**/api/v1/requests', async (route) => {
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ items: [] }),
        });
    });

    await page.goto('/login');
    await page.getByLabel('Почта').fill('dasha@demo.local');
    await page.getByRole('button', { name: 'Войти' }).click();

    await page.getByRole('link', { name: 'Заявки' }).click();

    await expect(page.getByRole('heading', { name: 'Мои заявки' })).toBeVisible();
    await expect(page.getByText('Заявок пока нет')).toBeVisible();
});
