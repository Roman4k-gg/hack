import { createFileRoute } from '@tanstack/react-router';
import { InternPlanPage } from '@/features/team/InternPlanPage';

export const Route = createFileRoute('/_app/team/$internId')({
  component: InternPlanRoute,
});

function InternPlanRoute() {
  const { internId } = Route.useParams();

  return <InternPlanPage internId={internId} />;
}