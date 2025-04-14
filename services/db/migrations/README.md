# Database Migrations

This directory contains SQL migration files that are applied to the database in order.

## Migration Naming Convention

Migrations are named using a simple numeric format:

```
<version_number>.sql
```

For example:
- `1.sql`
- `2.sql`
- `3.sql`

The version number must be a valid number and will be stored in the `schema_versions` table.

## Migration File Format

Each migration file should:

1. Start with a description comment
2. Be idempotent when possible (use IF NOT EXISTS, etc.)
3. Include only one logical change per file

Example:

```sql
-- Description: Add email column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS email VARCHAR(255);
```

## How Migrations Work

1. The database container initializes with `init.sql`, which creates the base schema
2. Migrations are baked into the container image during build in the `/migrations` directory
3. The `start.sh` script starts PostgreSQL and runs migrations once at startup
4. The `migrate.sh` script processes files in the `/migrations` directory
5. New migrations are applied in order and recorded in the `schema_versions` table

## Deployment Process

During deployment, the CI/CD pipeline:

1. Builds a new database image with migrations included
2. Scales down services that access the database to prevent conflicts
3. Restarts the database pod, which automatically runs migrations on startup
4. Scales services back up

## Adding New Migrations

To add a new migration:

1. Determine the next version number by checking the highest number in existing migrations
2. Create a new file named `<next_version>.sql` in this directory
3. Add your SQL statements with appropriate description comments
4. Test the migration locally before committing
5. Rebuild and redeploy the database image for the changes to take effect

## Migration Status

To check the status of applied migrations, run:

```sql
SELECT version, applied_at, description FROM schema_versions ORDER BY version;
```

## Tools

- `rename_migrations.sh`: Helper script to convert old format migrations to the new format
- `fix_version_format.sql`: Special migration to fix schema_versions table if needed 