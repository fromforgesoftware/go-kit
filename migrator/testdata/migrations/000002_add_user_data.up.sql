-- Migration 2: Add test data
-- Set search_path to test schema
SET search_path TO test, public;

INSERT INTO migrator_test_users (username, email) VALUES
    ('testuser1', 'test1@example.com'),
    ('testuser2', 'test2@example.com')
ON CONFLICT (username) DO NOTHING;
