import { expect, test } from '@playwright/test';

for (const scenario of [
    { status: 401, message: 'Сессия истекла. Войди заново.' },
    { status: 403, message: 'Для твоей роли чат недоступен.' },
    {
        status: 429,
        message: 'Слишком много запросов. Попробуй немного позже.',
    },
]) {
    test(`ошибка ${scenario.status} не подменяется моками`, async ({ page }) => {
        await page.route('**/api/v1/chat/stream', (route) =>
            route.fulfill({
                status: scenario.status,
                contentType: 'application/json',
                body: '{}',
            }),
        );

        await page.goto('/');
        await page.getByRole('textbox', {
            name: 'Сообщение',
            exact: true,
        }).fill('Покажи мой план адаптации');

        await page.getByRole('button', { name: 'Отправить' }).click();

        await expect(page.getByRole('alert')).toContainText(scenario.message);
        await expect(page.locator('.chat__notice')).toHaveCount(0);
        await expect(page.getByRole('textbox', {
            name: 'Сообщение',
            exact: true,
        })).toBeEnabled();
    });
}

test('оборванный ответ сохраняется и допускает повтор', async ({ page }) => {
    let attempts = 0;

    await page.route('**/api/v1/chat/stream', (route) => {
        attempts += 1;

        const events = [
            {
                type: 'delta',
                text: attempts === 1
                    ? 'Познакомься с наставником.'
                    : 'Проверь карту адаптации.',
            },
            ...(attempts > 1 ? [{ type: 'done' }] : []),
        ];

        return route.fulfill({
            status: 200,
            contentType: 'text/event-stream',
            body: events.map((event) =>
                `event: message\ndata: ${JSON.stringify(event)}\n\n`,
            ).join(''),
        });
    });

    await page.goto('/');
    const input = page.getByRole('textbox', {
        name: 'Сообщение',
        exact: true,
    });

    await input.fill('Что делать в первый день?');
    await page.getByRole('button', { name: 'Отправить' }).click();

    await expect(page.getByRole('alert')).toContainText('Ответ прервался');
    await expect(page.locator('.message-bubble--assistant'))
        .toContainText('Познакомься с наставником.');
    await expect(input).toBeEnabled();
    await expect(page.getByRole('button', { name: 'Остановить' }))
        .toHaveCount(0);

    await page.getByRole('button', { name: 'Повторить' }).click();

    await expect(page.locator('.message-bubble--assistant'))
        .toContainText('Проверь карту адаптации.');
    await expect(page.getByRole('alert')).toHaveCount(0);
    await expect(page.locator('.message-bubble--user')).toHaveCount(1);
    expect(attempts).toBe(2);
});

test('недоступность AI включает обозначенный демо-режим', async ({ page }) => {
    await page.route('**/api/v1/chat/stream', (route) =>
        route.fulfill({
            status: 503,
            contentType: 'application/json',
            body: '{}',
        }),
    );

    await page.goto('/');
    await page.getByRole('textbox', {
        name: 'Сообщение',
        exact: true,
    }).fill('Что делать в первый день?');

    await page.getByRole('button', { name: 'Отправить' }).click();

    await expect(page.locator('.chat__notice')).toContainText('Демо-режим');
    await expect(page.getByRole('button', { name: 'Остановить' })).toBeVisible();
    await page.getByRole('button', { name: 'Остановить' }).click();

    await expect(page.getByRole('textbox', {
        name: 'Сообщение',
        exact: true,
    })).toBeEnabled();
});