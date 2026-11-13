import { createFileRoute } from '@tanstack/react-router';
import { RequestsPage } from '@/features/requests/RequestsPage';

export const Route = createFileRoute('/_app/requests')({
  component: RequestsPage,
}); 