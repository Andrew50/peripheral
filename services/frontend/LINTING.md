# Frontend Linting Guide

## Current Status

The frontend codebase currently has a number of ESLint errors that have been temporarily suppressed to allow CI/CD pipelines to pass. This document provides guidance on how to gradually fix these issues.

## Common ESLint Errors

The most frequent issues are:

1. **Unused Variables**: `@typescript-eslint/no-unused-vars`
   - Remove unused imports and variables
   - If needed for future use, prefix with an underscore (e.g., `_unusedVar`)

2. **Any Types**: `@typescript-eslint/no-explicit-any`
   - Replace `any` with more specific types
   - If the type is complex, consider creating an interface

3. **Unused Expressions**: `@typescript-eslint/no-unused-expressions`
   - Ensure all expressions are properly used (typically in assignments or function calls)

4. **Accessibility Issues**: `svelte/valid-compile` (a11y errors)
   - Add proper ARIA roles to interactive elements
   - Use semantic HTML (buttons for actions, etc.)

5. **Unused CSS**: `svelte/valid-compile` (css-unused-selector)
   - Remove unused CSS classes
   - Or ensure they are used in the component

## How to Fix Issues

We've added a command to automatically fix issues where possible:

```bash
npm run lint:fix
```

For a more systematic approach:

1. Tackle one file at a time
2. Start with the most frequently modified files
3. Use TypeScript properly - avoid `any` types
4. Remove dead code and unused variables
5. Fix accessibility issues

## Long-term Plan

1. **Re-enable Rules Gradually**: Our temporary solution disables problematic rules, but we should re-enable them one by one as issues are fixed
2. **Add Pre-commit Hooks**: Consider adding pre-commit hooks to prevent new issues
3. **Set Up Regular Linting Reviews**: Schedule periodic reviews of linting issues

## Testing After Fixes

Always test changes after fixing linting issues, as removing unused code or changing types might expose bugs or edge cases that were previously hidden. 