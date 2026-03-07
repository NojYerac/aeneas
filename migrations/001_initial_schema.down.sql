-- Rollback initial schema for aeneas workflow orchestrator
-- Drop tables in reverse order to respect foreign key constraints

DROP TABLE IF EXISTS step_executions;
DROP TABLE IF EXISTS executions;
DROP TABLE IF EXISTS workflows;
