CREATE TABLE posts (
    id                 UUID PRIMARY KEY,
    author_id          UUID NOT NULL,
    title              TEXT NOT NULL,
    body               TEXT NOT NULL,
    comments_disabled  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_posts_created_at_id ON posts (created_at DESC, id DESC);

CREATE TABLE comments (
    id          UUID PRIMARY KEY,
    post_id     UUID NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    parent_id   UUID REFERENCES comments (id) ON DELETE CASCADE,
    author_id   UUID NOT NULL,
    body        VARCHAR(2000) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_comments_post_parent_created ON comments (post_id, parent_id, created_at, id);
