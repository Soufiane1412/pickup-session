-- Two lifecycles: the session (pickup window) and its last-known positions.
-- Postgres has no native TTL: expiry is enforced ON READ (every SELECT filters on expires_at)
-- and ON PURGE (a job deletes expired rows).

CREATE TABLE location_session (
    booking_id   text        PRIMARY KEY,
    rider_id     text        NOT NULL,
    driver_id    text        NOT NULL,
    state        text        NOT NULL CHECK (state IN ('open', 'closed')),
    expires_at   timestamptz NOT NULL,
    ttl          interval    NOT NULL,
    close_reason text                       -- NULL while open
);

CREATE TABLE last_position (
    booking_id  text             NOT NULL REFERENCES location_session(booking_id) ON DELETE CASCADE,
    user_id     text             NOT NULL,
    lat         double precision NOT NULL,
    lng         double precision NOT NULL,
    seq         bigint           NOT NULL,
    client_at   timestamptz      NOT NULL,
    PRIMARY KEY (booking_id, user_id)
);
