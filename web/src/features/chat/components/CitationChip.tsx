
import type { ChatCitation } from '../types';

interface CitationChipProps {
    citation: ChatCitation;
}

export function CitationChip({ citation }: CitationChipProps) {
    if (!citation.url) {
        return <span className="citation-chip">{citation.title}</span>;
    }

    return (
        <a
            className="citation-chip"
            href={citation.url}
            target="_blank"
            rel="noopener noreferrer"
        >
            {citation.title}
        </a>
    );
}
