# `input.svelte` Component Analysis

## Overview
The `input.svelte` component is a complex popup input interface for entering and validating financial data with multiple input types. It serves as a modal interface for capturing specific input types like tickers, timestamps, timeframes, and prices.

## Key Functionality

### State Management
- Uses Svelte stores for state management with an `inputQuery` store
- Tracks input status through states: `inactive`, `initializing`, `active`, `complete`, `cancelled`, and `shutdown`
- Manages input validation state with `inputValid` flag

### Input Types
- Supports various input types:
  - `ticker`: Stock ticker symbols
  - `timestamp`: Date/time values
  - `timeframe`: Time periods like "1d", "1w", etc.
  - `price`: Monetary values
  - `extendedHours`: Boolean toggle

### Focus Management Strategy
The component employs a specific focus management approach:
1. Stores the previously focused element when activated
2. Uses a hidden input element positioned on top to capture keyboard events
3. Uses document click handlers to detect clicks outside the input window
4. Attempts to restore focus when completed or cancelled
5. Cleans up event listeners when destroyed

### Input Validation
- Performs asynchronous validation based on input type
- For tickers, makes backend API calls to validate and fetch security details
- Validates timestamps against date format patterns
- Validates timeframes and prices against regex patterns

### UI Components
- Main popup container with header, search bar, and content area
- Security selection interface for ticker inputs
- Timestamp input interface with date picker
- Field selector for manually choosing input type

### Event Handling
- Handles keyboard events (Enter, Escape, Tab, etc.)
- Processes alphanumeric input for the active input type
- Handles outside clicks to dismiss the popup
- Manages touch events

## Current Issues

### Keyboard Focus Capture
The component currently captures all keyboard inputs, even when it might not be appropriate:
- Uses a hidden input element positioned at the top level
- All keyboard events are routed through this input
- May prevent other UI elements from receiving keyboard focus/input

### Focus Restoration
When the component is closed:
- Attempts to restore focus to previously focused element
- Uses blur() on hidden input before focus restoration
- May have issues with proper focus management in certain scenarios

## Technical Implementation

### Module Context Script
- Defines interfaces, stores, and utility functions
- Manages promise-based input querying with `queryInstanceInput` function
- Handles cancellation of active promises when new input is requested

### Component Script
- Manages component lifecycle with onMount and onDestroy
- Implements input validation and processing
- Handles UI state changes based on input status
- Manages focus and keyboard event capture

### Rendering Logic
- Conditionally renders the popup based on input status
- Dynamically shows different interfaces based on input type
- Displays validation state and security selection interfaces

## Focus Management Details
The focus management is implemented with these key parts:

```javascript
// Store previously focused element
prevFocusedElement = document.activeElement as HTMLElement;

// Focus hidden input
const input = document.getElementById('hidden-input');
if (input) {
  input.focus();
}

// When shutting down
if (hiddenInput && document.activeElement === hiddenInput) {
  hiddenInput.blur();
}

// Restore focus
prevFocusedElement?.focus();
```

This approach creates a hidden input that captures all keyboard events while the component is active. 