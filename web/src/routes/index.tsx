import { createFileRoute } from '@tanstack/react-router';
import { Chat } from '../features/chat/Chat';

export const Route = createFileRoute('/')({
  component: ChatPage,
});

function ChatPage() {
  return (
    <main style={{ height: '100vh' }}>
      <Chat />
    </main>
  );
}