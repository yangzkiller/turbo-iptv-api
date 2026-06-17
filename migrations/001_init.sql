CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    first_name VARCHAR(120) NOT NULL,
    last_name VARCHAR(120) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_playlists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    source_type VARCHAR(10) NOT NULL CHECK (source_type IN ('xc', 'm3u')),
    dns TEXT,
    xc_username TEXT,
    xc_password TEXT,
    m3u_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE favorites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    playlist_id UUID NOT NULL REFERENCES user_playlists(id) ON DELETE CASCADE,
    content_key VARCHAR(64) NOT NULL,
    name TEXT NOT NULL,
    category TEXT,
    logo TEXT,
    url TEXT NOT NULL,
    content_type VARCHAR(10) NOT NULL CHECK (content_type IN ('live', 'movie', 'series')),
    series_name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, playlist_id, content_key)
);

CREATE TABLE watch_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    playlist_id UUID NOT NULL REFERENCES user_playlists(id) ON DELETE CASCADE,
    content_key VARCHAR(64) NOT NULL,
    name TEXT NOT NULL,
    category TEXT,
    logo TEXT,
    url TEXT NOT NULL,
    content_type VARCHAR(10) NOT NULL CHECK (content_type IN ('live', 'movie', 'series')),
    series_name TEXT,
    season INT,
    episode INT,
    position_seconds DOUBLE PRECISION NOT NULL DEFAULT 0,
    duration_seconds DOUBLE PRECISION,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, playlist_id, content_key)
);

CREATE INDEX idx_user_playlists_user_id ON user_playlists(user_id);
CREATE INDEX idx_favorites_user_playlist ON favorites(user_id, playlist_id);
CREATE INDEX idx_watch_progress_user_playlist ON watch_progress(user_id, playlist_id);
CREATE INDEX idx_watch_progress_recent ON watch_progress(user_id, playlist_id, updated_at DESC);
