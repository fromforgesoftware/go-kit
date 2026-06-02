-- Migration 1: Create test table
-- Set search_path to test schema
SET search_path TO test, public;

CREATE TABLE IF NOT EXISTS migrator_test_users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
