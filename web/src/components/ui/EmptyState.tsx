import type { ReactNode } from 'react';

type EmptyStateProps = {
  title: string;
  text?: string;
  action?: ReactNode;
};

export function EmptyState({ title, text, action }: EmptyStateProps) {
  return (
    <div className="bg-mist p-8">
      <p className="text-xl font-extrabold tracking-tight">{title}</p>
      {text && <p className="mt-2 max-w-md text-sm text-muted">{text}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
