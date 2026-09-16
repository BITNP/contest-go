CREATE TABLE IF NOT EXISTS scores (
    username      text        NOT NULL,
    attempt_no    smallint    NOT NULL,
    score         smallint    NOT NULL,
    submitted_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (username, attempt_no)
);

CREATE INDEX IF NOT EXISTS scores_username_time_idx
    ON scores (username, submitted_at DESC);
