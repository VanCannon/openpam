-- Drop tables in reverse order due to foreign key constraints
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS credentials;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS targets;
DROP TABLE IF EXISTS zones;
