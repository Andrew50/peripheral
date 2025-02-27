
# Contribution Guidelines

This document outlines our basic guidelines for contributing to the project. Please follow these practices to keep our work clear and consistent.

---

## Branch Management (GitFlow)

We follow the GitFlow branching model for our development process:

### Main Branches
- **main**: Production-ready code only. Protected branch.
- **develop**: Main development branch. All feature branches start from here.

### Supporting Branches
- **feature/**: New features and non-emergency bug fixes
  - Branch from: `develop`
  - Merge back into: `develop`
  - Naming: `feature/description-of-feature`

- **hotfix/**: Urgent fixes for production issues
  - Branch from: `main`
  - Merge back into: `main` and `develop`
  - Naming: `hotfix/description-of-fix`

- **release/**: Preparing new production releases
  - Branch from: `develop`
  - Merge back into: `main` and `develop`
  - Naming: `release/version-number`

**Branch Naming Examples**:
- `feature/login-page`
- `hotfix/security-vulnerability`
- `release/1.2.0`

### Branch Lifecycle
1. Create branch from appropriate source
2. Develop and commit changes
3. Create pull request
4. Address review feedback
5. Merge into target branch(es)

---

## Commit Messages

- **Format**: Use consistent tense (either past or present) and be descriptive
- **Summary**: Keep the first line short (around 50 characters)
- **Details**: Optionally include more details and reference related issues

**Examples**:
```
# Past tense example:
Added login functionality

Implemented JWT authentication and included unit tests.
Fixes #123

# Present tense example:
Add login functionality

Implement JWT authentication and include unit tests.
Fixes #123
```

---

## Pull Requests

- **Preparation**: 
  - For features: ensure your branch is updated with `develop`
  - For hotfixes: ensure your branch is updated with `main`
  - All tests must pass
- **Description**: Briefly describe what was changed and why
- **Process**: Address any feedback and merge once approved
- **Merging**:
  - Features merge into `develop`
  - Hotfixes merge into both `main` and `develop`
  - Releases merge into both `main` and `develop`

---

## Development Setup

- Follow the instructions in [Setup.md](setup.md) for setting up your environment
- Use `.env` files for local configuration when needed
- Run tests locally to ensure everything works as expected

---

## Documentation

- **Updates**: Keep documentation current with code changes
- **Location**: Main docs are in the `/docs` directory, with README files in key project folders
- **Version Tags**: Update version numbers in documentation when creating releases
