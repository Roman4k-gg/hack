import { createFileRoute } from '@tanstack/react-router';
import { Chat } from '@/features/chat/Chat';

export const Route = createFileRoute('/_app/chat')({
  component: ChatPage,
});

function ChatPage() {
  return (
    <section className="flex min-h-0 flex-1 flex-col">
      <h1 className="mb-4 text-3xl font-extrabold tracking-tight">Чат с AI-buddy</h1>
      <Chat />
    </section>
  );
}
