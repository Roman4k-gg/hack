import { useState } from 'react';
import type { FormEvent, KeyboardEvent } from 'react';

interface ComposerProps {
    onSend: (message: string) => void;
    disabled?: boolean;
}

export function Composer({ onSend, disabled = false }: ComposerProps) {
    const [text, setText] = useState('');

    function handleSubmit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();

        const message = text.trim();

        if (!message || disabled) return;

        onSend(message);
        setText('');
    }

    function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
        if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
            event.preventDefault();
            event.currentTarget.form?.requestSubmit();
        }
    }

    return (
        <form className="composer" onSubmit={handleSubmit}>
            <textarea
                className="composer__input"
                value={text}
                onChange={(event) => setText(event.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Напишите сообщение..."
                aria-label="Сообщение"
                disabled={disabled}
                rows={2}
            />

            <button
                className="composer__button"
                type="submit"
                disabled={disabled || !text.trim()}
            >
                Отправить
            </button>
        </form>
    );
}