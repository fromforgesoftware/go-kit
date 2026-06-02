-- Test fixture: Create table and insert test data
-- search_path is set to "test,public" by sqldbtest
-- But CREATE TABLE needs explicit schema or SET search_path first
SET search_path TO test, public;

CREATE TABLE IF NOT EXISTS fixtures_test (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO fixtures_test (name, value) VALUES 
    ('fixture_1', 'Test value 1'),
    ('fixture_2', 'Test value 2'),
    ('fixture_3', 'Test value 3');
