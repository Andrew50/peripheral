<script lang="ts">
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { goto } from '$app/navigation';
    import { getNote, updateNote, deleteNote } from '$lib/services/notes';
    import type { Note } from '$lib/core/types';

    let note: Note | null = null;
    let loading = true;
    let error: string | null = null;
    let editedNote = {
        noteId: 0,
        title: '',
        content: '',
        category: '',
        tags: [] as string[],
        isPinned: false,
        isArchived: false
    };
    let newTag = '';
    let saving = false;

    onMount(async () => {
        const noteId = parseInt($page.params.id);
        if (isNaN(noteId)) {
            error = 'Invalid note ID';
            loading = false;
            return;
        }

        await loadNote(noteId);
    });

    async function loadNote(noteId: number) {
        loading = true;
        error = null;
        try {
            note = await getNote(noteId);
            
            // Initialize the edited note with the loaded note data
            editedNote = {
                noteId: note.noteId,
                title: note.title,
                content: note.content,
                category: note.category,
                tags: [...note.tags],
                isPinned: note.isPinned,
                isArchived: note.isArchived
            };
        } catch (err) {
            error = err instanceof Error ? err.message : 'Failed to load note';
            console.error('Error loading note:', err);
        } finally {
            loading = false;
        }
    }

    async function handleSave() {
        if (!editedNote.title.trim()) {
            error = 'Title is required';
            return;
        }

        saving = true;
        error = null;
        try {
            note = await updateNote(editedNote);
            // Update the edited note with the latest data
            editedNote = {
                noteId: note.noteId,
                title: note.title,
                content: note.content,
                category: note.category,
                tags: [...note.tags],
                isPinned: note.isPinned,
                isArchived: note.isArchived
            };
        } catch (err) {
            error = err instanceof Error ? err.message : 'Failed to save note';
            console.error('Error saving note:', err);
        } finally {
            saving = false;
        }
    }

    async function handleDelete() {
        if (!note) return;
        
        if (!confirm('Are you sure you want to delete this note?')) {
            return;
        }
        
        try {
            await deleteNote(note.noteId);
            goto('/app/notes');
        } catch (err) {
            error = err instanceof Error ? err.message : 'Failed to delete note';
            console.error('Error deleting note:', err);
        }
    }

    function addTag() {
        if (newTag && !editedNote.tags.includes(newTag)) {
            editedNote.tags = [...editedNote.tags, newTag];
            newTag = '';
        }
    }

    function removeTag(tag: string) {
        editedNote.tags = editedNote.tags.filter(t => t !== tag);
    }

    function formatDate(dateString: string): string {
        const date = new Date(dateString);
        return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
    }
</script>

