# Input Window Focus Management Solutions

## Problem Statement
The `input.svelte` component currently captures all keyboard events when active, preventing other UI elements from receiving keyboard input. This creates a poor user experience when the input window is open but users need to interact with other parts of the application.

## Current Implementation Issues
1. Uses a hidden input element that captures all keyboard events
2. Sets the hidden input to cover the entire component (width/height: 100%)
3. Relies on focus/blur and manual event propagation control
4. May not properly handle focus restoration in all cases

## Proposed Solutions

### Solution 1: Targeted Event Listener Approach
**Core idea**: Instead of using a hidden input to capture keyboard events, use a direct event listener on the component.

```javascript
// Replace the hidden input approach with:
onMount(() => {
  // Store previous focus
  prevFocusedElement = document.activeElement as HTMLElement;
  
  // Add keyboard listener to the component itself
  const inputWindow = document.getElementById('input-window');
  if (inputWindow) {
    inputWindow.addEventListener('keydown', handleKeyDown);
    // Set tabindex to make it focusable
    inputWindow.setAttribute('tabindex', '0');
    // Focus the component
    inputWindow.focus();
  }
});

// Modify handleKeyDown to check if the event is meant for this component
function handleKeyDown(event: KeyboardEvent) {
  if ($inputQuery.status !== 'active') return;
  
  // Process the event without stopping propagation for all keys
  // Only stop propagation for keys specifically handled by this component
  if (['Enter', 'Escape', 'Tab', 'Backspace'].includes(event.key) ||
      /^[a-zA-Z0-9]$/.test(event.key) || 
      /[-:.]/.test(event.key)) {
    event.stopPropagation();
    // Handle the input...
  }
  // Let other events pass through
}
```

### Solution 2: Modal Dialog with Focus Trap
**Core idea**: Implement a proper focus trap that only affects tabbing within the modal, not all keyboard events.

```javascript
// Use a focus trap library or implement a custom one
import { createFocusTrap } from 'focus-trap';

let focusTrap: any = null;

onMount(() => {
  // Store previous focus
  prevFocusedElement = document.activeElement as HTMLElement;
  
  // Create a focus trap when the component becomes active
  const inputWindow = document.getElementById('input-window');
  if (inputWindow && $inputQuery.status === 'active') {
    focusTrap = createFocusTrap(inputWindow, {
      initialFocus: '#visible-input',
      escapeDeactivates: true,
      onDeactivate: () => {
        inputQuery.update(q => ({ ...q, status: 'cancelled' }));
      }
    });
    focusTrap.activate();
  }
});

// Ensure proper cleanup
onDestroy(() => {
  if (focusTrap) {
    focusTrap.deactivate();
  }
});
```

### Solution 3: Hybrid Approach with Component-Specific Input Fields
**Core idea**: Use regular input fields that only capture events when focused, with proper tab indexes.

```svelte
<div class="popup-container" id="input-window">
  <!-- Other content -->
  
  <input
    type="text"
    id="visible-input"
    class="search-input"
    value={$inputQuery.inputString}
    on:keydown={handleKeyDown}
    tabindex="0"
    autocomplete="off"
  />
  
  <!-- Make all interactive elements properly tabbable -->
  <div class="content-container" tabindex="-1">
    <!-- Content with proper tab indexes -->
  </div>
</div>
```

### Solution 4: Event Delegation with Selective Capture
**Core idea**: Use event delegation at the component level but selectively determine which events to capture.

```javascript
function handleComponentEvent(event: Event) {
  // Only handle events if component is active
  if ($inputQuery.status !== 'active') return;
  
  // For keyboard events on inputs, process them
  if (event.type === 'keydown' && 
      (event.target as HTMLElement).tagName === 'INPUT') {
    handleKeyDown(event as KeyboardEvent);
  }
  
  // Let other events propagate normally
}

onMount(() => {
  const inputWindow = document.getElementById('input-window');
  if (inputWindow) {
    inputWindow.addEventListener('keydown', handleComponentEvent);
    inputWindow.addEventListener('click', handleComponentEvent);
  }
});
```

### Solution 5: Use the Dialog Element with Proper Focus Management
**Core idea**: Leverage the HTML dialog element which has built-in focus management.

```svelte
<dialog
  id="input-dialog"
  class="popup-container"
  open={$inputQuery.status === 'active' || $inputQuery.status === 'initializing'}
>
  <!-- Dialog content -->
  <form method="dialog">
    <!-- Input fields -->
    <button type="submit" on:click={() => closeWindow()}>Close</button>
  </form>
</dialog>

<script>
  onMount(() => {
    const dialog = document.getElementById('input-dialog');
    if (dialog && ($inputQuery.status === 'active' || $inputQuery.status === 'initializing')) {
      (dialog as HTMLDialogElement).showModal();
    }
  });
  
  function closeWindow() {
    const dialog = document.getElementById('input-dialog');
    if (dialog) {
      (dialog as HTMLDialogElement).close();
      inputQuery.update((v) => ({ ...v, status: 'cancelled' }));
    }
  }
</script>
```

## Recommended Approach
Based on the analysis, **Solution 5** using the HTML dialog element is the most robust approach:

1. **Benefits**:
   - Native browser focus management
   - Built-in modal behavior
   - Automatic focus restoration
   - Respects accessibility standards

2. **Implementation Steps**:
   - Replace the current div-based popup with a dialog element
   - Use the dialog's native open/close methods
   - Remove the custom focus management code
   - Maintain existing input handlers within the dialog

3. **Fallback**: If dialog element support is a concern, implement Solution 2 with a focus trap library.

## Additional Considerations
- Test with screen readers and keyboard-only navigation
- Ensure proper accessibility attributes (aria-* attributes)
- Consider adding keyboard shortcuts for common actions
- Add visual indicators for keyboard focus states 