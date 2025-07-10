# Database Service (`services/db`)

This directory contains the configuration, initialization scripts, migrations, and Dockerfiles for the PostgreSQL/TimescaleDB database service.

## Overview

The database service uses TimescaleDB (built on PostgreSQL) for storing time-series data and relational data. It includes automated initialization, migrations, and backup capabilities.

## Configuration

-   **`config/dev.conf`**: PostgreSQL configuration optimized for development environments. Used by `Dockerfile.dev`.
-   **`config/prod.conf`**: PostgreSQL configuration optimized for production environments. Used by `Dockerfile.prod`.
-   Configuration settings are copied into the container at build time.

## Initialization

-   **`init/init.sql`**: Executed automatically by the `docker-entrypoint.sh` script when the database container starts for the *first time* or when the data volume is empty. It sets up the initial schema, including the `schema_versions` table required for migrations.
-   **`init/securities.csv`**: Contains initial seed data for securities, loaded by `init.sql`.

## Migrations

-   **Purpose**: To manage incremental changes to the database schema after initial setup.
-   **Location**: SQL files are placed in the `migrations/` directory.
-   **Naming**: Files must be named `<version_number>.sql` (e.g., `1.sql`, `2.sql`). The version number must be an integer.
-   **Execution**:
    1.  Migrations are copied into the `/migrations` directory within the Docker image.
    2.  The `scripts/start.sh` script runs `scripts/migrate.sh` *after* the initial database startup (or temporary startup).
    3.  `migrate.sh` checks the `schema_versions` table for the current version and applies any migrations with a higher version number found in `/migrations`, sorted numerically.
    4.  Each applied migration's version and description (extracted from the first comment line) are recorded in `schema_versions`.
-   **Adding Migrations**:
    1.  Determine the next sequential version number.
    2.  Create a new file `<next_version>.sql` in `migrations/`.
    3.  Add a description comment (e.g., `-- Description: Add index to trades table`).
    4.  Write your idempotent SQL statements.
    5.  Rebuild the Docker image.
-   **Checking Status**: Connect to the database and run `SELECT version, applied_at, description FROM schema_versions ORDER BY version;`.

## Startup Process (`scripts/start.sh`)

This script is the container's entrypoint. It performs the following steps:

1.  Starts a *temporary* PostgreSQL instance in the background.
2.  Waits for the temporary instance to become ready.
3.  Executes the `scripts/migrate.sh` script against the temporary instance.
4.  If migrations succeed, it cleanly shuts down the temporary instance.
5.  Uses `exec` to replace itself with the final `docker-entrypoint.sh postgres` command, making the main PostgreSQL server PID 1 within the container. This ensures proper signal handling.

## Backup (`scripts/backup-improved.sh`)

-   This script performs a comprehensive `pg_dump` of the database with verification and error handling.
-   It's intended to be run periodically (e.g., via `cron` within the container or an external scheduler).
-   Backups are timestamped, stored in the `/backups` volume, compressed, verified for integrity, and older backups are pruned automatically.

## Dockerfiles

-   **`Dockerfile.dev`**: Builds the development image using `config/dev.conf`. Includes scripts for startup and migration.
-   **`Dockerfile.prod`**: Builds the production image using `config/prod.conf`. Structure is similar to the dev Dockerfile but uses production settings.

