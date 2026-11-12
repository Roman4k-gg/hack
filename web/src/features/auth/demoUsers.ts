import type { User } from './types';

export const demoUsers: User[] = [
  {
    id: 'demo-dasha',
    email: 'dasha@demo.local',
    full_name: 'Дарья Соколова',
    role: 'intern',
    position: 'Стажёр-разработчик',
    team_name: 'Search',
    mentor_name: 'Пётр',
  },
  {
    id: 'demo-petr',
    email: 'petr@demo.local',
    full_name: 'Пётр Наставников',
    role: 'mentor',
    position: 'Старший разработчик',
    team_name: 'Search',
  },
  {
    id: 'demo-admin',
    email: 'admin@demo.local',
    full_name: 'Администратор',
    role: 'admin',
    position: 'HR-партнёр',
  },
];
