ALTER TABLE kb_documents
    ADD COLUMN IF NOT EXISTS owner_email text NOT NULL DEFAULT '';

ALTER TABLE kb_documents
    ADD COLUMN IF NOT EXISTS visibility text NOT NULL DEFAULT 'team';

ALTER TABLE kb_documents
    DROP CONSTRAINT IF EXISTS kb_documents_visibility_check;

ALTER TABLE kb_documents
    ADD CONSTRAINT kb_documents_visibility_check
    CHECK (visibility IN ('public', 'team', 'hr'));

CREATE TABLE IF NOT EXISTS doc_chunks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id uuid NOT NULL REFERENCES kb_documents (id) ON DELETE CASCADE,
    chunk_index integer NOT NULL,
    content     text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (document_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS kb_documents_visibility_idx ON kb_documents (visibility);
CREATE INDEX IF NOT EXISTS doc_chunks_document_idx ON doc_chunks (document_id);
