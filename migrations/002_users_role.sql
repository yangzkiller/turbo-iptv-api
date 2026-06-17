ALTER TABLE users
    ADD COLUMN role VARCHAR(10) NOT NULL DEFAULT 'user'
    CHECK (role IN ('admin', 'user'));

CREATE INDEX idx_users_role ON users(role);