<div class="note-detail-container">
    <div class="note-detail-header">
        <div class="header-left">
            <button class="btn-secondary" on:click={() => goto('/app/notes')}>← Back to Notes</button>
            <h1>{note ? 'Edit Note' : 'Note'}</h1>
        </div>
        <div class="header-actions">
            {#if note}
                <button class="btn-danger" on:click={handleDelete}>Delete</button>
                <button class="btn-primary" on:click={handleSave} disabled={saving}>
                    {saving ? 'Saving...' : 'Save Changes'}
                </button>
            {/if}
        </div>
    </div>

    {#if error}
        <div class="error-message">
            <p>{error}</p>
            <button on:click={() => error = null}>Dismiss</button>
        </div>
    {/if}

    {#if loading}
        <div class="loading">Loading note...</div>
    {:else if note}
        <div class="note-form">
            <div class="form-group">
                <label for="title">Title *</label>
                <input type="text" id="title" bind:value={editedNote.title} required />
            </div>
            
            <div class="form-group">
                <label for="content">Content</label>
                <textarea id="content" bind:value={editedNote.content} rows="12"></textarea>
            </div>
            
            <div class="form-group">
                <label for="category">Category</label>
                <input type="text" id="category" bind:value={editedNote.category} />
            </div>
            
            <div class="form-group">
                <label>Tags</label>
                <div class="tag-input">
                    <input type="text" bind:value={newTag} placeholder="Add a tag" />
                    <button type="button" on:click={addTag}>Add</button>
                </div>
                <div class="tags-container">
                    {#each editedNote.tags as tag}
                        <span class="tag">
                            {tag}
                            <button type="button" on:click={() => removeTag(tag)}>×</button>
                        </span>
                    {/each}
                </div>
            </div>
            
            <div class="form-options">
                <div class="checkbox-group">
                    <label>
                        <input type="checkbox" bind:checked={editedNote.isPinned} />
                        Pin this note
                    </label>
                </div>
                
                <div class="checkbox-group">
                    <label>
                        <input type="checkbox" bind:checked={editedNote.isArchived} />
                        Archive this note
                    </label>
                </div>
            </div>
            
            <div class="note-metadata">
                <p>Created: {formatDate(note.createdAt)}</p>
                <p>Last updated: {formatDate(note.updatedAt)}</p>
            </div>
        </div>
    {:else}
        <div class="empty-state">
            <p>Note not found or you don't have permission to view it.</p>
        </div>
    {/if}
</div>

<style>
    .note-detail-container {
        max-width: 800px;
        margin: 0 auto;
        padding: 20px;
    }

    .note-detail-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 20px;
    }

    .header-left {
        display: flex;
        align-items: center;
        gap: 15px;
    }

    .header-left h1 {
        margin: 0;
    }

    .header-actions {
        display: flex;
        gap: 10px;
    }

    .btn-primary, .btn-secondary, .btn-danger {
        padding: 8px 16px;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
    }

    .btn-primary {
        background-color: #4a6cf7;
        color: white;
        border: none;
    }

    .btn-primary:disabled {
        background-color: #a0aee8;
        cursor: not-allowed;
    }

    .btn-secondary {
        background-color: #f0f0f0;
        color: #333;
        border: 1px solid #ddd;
    }

    .btn-danger {
        background-color: #f44336;
        color: white;
        border: none;
    }

    .error-message {
        background-color: #ffebee;
        color: #c62828;
        padding: 10px;
        border-radius: 4px;
        margin-bottom: 20px;
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .error-message button {
        background: none;
        border: none;
        color: #c62828;
        cursor: pointer;
        font-weight: bold;
    }

    .loading {
        text-align: center;
        padding: 40px;
        color: #666;
    }

    .empty-state {
        text-align: center;
        padding: 40px;
        color: #666;
        background-color: #f9f9f9;
        border-radius: 8px;
    }

    .note-form {
        background-color: white;
        border-radius: 8px;
        padding: 20px;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
    }

    .form-group {
        margin-bottom: 20px;
    }

    .form-group label {
        display: block;
        margin-bottom: 8px;
        font-weight: 500;
    }

    .form-group input[type="text"],
    .form-group textarea {
        width: 100%;
        padding: 10px;
        border: 1px solid #ddd;
        border-radius: 4px;
        font-size: 1rem;
    }

    .form-options {
        display: flex;
        gap: 20px;
        margin-bottom: 20px;
    }

    .checkbox-group {
        display: flex;
        align-items: center;
    }

    .checkbox-group input {
        margin-right: 8px;
    }

    .tag-input {
        display: flex;
        gap: 8px;
        margin-bottom: 10px;
    }

    .tag-input input {
        flex-grow: 1;
    }

    .tag-input button {
        padding: 8px 12px;
        background-color: #4a6cf7;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
    }

    .tags-container {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
    }

    .tag {
        background-color: #e0e0e0;
        color: #333;
        padding: 4px 8px;
        border-radius: 16px;
        font-size: 0.85rem;
        display: inline-flex;
        align-items: center;
    }

    .tag button {
        background: none;
        border: none;
        color: #666;
        margin-left: 4px;
        cursor: pointer;
        font-size: 1rem;
        line-height: 1;
    }

    .note-metadata {
        margin-top: 20px;
        padding-top: 15px;
        border-top: 1px solid #eee;
        font-size: 0.85rem;
        color: #888;
    }

    .note-metadata p {
        margin: 5px 0;
    }
</style> 