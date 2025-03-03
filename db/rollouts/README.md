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

## Best Practices for Writing Migrations

To avoid locking issues and ensure migrations complete successfully:

1. **Use Maintenance Mode**: The deployment process automatically scales down services that access the database before running migrations.

2. **Break Down Large Changes**: Split large migrations into smaller, more manageable pieces.

3. **Use Exception Handling**: Wrap ALTER TABLE statements in DO blocks with exception handling:

   ```sql
   DO $$
   BEGIN
       BEGIN
           ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture TEXT;
       EXCEPTION WHEN duplicate_column THEN
           -- Column already exists, do nothing
       END;
   END $$;
   ```

4. **Avoid Long Locks**: Operations that require exclusive locks (like adding columns with default values to large tables) should be done in multiple steps:
   - First add the column without a default value
   - Then update existing rows in batches
   - Finally add a default constraint if needed

5. **Consider Table Size**: For very large tables, consider using background operations like:
   ```sql
   -- Instead of this (locks the table):
   ALTER TABLE large_table ADD COLUMN new_col INT NOT NULL DEFAULT 0;
   
   -- Do this (avoids long locks):
   ALTER TABLE large_table ADD COLUMN new_col INT;
   UPDATE large_table SET new_col = 0;
   ALTER TABLE large_table ALTER COLUMN new_col SET NOT NULL;
   ```

6. **Use Transactions Wisely**: Each migration runs in its own transaction with retry logic.

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