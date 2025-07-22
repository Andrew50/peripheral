#!/usr/bin/env bash
set -eo pipefail

DB_NAME="${1:-postgres}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"
DB_PORT_INT="${DB_PORT_INT:-5433}"

# ─────────────────────────  helpers  ──────────────────────────
log()       { printf '[%(%F %T)T] MIGRATION: %s\n' -1 "$1"; }
error_log() { printf '[%(%F %T)T] MIGRATION ERROR: %s\n' -1 "$1" >&2; }

# extract numeric prefix before optional separator (001, 12_add_users, etc.)
extract_version() {
  local file="$1"
  file="${file##*/}"         # strip path
  file="${file%.sql}"        # strip extension
  [[ $file =~ ^([0-9]+) ]] && printf '%d' "${BASH_REMATCH[1]}"
}

# extract description from first comment line
extract_description() {
  local file="$1"
  local desc
  desc=$(head -1 "$file" 2>/dev/null | sed 's/^--[[:space:]]*//' || echo "")
  if [[ $desc =~ ^Description:[[:space:]]*(.*) ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  else
    printf 'Migration %s' "$(basename "$file")"
  fi
}

PSQL() { PGPASSWORD=$POSTGRES_PASSWORD psql -qAt -U postgres -h localhost -p "$DB_PORT_INT" -d "$DB_NAME" "$@"; }

# ───────────────────────  sanity checks  ──────────────────────
TABLE_EXISTS=$(PSQL -c "SELECT to_regclass('public.schema_versions') IS NOT NULL;")

if [[ $TABLE_EXISTS != "t" ]]; then
  error_log "schema_versions table does not exist – aborting."
  exit 1
fi

CURRENT_VERSION=$(PSQL -c "SELECT COALESCE(MAX(version), 0) FROM schema_versions;")
log "Current schema version: $CURRENT_VERSION"

# ──────────────────────  collect migrations  ──────────────────
if [[ ! -d $MIGRATIONS_DIR ]]; then
  error_log "Migrations directory $MIGRATIONS_DIR not found."
  exit 1
fi

mapfile -t MIGRATION_FILES < <(find "$MIGRATIONS_DIR" -type f -name '*.sql' | sort -V)

if [[ ${#MIGRATION_FILES[@]} -eq 0 ]]; then
  log "No migration files found – nothing to do."
  exit 0
fi

# ──────────────────────  apply migrations  ────────────────────
for FILE in "${MIGRATION_FILES[@]}"; do
  VERSION=$(extract_version "$FILE") || continue
  [[ -n $VERSION ]] || continue  # skip files without numeric prefix
  (( VERSION > CURRENT_VERSION )) || continue

  DESCRIPTION=$(extract_description "$FILE")
  log "Applying migration $VERSION from $(basename "$FILE")"

  # Check if this version already exists
  EXISTING=$(PSQL -c "SELECT COUNT(*) FROM schema_versions WHERE version = $VERSION;")
  if [[ $EXISTING -gt 0 ]]; then
    log "Migration $VERSION already applied, skipping."
    continue
  fi

  # -- run the file and capture full output -----------------------------
  if OUTPUT=$(PSQL -v ON_ERROR_STOP=1 -f "$FILE" 2>&1); then
      # Check if the migration file already inserted the version record
      EXISTING_AFTER=$(PSQL -c "SELECT COUNT(*) FROM schema_versions WHERE version = $VERSION;")
      if [[ $EXISTING_AFTER -eq 0 ]]; then
        # Migration file didn't insert version record, so we do it
        PSQL -c "INSERT INTO schema_versions(version, description)
                 VALUES ($VERSION, \$\$${DESCRIPTION}\$\$);"
      fi
      CURRENT_VERSION=$VERSION
      log "Migration $VERSION applied successfully."
  else
      error_log "Failed to apply migration $VERSION – aborting."
      error_log "psql said:\n${OUTPUT}"
      exit 1
  fi
done

log "All pending migrations executed."
PSQL -c "SELECT version, applied_at, description FROM schema_versions ORDER BY version;"

