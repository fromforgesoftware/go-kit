CREATE SCHEMA IF NOT EXISTS "test";
CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;

-- Table for fixtures package testing (in test schema)
CREATE TABLE IF NOT EXISTS test.fixtures_test (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
