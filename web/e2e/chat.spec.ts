import { expect, test } from '@playwright/test';

test('стажёр получает план с инструментом и источником', async ({ page }) => {
    let requestBody: unknown;
    let requestMethod = '';

    const events = [
        {
            type: 'tool_call',
            toolCall: {
                id: 'tool-1',
                name: 'get_onboarding_plan',
                status: 'running',
            },
        },
        {
            type: 'tool_call',
            toolCall: {
                id: 'tool-1',
                name: 'get_onboarding_plan',
                status: 'success',
            },
        },
        {
            type: 'delta',
            text: '## Первый день\n\n',
        },
        {
            type: 'delta',
            text: 'Познакомься с **наставником** и проверь доступы.',
        },
        {
            type: 'citation',
            citation: {
                id: 'source-1',
                title: 'План первой недели',
                url: 'https://example.com/onboarding',
            },
        },
        {
            type: 'done',
        },
    ];

    await page.route('**/api/v1/chat/stream', async (route) => {
        requestMethod = route.request().method();
        requestBody = route.request().postDataJSON();

        await route.fulfill({
            status: 200,
            contentType: 'text/event-stream',
            body: events
                .map((event) =>
                    `event: ${event.type}\ndata: ${JSON.stringify(event)}\n\n`,
                )
                .join(''),
        });
    });

    await page.goto('/');

    await expect(
        page.getByRole('heading', { name: 'Привет! Я AI-buddy' }),
    ).toBeVisible();

    const input = page.getByRole('textbox', {
        name: 'Сообщение',
        exact: true,
    });

    await input.fill('Что делать в первый день?');
    await page.getByRole('button', { name: 'Отправить' }).click();

    await expect(
        page.getByRole('heading', { name: 'Первый день', exact: true }),
    ).toBeVisible();

    expect(requestMethod).toBe('POST');
    expect(requestBody).toEqual({
        message: 'Что делать в первый день?',
    });

    await expect(
        page.locator('.message-bubble--user'),
    ).toHaveText('Что делать в первый день?');

    await expect(
        page.locator('.message-bubble__markdown strong'),
    ).toHaveText('наставником');

    await expect(
        page.locator('.tool-call-card__name'),
    ).toHaveText('get_onboarding_plan');

    await expect(
        page.locator('.tool-call-card__status'),
    ).toHaveText('Выполнено');

    await expect(
        page.getByRole('link', { name: 'План первой недели' }),
    ).toHaveAttribute('href', 'https://example.com/onboarding');

    await expect(input).toBeEnabled();
    await expect(
        page.getByRole('button', { name: 'Остановить' }),
    ).toHaveCount(0);

    await expect(page.locator('.chat__notice')).toHaveCount(0);
});