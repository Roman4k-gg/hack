
export interface SSEEvent {
    event: string;
    data: string;
    id?: string;
}

export function createSSEParser(onEvent: (event: SSEEvent) => void) {
    let lineBuffer = '';
    let dataLines: string[] = [];
    let eventName = '';
    let eventId: string | undefined;
    let pendingCR = false;

    function dispatchEvent() {
        if (dataLines.length > 0) {
            onEvent({
                event: eventName || 'message',
                data: dataLines.join('\n'),
                ...(eventId !== undefined ? { id: eventId } : {}),
            });
        }

        dataLines = [];
        eventName = '';
    }

    function processLine(line: string) {
        if (line === '') {
            dispatchEvent();
            return;
        }

        if (line.startsWith(':')) {
            return;
        }

        const colonIndex = line.indexOf(':');

        const field = colonIndex === -1
            ? line
            : line.slice(0, colonIndex);

        let value = colonIndex === -1
            ? ''
            : line.slice(colonIndex + 1);

        if (value.startsWith(' ')) {
            value = value.slice(1);
        }

        switch (field) {
            case 'data':
                dataLines.push(value);
                break;

            case 'event':
                eventName = value;
                break;

            case 'id':
                if (!value.includes('\0')) {
                    eventId = value;
                }
                break;
        }
    }

    function feed(chunk: string) {
        for (const char of chunk) {
            if (pendingCR) {
                pendingCR = false;

                if (char === '\n') {
                    continue;
                }
            }

            if (char === '\r') {
                processLine(lineBuffer);
                lineBuffer = '';
                pendingCR = true;
            } else if (char === '\n') {
                processLine(lineBuffer);
                lineBuffer = '';
            } else {
                lineBuffer += char;
            }
        }
    }

    function flush() {
        if (lineBuffer !== '') {
            processLine(lineBuffer);
            lineBuffer = '';
        }

        dispatchEvent();
        pendingCR = false;
    }

    return { feed, flush };
}
