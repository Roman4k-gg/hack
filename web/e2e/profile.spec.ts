import { expect, test } from '@playwright/test';

const intern = {
    id: '22222222-2222-2222-2222-222222222224',
    email: 'dasha@demo.local',
    full_name: 'Дарья Соколова',
    role: 'intern',
    position: 'Стажёр-разработчик',
    team_name: 'Search',
    scopes: ['onboarding:plan:read', 'chat:use'],
};

const mentor = {
    id: '22222222-2222-2222-2222-222222222221',
    email: 'petr@demo.local',
    full_name: 'Пётр Иванов',
    role: 'mentor',
    position: 'Ведущий разработчик',
    team_name: 'Search',
    scopes: ['onboarding:plan:read', 'chat:use'],
};

test('при смене профиля история чата очищается', async ({ page }) => {
    await page.route('**/api/v1/auth/login', async (route) => {
        const email = (route.request().postDataJSON() as { email: string }).email;
        const user = email === mentor.email ? mentor : intern;

        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
                token: `token-${email}`,
                token_type: 'Bearer',
                expires_at: '2026-11-17T12:00:00Z',
                user,
            }),
        });
    });

    await page.route('**/api/v1/chat/stream', async (route) => {
        await route.fulfill({
            status: 200,
            contentType: 'text/event-stream',
            body: [
                'event: message',
                'data: {"type":"delta","text":"Спроси наставника"}',
                '',
                'event: message',
                'data: {"type":"done"}',
                '',
                '',
            ].join('\n'),
        });
    });

    await page.goto('/login');
    await page.getByLabel('Почта').fill(intern.email);
    await page.getByRole('button', { name: 'Войти' }).click();

    await page.getByRole('link', { name: 'Чат' }).click();

    await page.getByRole('textbox', { name: 'Сообщение', exact: true }).fill('Где взять доступ к Jira?');
    await page.getByRole('button', { name: 'Отправить' }).click();

    await expect(page.getByText('Спроси наставника')).toBeVisible();

    await page.getByRole('button', { name: 'Выйти' }).click();
    await expect(page).toHaveURL(/\/login$/);

    await page.getByLabel('Почта').fill(mentor.email);
    await page.getByRole('button', { name: 'Войти' }).click();

    await expect(page.getByText('Пётр Иванов')).toBeVisible();
    await expect(page).toHaveURL(/\/dashboard$/);

    await page.getByRole('link', { name: 'Чат' }).click();

    await expect(page).toHaveURL(/\/chat$/);
    await expect(page.getByText('Спроси наставника')).toHaveCount(0);
    await expect(page.getByText('Где взять доступ к Jira?')).toHaveCount(0);
});
