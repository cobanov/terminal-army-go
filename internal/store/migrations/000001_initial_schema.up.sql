-- Initial schema, consolidates the seven Python alembic migrations
-- (0001 init through 0007 planet_code) into one Go migration.
-- All formulas and game constants are documented in CLAUDE.md and verified
-- against OGame Fandom Wiki: https://ogame.fandom.com/wiki/OGame_Wiki

CREATE TABLE universes (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(64) NOT NULL UNIQUE,
    speed_economy   INTEGER NOT NULL DEFAULT 1,
    speed_fleet     INTEGER NOT NULL DEFAULT 1,
    speed_research  INTEGER NOT NULL DEFAULT 1,
    galaxies_count  INTEGER NOT NULL DEFAULT 9,
    systems_count   INTEGER NOT NULL DEFAULT 499,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
    id                  SERIAL PRIMARY KEY,
    username            VARCHAR(64) NOT NULL UNIQUE,
    email               VARCHAR(255) NOT NULL UNIQUE,
    password_hash       VARCHAR(255) NOT NULL,
    role                VARCHAR(16) NOT NULL DEFAULT 'player',
    current_universe_id INTEGER REFERENCES universes(id),
    last_seen_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ix_users_username ON users(username);
CREATE INDEX ix_users_email    ON users(email);

CREATE TABLE planets (
    id                         SERIAL PRIMARY KEY,
    code                       VARCHAR(8) NOT NULL UNIQUE,
    owner_user_id              INTEGER NOT NULL REFERENCES users(id),
    universe_id                INTEGER NOT NULL REFERENCES universes(id),
    galaxy                     INTEGER NOT NULL,
    system                     INTEGER NOT NULL,
    position                   INTEGER NOT NULL,
    name                       VARCHAR(64) NOT NULL DEFAULT 'Homeworld',
    fields_used                INTEGER NOT NULL DEFAULT 0,
    fields_total               INTEGER NOT NULL DEFAULT 160,
    temp_min                   INTEGER NOT NULL DEFAULT 0,
    temp_max                   INTEGER NOT NULL DEFAULT 40,
    resources_metal            DOUBLE PRECISION NOT NULL DEFAULT 500,
    resources_crystal          DOUBLE PRECISION NOT NULL DEFAULT 500,
    resources_deuterium        DOUBLE PRECISION NOT NULL DEFAULT 0,
    resources_last_updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_planet_coord UNIQUE (universe_id, galaxy, system, position)
);
CREATE INDEX ix_planets_owner    ON planets(owner_user_id);
CREATE INDEX ix_planets_universe ON planets(universe_id);
CREATE INDEX ix_planets_code     ON planets(code);

CREATE TABLE buildings (
    id            SERIAL PRIMARY KEY,
    planet_id     INTEGER NOT NULL REFERENCES planets(id) ON DELETE CASCADE,
    building_type VARCHAR(32) NOT NULL,
    level         INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT uq_planet_building_type UNIQUE (planet_id, building_type)
);
CREATE INDEX ix_buildings_planet ON buildings(planet_id);

CREATE TABLE researches (
    id        SERIAL PRIMARY KEY,
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tech_type VARCHAR(32) NOT NULL,
    level     INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT uq_user_tech UNIQUE (user_id, tech_type)
);
CREATE INDEX ix_researches_user ON researches(user_id);

CREATE TABLE build_queue (
    id             SERIAL PRIMARY KEY,
    planet_id      INTEGER REFERENCES planets(id) ON DELETE CASCADE,
    user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    queue_type     VARCHAR(16) NOT NULL,
    item_key       VARCHAR(32) NOT NULL,
    target_level   INTEGER NOT NULL DEFAULT 0,
    count          INTEGER NOT NULL DEFAULT 1,
    cost_metal     INTEGER NOT NULL DEFAULT 0,
    cost_crystal   INTEGER NOT NULL DEFAULT 0,
    cost_deuterium INTEGER NOT NULL DEFAULT 0,
    started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at    TIMESTAMPTZ NOT NULL,
    cancelled      BOOLEAN NOT NULL DEFAULT FALSE,
    applied        BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX ix_queue_user              ON build_queue(user_id);
CREATE INDEX ix_queue_planet_finished   ON build_queue(planet_id, finished_at);
CREATE INDEX ix_queue_cancelled_applied ON build_queue(cancelled, applied, finished_at);

CREATE TABLE messages (
    id           SERIAL PRIMARY KEY,
    sender_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    recipient_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject      VARCHAR(255) NOT NULL DEFAULT '',
    body         VARCHAR(2000) NOT NULL,
    category     VARCHAR(32) NOT NULL DEFAULT 'player',
    read         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ix_messages_sender_id         ON messages(sender_id);
CREATE INDEX ix_messages_recipient_created ON messages(recipient_id, created_at);
CREATE INDEX ix_messages_recipient_unread  ON messages(recipient_id, read);

CREATE TABLE device_sessions (
    id         SERIAL PRIMARY KEY,
    code       VARCHAR(64) NOT NULL UNIQUE,
    token      TEXT,
    user_id    INTEGER REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX ix_device_sessions_code ON device_sessions(code);

CREATE TABLE planet_ships (
    id        SERIAL PRIMARY KEY,
    planet_id INTEGER NOT NULL REFERENCES planets(id) ON DELETE CASCADE,
    ship_type VARCHAR(32) NOT NULL,
    count     INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT uq_planet_ship_type UNIQUE (planet_id, ship_type)
);
CREATE INDEX ix_planet_ships_planet ON planet_ships(planet_id);

CREATE TABLE planet_defenses (
    id           SERIAL PRIMARY KEY,
    planet_id    INTEGER NOT NULL REFERENCES planets(id) ON DELETE CASCADE,
    defense_type VARCHAR(32) NOT NULL,
    count        INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT uq_planet_defense_type UNIQUE (planet_id, defense_type)
);
CREATE INDEX ix_planet_defenses_planet ON planet_defenses(planet_id);

CREATE TABLE fleets (
    id                SERIAL PRIMARY KEY,
    owner_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    origin_planet_id  INTEGER NOT NULL REFERENCES planets(id),
    mission           VARCHAR(16) NOT NULL,
    status            VARCHAR(16) NOT NULL DEFAULT 'outbound',
    universe_id       INTEGER NOT NULL REFERENCES universes(id),
    target_galaxy     INTEGER NOT NULL,
    target_system     INTEGER NOT NULL,
    target_position   INTEGER NOT NULL,
    target_planet_id  INTEGER REFERENCES planets(id),
    speed_percent     INTEGER NOT NULL DEFAULT 100,
    departure_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    arrival_at        TIMESTAMPTZ NOT NULL,
    return_at         TIMESTAMPTZ,
    cargo_metal       INTEGER NOT NULL DEFAULT 0,
    cargo_crystal     INTEGER NOT NULL DEFAULT 0,
    cargo_deuterium   INTEGER NOT NULL DEFAULT 0,
    fuel_cost         INTEGER NOT NULL DEFAULT 0,
    arrival_processed BOOLEAN NOT NULL DEFAULT FALSE,
    return_processed  BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX ix_fleets_owner          ON fleets(owner_id);
CREATE INDEX ix_fleets_origin         ON fleets(origin_planet_id);
CREATE INDEX ix_fleets_status_arrival ON fleets(status, arrival_at);
CREATE INDEX ix_fleets_status_return  ON fleets(status, return_at);

CREATE TABLE fleet_ships (
    id        SERIAL PRIMARY KEY,
    fleet_id  INTEGER NOT NULL REFERENCES fleets(id) ON DELETE CASCADE,
    ship_type VARCHAR(32) NOT NULL,
    count     INTEGER NOT NULL
);
CREATE INDEX ix_fleet_ships_fleet ON fleet_ships(fleet_id);

CREATE TABLE reports (
    id              SERIAL PRIMARY KEY,
    owner_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    report_type     VARCHAR(16) NOT NULL,
    title           VARCHAR(255) NOT NULL,
    body            TEXT NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}'::jsonb,
    target_galaxy   INTEGER NOT NULL DEFAULT 0,
    target_system   INTEGER NOT NULL DEFAULT 0,
    target_position INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ix_reports_owner ON reports(owner_id);

CREATE TABLE alliances (
    id          SERIAL PRIMARY KEY,
    tag         VARCHAR(6) NOT NULL UNIQUE,
    name        VARCHAR(64) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    founder_id  INTEGER NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ix_alliances_tag ON alliances(tag);

CREATE TABLE alliance_members (
    id          SERIAL PRIMARY KEY,
    alliance_id INTEGER NOT NULL REFERENCES alliances(id) ON DELETE CASCADE,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        VARCHAR(16) NOT NULL DEFAULT 'member',
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_alliance_member_user UNIQUE (user_id)
);
CREATE INDEX ix_alliance_members_alliance ON alliance_members(alliance_id);

CREATE TABLE alliance_join_requests (
    id          SERIAL PRIMARY KEY,
    alliance_id INTEGER NOT NULL REFERENCES alliances(id) ON DELETE CASCADE,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_alliance_join_request_pair UNIQUE (alliance_id, user_id)
);
CREATE INDEX ix_alliance_join_requests_alliance ON alliance_join_requests(alliance_id);
CREATE INDEX ix_alliance_join_requests_user     ON alliance_join_requests(user_id);
