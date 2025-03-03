# Database Migrations

This directory contains database migration files that are automatically applied during deployment.

## Migration File Format

Each migration file should:

1. Be named with a version number prefix (e.g., `001_add_columns.sql`, `002_create_table.sql`)
2. Include a description comment at the top
3. Use `IF NOT EXISTS` or similar conditional logic where possible to make migrations idempotent

Example:

```sql
-- Migration: 001_add_columns
-- Description: Adds additional columns to the users table

ALTER TABLE users 
ADD COLUMN IF NOT EXISTS profile_picture TEXT;
```

## How Migrations Work

1. During deployment, the CI/CD pipeline finds all SQL files in this directory
2. Files are sorted by name (numerically)
3. Each file is checked against the `schema_versions` table to see if it has already been applied
4. New migrations are applied in order and recorded in the `schema_versions` table

## Adding New Migrations

To add a new migration:

1. Create a new SQL file with the next sequential number
2. Add a description comment at the top
3. Write your SQL statements using conditional logic where possible
4. Commit the file to the repository

The migration will be automatically applied during the next deployment.

## Tracking Table

Migrations are tracked in the `schema_versions` table:

```sql
CREATE TABLE schema_versions (
    version VARCHAR(50) PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);
``` 