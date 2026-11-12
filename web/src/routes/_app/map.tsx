import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/_app/map')({
  component: MapPage,
});

function MapPage() {
  return (
    <section>
      <p className="text-xs text-muted">Неделя 1 · команда Search</p>
      <h1 className="mt-1 text-3xl font-extrabold tracking-tight">Привет, Даша</h1>
    </section>
  );
}