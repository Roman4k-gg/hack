import { createFileRoute } from '@tanstack/react-router';
import { OnboardingMap } from '@/features/onboarding/OnboardingMap';

export const Route = createFileRoute('/_app/map')({
  component: OnboardingMap,
});
