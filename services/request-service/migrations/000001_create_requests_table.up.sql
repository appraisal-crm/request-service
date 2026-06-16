CREATE TABLE object_types (
    id TEXT PRIMARY KEY
);

CREATE TABLE request_statuses (
    id         TEXT PRIMARY KEY,
    sort_order INTEGER NOT NULL
);

INSERT INTO object_types (id) VALUES
    ('apartment'),
    ('house'),
    ('land'),
    ('commercial'),
    ('car');

INSERT INTO request_statuses (id, sort_order) VALUES
    ('new',                   1),
    ('in_progress',           2),
    ('inspection_scheduled',  3),
    ('inspection_completed',  4),
    ('appraisal',             5),
    ('report_sent',           6),
    ('closed',                7);

CREATE TABLE requests (
    id           UUID PRIMARY KEY,
    client_id    UUID NOT NULL,
    inspector_id UUID,
    object_type  TEXT REFERENCES object_types(id),
    address      TEXT,
    status       TEXT NOT NULL DEFAULT 'new' REFERENCES request_statuses(id),
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_requests_client_id ON requests (client_id);
CREATE INDEX idx_requests_status ON requests (status);
