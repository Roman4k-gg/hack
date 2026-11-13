export const mockPlan = {
  intern: {
    id: '22222222-2222-2222-2222-222222222204',
    full_name: 'Дарья Соколова',
    team_name: 'Search',
  },
  stages: [
    {
      id: 'stage-1',
      title: 'Неделя 1 · Окружение',
      description: 'Доступы, ноутбук и рабочее окружение.',
      position: 1,
      items: [
        {
          id: 'item-01',
          title: 'Получить ноутбук и доступ к VPN',
          description: 'Выдаёт IT-отдел, доступ приходит на рабочую почту.',
          kind: 'access',
          status: 'done',
        },
        {
          id: 'item-02',
          title: 'Настроить рабочее окружение',
          description: 'Go, Docker, редактор и доступы к репозиторию.',
          kind: 'task',
          status: 'done',
        },
        {
          id: 'item-03',
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
      items: [
        {
          id: 'item-04',
          title: 'Познакомиться с командой',
          description: 'Представиться в командном чате и на дейлике.',
          kind: 'meeting',
          status: 'done',
        },
        {
          id: 'item-05',
          title: 'Встреча 1:1 с наставником',
          description: 'Обсудить ожидания, цели на месяц и формат поддержки.',
          kind: 'meeting',
          status: 'done',
        },
        {
          id: 'item-06',
          title: 'Прочитать гайд по код-ревью',
          description: 'Правила ревью, размер PR и работа с комментариями.',
          kind: 'doc',
          status: 'pending',
        },
      ],
    },
    {
      id: 'stage-3',
      title: 'Неделя 2 · Процессы',
      description: 'Как устроены разработка, ревью и релизы.',
      position: 3,
      items: [
        {
          id: 'item-07',
          title: 'Пройти курс по безопасности',
          description: 'Обязательный курс для всех новых сотрудников.',
          kind: 'task',
          status: 'blocked',
          blocked_reason: 'Ждём подтверждения от HR',
        },
        {
          id: 'item-08',
          title: 'Разобраться с процессом релизов',
          description: 'Как устроены ветки, стенды и деплой.',
          kind: 'doc',
          status: 'pending',
        },
        {
          id: 'item-09',
          title: 'Сделать первый PR',
          description: 'Небольшая правка с ревью наставника.',
          kind: 'task',
          status: 'in_progress',
        },
      ],
    },
    {
      id: 'stage-4',
      title: 'Месяц 1 · Первые задачи',
      description: 'Первый вклад в продукт.',
      position: 4,
      items: [
        {
          id: 'item-10',
          title: 'Взять первую задачу из бэклога',
          description: 'Задача с меткой good first issue в команде Search.',
          kind: 'task',
          status: 'pending',
        },
      ],
    },
  ],
  progress: { done: 4, total: 10, percent: 40 },
  next_step: {
    item_id: 'item-03',
    title: 'Получить доступ к Jira',
    reason: 'Нужен для работы с задачами команды Search.',
  },
};
