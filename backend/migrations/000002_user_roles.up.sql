-- Existing accounts retain administrator access; new accounts default to users.
ALTER TABLE admin_users ADD COLUMN role text NOT NULL DEFAULT 'admin';
ALTER TABLE admin_users ADD CONSTRAINT users_role_check CHECK (role IN ('user', 'admin'));
ALTER TABLE admin_users ALTER COLUMN role SET DEFAULT 'user';
