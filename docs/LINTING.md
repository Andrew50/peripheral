# Linting and Code Style Guide

This project enforces **camelCase** naming conventions across all programming languages and file types. The linting setup ensures consistent code style and prevents the use of snake_case.

## Overview

The linting system is configured to run automatically on:
- **Push** to any branch
- **Pull requests** to main branches
- **Manual workflow dispatch**

**Important**: All linters run in **CHECK-ONLY** mode. They will report inconsistencies but will **NOT** modify any code automatically.

## Supported Languages and Tools

### 1. Go (Backend)
- **Tool**: golangci-lint with revive
- **Configuration**: `.golangci.yml`
- **Enforces**: camelCase for variables, functions, and exported names
- **Disallows**: snake_case naming

### 2. Python (Worker)
- **Tool**: Pylint
- **Configuration**: `.pylintrc`
- **Enforces**: camelCase for all identifiers
- **Disallows**: snake_case naming
- **Exceptions**: Standard library names, test files

### 3. SQL (Database)
- **Tool**: SQLFluff
- **Configuration**: `.sqlfluff`
- **Enforces**: camelCase for identifiers and functions
- **Disallows**: snake_case naming
- **Keywords**: UPPER_CASE

### 4. Svelte/TypeScript/JavaScript (Frontend)
- **Tools**: ESLint + Stylelint
- **Configurations**: `.eslintrc.js`, `.stylelintrc.json`
- **Enforces**: camelCase for variables, functions, properties
- **Disallows**: snake_case naming
- **CSS Classes**: camelCase pattern

### 5. YAML/JSON (Configuration)
- **Tool**: yamllint
- **Configuration**: `.yamllint`
- **Enforces**: Consistent formatting and structure

## Workflow File

The main linting workflow is defined in `.github/workflows/format.yaml` and includes:

- **Parallel execution** of all language-specific linters
- **Comprehensive reporting** with detailed error messages
- **Summary job** that provides an overview of all checks
- **Timeout protection** to prevent hanging builds
- **Check-only mode** - no automatic code modifications

## Running Linters Locally

### Go
```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linting
golangci-lint run --config .golangci.yml ./services/backend/...
```

### Python
```bash
# Install Pylint
pip install pylint

# Run linting
pylint services/worker/ --rcfile=.pylintrc
```

### SQL
```bash
# Install SQLFluff
pip install sqlfluff

# Run linting
sqlfluff lint --config .sqlfluff '**/*.sql'
```

### Svelte/TypeScript
```bash
# Install dependencies
cd services/frontend
npm install

# Run ESLint
npx eslint 'src/**/*.{js,ts,svelte}' --config ../../.eslintrc.js

# Run Stylelint
npx stylelint 'src/**/*.{css,svelte}' --config ../../.stylelintrc.json
```

### YAML
```bash
# Install yamllint
pip install yamllint

# Run linting
yamllint -c .yamllint .github/ config/ services/
```

## Naming Convention Rules

### camelCase Examples

✅ **Correct**:
```go
// Go
var userName string
func getUserData() {}
type UserProfile struct {}
```

```python
# Python
userName = "john"
def getUserData():
    pass
class UserProfile:
    pass
```

```sql
-- SQL
SELECT userName, userEmail FROM userTable WHERE userId = 1;
```

```javascript
// JavaScript/TypeScript
const userName = "john";
function getUserData() {}
class UserProfile {}
```

```css
/* CSS */
.userName { color: blue; }
#userProfile { background: white; }
```

❌ **Incorrect** (snake_case):
```go
// Go
var user_name string  // ❌
func get_user_data() {}  // ❌
```

```python
# Python
user_name = "john"  # ❌
def get_user_data():  # ❌
    pass
```

```sql
-- SQL
SELECT user_name, user_email FROM user_table WHERE user_id = 1;  -- ❌
```

```javascript
// JavaScript/TypeScript
const user_name = "john";  // ❌
function get_user_data() {}  // ❌
```

```css
/* CSS */
.user-name { color: blue; }  /* ❌ */
#user-profile { background: white; }  /* ❌ */
```

## Exceptions and Special Cases

### Python
- **Standard library names**: `os`, `sys`, `json`, etc.
- **Third-party library names**: `requests`, `pandas`, etc.
- **Test files**: Excluded from strict naming checks
- **Constants**: Can use UPPER_CASE for true constants

### Go
- **Package names**: Should be lowercase, single word
- **Test files**: Excluded from certain linting rules
- **Generated code**: Excluded from linting

### SQL
- **Keywords**: Must be UPPER_CASE (`SELECT`, `FROM`, `WHERE`)
- **String literals**: Can contain any case
- **Comments**: Excluded from naming checks

### CSS
- **Vendor prefixes**: Allowed for browser compatibility
- **CSS custom properties**: Must follow camelCase
- **Pseudo-classes**: Standard CSS naming (`:hover`, `:focus`)

## Fixing Linting Errors

### Automatic Fixes (Local Development Only)
Some linters support automatic fixing when run locally:

```bash
# Go (limited auto-fix)
golangci-lint run --fix

# Python (no auto-fix in Pylint)
# Manual fixes required

# SQL
sqlfluff fix --config .sqlfluff '**/*.sql'

# JavaScript/TypeScript
npx eslint 'src/**/*.{js,ts,svelte}' --fix

# CSS
npx stylelint 'src/**/*.{css,svelte}' --fix
```

**Note**: CI/CD pipeline runs all linters in check-only mode (`--no-fix` flags) to prevent automatic code modifications.

### Manual Fixes
For issues that can't be auto-fixed:

1. **Read the error message** carefully
2. **Identify the problematic identifier**
3. **Rename to camelCase** following the patterns above
4. **Update all references** to the renamed identifier
5. **Test your changes** to ensure functionality is preserved

## CI/CD Integration

The linting workflow is integrated into the CI/CD pipeline:

1. **Automatic triggering** on code changes
2. **Parallel execution** for faster feedback
3. **Detailed reporting** in GitHub Actions
4. **Blocking PRs** with linting errors
5. **Summary reports** for quick overview
6. **Check-only mode** - no automatic code modifications in CI

## Troubleshooting

### Common Issues

1. **False positives**: Check if the identifier is from a third-party library
2. **Generated code**: Add to ignore patterns if needed
3. **Legacy code**: Gradually migrate to camelCase
4. **Configuration conflicts**: Ensure all config files are properly set

### Getting Help

- Check the linter documentation for specific rules
- Review the configuration files for custom settings
- Look at existing code for examples of correct naming
- Ask the team for guidance on edge cases

## Contributing

When contributing to this project:

1. **Follow camelCase** for all new code
2. **Run linters locally** before submitting PRs
3. **Fix any linting errors** in your changes
4. **Update documentation** if adding new patterns
5. **Test thoroughly** after making naming changes 