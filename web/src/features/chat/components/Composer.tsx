import { useEffect, useId, useRef, useState } from 'react';
import type { FormEvent, KeyboardEvent } from 'react';

interface ComposerProps {
    onSend: (message: string) => void;
    onStop: () => void;
    disabled?: boolean;
}

export function Composer({
    onSend,
    onStop,
    disabled = false,
}: ComposerProps) {
    const [text, setText] = useState('');
    const inputRef = useRef<HTMLTextAreaElement>(null);
    const wasBusyRef = useRef(disabled);
    const hintId = useId();

    useEffect(() => {
        function handleShortcut(event: globalThis.KeyboardEvent) {
            if (event.isComposing || event.repeat) return;

            if (
                event.code === 'Slash' &&
                (event.ctrlKey || event.metaKey) &&
                !event.altKey &&
                !event.shiftKey
            ) {
                event.preventDefault();

                if (!disabled) {
                    inputRef.current?.focus();
                }

                return;
            }

            if (
                event.key === 'Escape' &&
                !event.ctrlKey &&
                !event.metaKey &&
                !event.altKey &&
                !event.shiftKey &&
                disabled
            ) {
                event.preventDefault();
                onStop();
            }
        }

        window.addEventListener('keydown', handleShortcut);

        return () => {
            window.removeEventListener('keydown', handleShortcut);
        };
    }, [disabled, onStop]);

    useEffect(() => {
        if (wasBusyRef.current && !disabled) {
            const activeElement = document.activeElement;

            if (
                activeElement === document.body ||
                activeElement?.closest('.chat')
            ) {
                inputRef.current?.focus();
            }
        }

        wasBusyRef.current = disabled;
    }, [disabled]);

    function handleSubmit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();

        const message = text.trim();

        if (!message || disabled) return;

        onSend(message);
        setText('');
    }

    function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
        if (
            event.key === 'Enter' &&
            !event.shiftKey &&
            !event.ctrlKey &&
            !event.metaKey &&
            !event.altKey &&
            !event.nativeEvent.isComposing
        ) {
            event.preventDefault();
            event.currentTarget.form?.requestSubmit();
        }
    }

    return (
        <div>
            <form className="composer" onSubmit={handleSubmit}>
                <textarea
                    ref={inputRef}
                    className="composer__input"
                    value={text}
                    onChange={(event) => setText(event.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder="Напишите сообщение..."
                    aria-label="Сообщение"
                    aria-describedby={hintId}
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

            <p
                id={hintId}
                style={{
                    margin: '0 16px 12px',
                    color: '#64748b',
                    fontSize: 12,
                }}
            >
                Enter — отправить · Shift+Enter — новая строка ·
                Esc — остановить ответ · Ctrl+/ — перейти к вводу
            </p>
        </div>
    );
}