-- Post-migration: Record completion
INSERT INTO migration_tracking (event) VALUES ('post-migration-executed');
