import { createFileRoute } from '@tanstack/react-router';
import { TeamDashboard } from '@/features/team/TeamDashboard';

export const Route = createFileRoute('/_app/dashboard')({
  component: DashboardPage,
});

function DashboardPage() {
  return <TeamDashboard />;
}
