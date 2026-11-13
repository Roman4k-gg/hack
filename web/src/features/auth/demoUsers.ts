import type { User } from './types';

export const demoUsers: User[] = [
  {
    id: 'demo-dasha',
    email: 'dasha@demo.local',
    full_name: 'Дарья Соколова',
    role: 'intern',
    position: 'Стажёр-разработчик',
    team_name: 'Search',
    mentor_name: 'Пётр Иванов',
    scopes: [],
  },
  {
    id: 'demo-petr',
    email: 'petr@demo.local',
    full_name: 'Пётр Иванов',
    role: 'mentor',
    position: 'Ведущий разработчик',
    team_name: 'Search',
    scopes: [],
  },
  {
    id: 'demo-olga',
    email: 'olga@demo.local',
    full_name: 'Ольга Кузнецова',
    role: 'manager',
    position: 'Руководитель команды Search',
    team_name: 'Search',
    scopes: [],
  },
  {
    id: 'demo-admin',
    email: 'admin@demo.local',
    full_name: 'Администратор',
    role: 'admin',
    position: 'Платформа',
    team_name: 'Platform',
    scopes: [],
  },
];
