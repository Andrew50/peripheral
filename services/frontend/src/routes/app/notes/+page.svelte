<script lang="ts">
	import { onMount } from 'svelte';
	import {
		getNotes,
		createNote,
		deleteNote,
		toggleNotePin,
		toggleNoteArchive,
		searchNotes,
		getNote
	} from '$lib/services/notes';
	import type { Note, NoteFilter, SearchResult } from '$lib/core/types';
	import { goto } from '$app/navigation';

	let notes: Note[] = [];
	let searchResults: SearchResult[] = [];
	let loading = true;
	let searching = false;
	let error: string | null = null;

	// Filter state
	let filter: NoteFilter = {
		isArchived: false // Default to showing non-archived notes
	};
	let categories: string[] = [];
	let allTags: string[] = [];

	// Search state
	let searchQuery = '';
	let isSearchMode = false;

	// New note form
	let showNewNoteForm = false;
	let newNote = {
		title: '',
		content: '',
		category: '',
		tags: [] as string[],
		isPinned: false,
		isArchived: false
	};
	let newNoteTag = '';

	onMount(async () => {
		await loadNotes();
	});

	async function loadNotes() {
		loading = true;
		error = null;
		isSearchMode = false;
		try {
			notes = await getNotes(filter);

			// Extract unique categories and tags
			const categorySet = new Set<string>();
			const tagSet = new Set<string>();

			notes.forEach((note) => {
				if (note.category) categorySet.add(note.category);
				note.tags.forEach((tag) => tagSet.add(tag));
			});

			categories = Array.from(categorySet).sort();
			allTags = Array.from(tagSet).sort();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load notes';
			console.error('Error loading notes:', err);
		} finally {
			loading = false;
		}
	}

	async function handleSearch() {
		if (!searchQuery.trim()) {
			await loadNotes();
			return;
		}

		searching = true;
		error = null;
		try {
			searchResults = await searchNotes(searchQuery, filter.isArchived);
			isSearchMode = true;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to search notes';
			console.error('Error searching notes:', err);
		} finally {
			searching = false;
		}
	}

	function clearSearch() {
		searchQuery = '';
		isSearchMode = false;
		loadNotes();
	}

	async function handleCreateNote() {
		if (!newNote.title.trim()) {
			error = 'Title is required';
			return;
		}

		try {
			await createNote(newNote);
			// Reset form
			newNote = {
				title: '',
				content: '',
				category: '',
				tags: [],
				isPinned: false,
				isArchived: false
			};
			showNewNoteForm = false;
			await loadNotes();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create note';
			console.error('Error creating note:', err);
		}
	}

	async function handleDeleteNote(noteId: number) {
		if (!confirm('Are you sure you want to delete this note?')) {
			return;
		}

		try {
			await deleteNote(noteId);
			if (isSearchMode) {
				// Remove the deleted note from search results
				searchResults = searchResults.filter((result) => result.note.noteId !== noteId);
			} else {
				await loadNotes();
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete note';
			console.error('Error deleting note:', err);
		}
	}

	async function handleTogglePin(note: Note) {
		try {
			const updatedNote = await toggleNotePin(note.noteId, !note.isPinned);
			if (isSearchMode) {
				// Update the note in search results
				searchResults = searchResults.map((result) =>
					result.note.noteId === note.noteId ? { ...result, note: updatedNote } : result
				);
			} else {
				await loadNotes();
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to toggle pin status';
			console.error('Error toggling pin status:', err);
		}
	}

	async function handleToggleArchive(note: Note) {
		try {
			await toggleNoteArchive(note.noteId, !note.isArchived);
			if (isSearchMode) {
				// If we're filtering by archive status, remove the note from results
				if (filter.isArchived !== undefined) {
					searchResults = searchResults.filter((result) => result.note.noteId !== note.noteId);
				} else {
					// Otherwise, update the note in search results
					const updatedNote = await getNote(note.noteId);
					searchResults = searchResults.map((result) =>
						result.note.noteId === note.noteId ? { ...result, note: updatedNote } : result
					);
				}
			} else {
				await loadNotes();
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to toggle archive status';
			console.error('Error toggling archive status:', err);
		}
	}

	function handleEditNote(noteId: number) {
		goto(`/app/notes/${noteId}`);
	}

	function addTag() {
		if (newNoteTag && !newNote.tags.includes(newNoteTag)) {
			newNote.tags = [...newNote.tags, newNoteTag];
			newNoteTag = '';
		}
	}

	function removeTag(tag: string) {
		newNote.tags = newNote.tags.filter((t) => t !== tag);
	}

	function resetFilter() {
		filter = {
			isArchived: false
		};
		if (isSearchMode && searchQuery) {
			handleSearch();
		} else {
			loadNotes();
		}
	}

	function toggleShowArchived() {
		filter.isArchived = !filter.isArchived;
		if (isSearchMode && searchQuery) {
			handleSearch();
		} else {
			loadNotes();
		}
	}

	function filterByCategory(category: string) {
		filter.category = filter.category === category ? undefined : category;
		if (isSearchMode) {
			isSearchMode = false;
		}
		loadNotes();
	}

	function filterByTag(tag: string) {
		if (!filter.tags) {
			filter.tags = [tag];
		} else if (filter.tags.includes(tag)) {
			filter.tags = filter.tags.filter((t) => t !== tag);
			if (filter.tags.length === 0) {
				filter.tags = undefined;
			}
		} else {
			filter.tags = [...filter.tags, tag];
		}
		if (isSearchMode) {
			isSearchMode = false;
		}
		loadNotes();
	}

	// Format date to a readable format
	function formatDate(dateString: string): string {
		const date = new Date(dateString);
		return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
	}
</script>

<div class="notes-container">
	<div class="notes-header">
		<h1>Notes</h1>
		<div class="search-bar">
			<input
				type="text"
				placeholder="Search notes..."
				bind:value={searchQuery}
				on:keydown={(e) => e.key === 'Enter' && handleSearch()}
			/>
			{#if searchQuery}
				<button class="clear-search" on:click={clearSearch}>×</button>
			{/if}
			<button class="search-button" on:click={handleSearch} disabled={searching}>
				{searching ? 'Searching...' : 'Search'}
			</button>
		</div>
		<div class="notes-actions">
			<button class="btn-primary" on:click={() => (showNewNoteForm = !showNewNoteForm)}>
				{showNewNoteForm ? 'Cancel' : 'New Note'}
			</button>
			<button class="btn-secondary" on:click={toggleShowArchived}>
				{filter.isArchived ? 'Hide Archived' : 'Show Archived'}
			</button>
			<button class="btn-secondary" on:click={resetFilter}>Reset Filters</button>
		</div>
	</div>

	{#if error}
		<div class="error-message">
			<p>{error}</p>
			<button on:click={() => (error = null)}>Dismiss</button>
		</div>
	{/if}

	{#if showNewNoteForm}
		<div class="new-note-form">
			<h2>Create New Note</h2>
			<div class="form-group">
				<label for="title">Title *</label>
				<input type="text" id="title" bind:value={newNote.title} required />
			</div>
			<div class="form-group">
				<label for="content">Content</label>
				<textarea id="content" bind:value={newNote.content} rows="5"></textarea>
			</div>
			<div class="form-group">
				<label for="category">Category</label>
				<input type="text" id="category" bind:value={newNote.category} list="categories" />
				<datalist id="categories">
					{#each categories as category}
						<option value={category} />
					{/each}
				</datalist>
			</div>
			<div class="form-group">
				<label>Tags</label>
				<div class="tag-input">
					<input type="text" bind:value={newNoteTag} placeholder="Add a tag" />
					<button type="button" on:click={addTag}>Add</button>
				</div>
				<div class="tags-container">
					{#each newNote.tags as tag}
						<span class="tag">
							{tag}
							<button type="button" on:click={() => removeTag(tag)}>×</button>
						</span>
					{/each}
				</div>
			</div>
			<div class="form-group checkbox-group">
				<label>
					<input type="checkbox" bind:checked={newNote.isPinned} />
					Pin this note
				</label>
			</div>
			<div class="form-actions">
				<button type="button" class="btn-secondary" on:click={() => (showNewNoteForm = false)}
					>Cancel</button
				>
				<button type="button" class="btn-primary" on:click={handleCreateNote}>Create Note</button>
			</div>
		</div>
	{/if}

	{#if !isSearchMode}
		<div class="filter-section">
			<div class="filter-group">
				<h3>Categories</h3>
				<div class="filter-options">
					{#each categories as category}
						<button
							class="filter-btn {filter.category === category ? 'active' : ''}"
							on:click={() => filterByCategory(category)}
						>
							{category}
						</button>
					{/each}
				</div>
			</div>

			<div class="filter-group">
				<h3>Tags</h3>
				<div class="filter-options">
					{#each allTags as tag}
						<button
							class="filter-btn {filter.tags?.includes(tag) ? 'active' : ''}"
							on:click={() => filterByTag(tag)}
						>
							{tag}
						</button>
					{/each}
				</div>
			</div>
		</div>
	{/if}

	{#if isSearchMode}
		<div class="search-results-header">
			<h2>Search Results for "{searchQuery}"</h2>
			<button class="btn-secondary" on:click={clearSearch}>Clear Search</button>
		</div>

		{#if searching}
			<div class="loading">Searching notes...</div>
		{:else if searchResults.length === 0}
			<div class="empty-state">
				<p>No notes found matching your search query.</p>
			</div>
		{:else}
			<div class="search-results">
				{#each searchResults as result (result.note.noteId)}
					<div class="search-result-card {result.note.isPinned ? 'pinned' : ''}">
						<div class="note-header">
							<h3>
								{@html result.titleHighlight || result.note.title}
							</h3>
							<div class="note-actions">
								<button
									class="icon-btn"
									on:click={() => handleTogglePin(result.note)}
									title={result.note.isPinned ? 'Unpin' : 'Pin'}
								>
									{result.note.isPinned ? '📌' : '📍'}
								</button>
								<button
									class="icon-btn"
									on:click={() => handleToggleArchive(result.note)}
									title={result.note.isArchived ? 'Unarchive' : 'Archive'}
								>
									{result.note.isArchived ? '🔄' : '📁'}
								</button>
								<button
									class="icon-btn"
									on:click={() => handleEditNote(result.note.noteId)}
									title="Edit"
								>
									✏️
								</button>
								<button
									class="icon-btn"
									on:click={() => handleDeleteNote(result.note.noteId)}
									title="Delete"
								>
									🗑️
								</button>
							</div>
						</div>

						<div class="note-content">
							{#if result.contentHighlight}
								<p class="search-highlight">{@html result.contentHighlight}</p>
							{:else if result.note.content}
								<p>
									{result.note.content.length > 150
										? result.note.content.substring(0, 150) + '...'
										: result.note.content}
								</p>
							{:else}
								<p class="empty-content">No content</p>
							{/if}
						</div>

						{#if result.note.category}
							<div class="note-category">
								<span>Category: {result.note.category}</span>
							</div>
						{/if}

						{#if result.note.tags.length > 0}
							<div class="note-tags">
								{#each result.note.tags as tag}
									<span class="tag">{tag}</span>
								{/each}
							</div>
						{/if}

						<div class="note-footer">
							<span class="note-date">Updated: {formatDate(result.note.updatedAt)}</span>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	{:else if loading}
		<div class="loading">Loading notes...</div>
	{:else if notes.length === 0}
		<div class="empty-state">
			<p>No notes found. {filter.isArchived ? 'No archived notes.' : 'Create your first note!'}</p>
		</div>
	{:else}
		<div class="notes-grid">
			{#each notes as note (note.noteId)}
				<div class="note-card {note.isPinned ? 'pinned' : ''}">
					<div class="note-header">
						<h3>{note.title}</h3>
						<div class="note-actions">
							<button
								class="icon-btn"
								on:click={() => handleTogglePin(note)}
								title={note.isPinned ? 'Unpin' : 'Pin'}
							>
								{note.isPinned ? '📌' : '📍'}
							</button>
							<button
								class="icon-btn"
								on:click={() => handleToggleArchive(note)}
								title={note.isArchived ? 'Unarchive' : 'Archive'}
							>
								{note.isArchived ? '🔄' : '📁'}
							</button>
							<button class="icon-btn" on:click={() => handleEditNote(note.noteId)} title="Edit">
								✏️
							</button>
							<button
								class="icon-btn"
								on:click={() => handleDeleteNote(note.noteId)}
								title="Delete"
							>
								🗑️
							</button>
						</div>
					</div>

					<div class="note-content">
						{#if note.content}
							<p>
								{note.content.length > 150 ? note.content.substring(0, 150) + '...' : note.content}
							</p>
						{:else}
							<p class="empty-content">No content</p>
						{/if}
					</div>

					{#if note.category}
						<div class="note-category">
							<span>Category: {note.category}</span>
						</div>
					{/if}

					{#if note.tags.length > 0}
						<div class="note-tags">
							{#each note.tags as tag}
								<span class="tag">{tag}</span>
							{/each}
						</div>
					{/if}

					<div class="note-footer">
						<span class="note-date">Updated: {formatDate(note.updatedAt)}</span>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.notes-container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 20px;
	}

	.notes-header {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 20px;
		gap: 15px;
	}

	.search-bar {
		flex: 1;
		display: flex;
		position: relative;
		max-width: 500px;
	}

	.search-bar input {
		flex: 1;
		padding: 8px 35px 8px 12px;
		border: 1px solid #ddd;
		border-radius: 4px 0 0 4px;
		font-size: 1rem;
	}

	.clear-search {
		position: absolute;
		right: 80px;
		top: 50%;
		transform: translateY(-50%);
		background: none;
		border: none;
		font-size: 1.2rem;
		color: #999;
		cursor: pointer;
	}

	.search-button {
		padding: 8px 16px;
		background-color: #4a6cf7;
		color: white;
		border: none;
		border-radius: 0 4px 4px 0;
		cursor: pointer;
	}

	.search-button:disabled {
		background-color: #a0aee8;
		cursor: not-allowed;
	}

	.notes-actions {
		display: flex;
		gap: 10px;
	}

	.btn-primary,
	.btn-secondary {
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

	.btn-secondary {
		background-color: #f0f0f0;
		color: #333;
		border: 1px solid #ddd;
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

	.new-note-form {
		background-color: #f9f9f9;
		padding: 20px;
		border-radius: 8px;
		margin-bottom: 20px;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	.form-group {
		margin-bottom: 15px;
	}

	.form-group label {
		display: block;
		margin-bottom: 5px;
		font-weight: 500;
	}

	.form-group input[type='text'],
	.form-group textarea {
		width: 100%;
		padding: 8px;
		border: 1px solid #ddd;
		border-radius: 4px;
	}

	.checkbox-group {
		display: flex;
		align-items: center;
	}

	.checkbox-group input {
		margin-right: 8px;
	}

	.form-actions {
		display: flex;
		justify-content: flex-end;
		gap: 10px;
		margin-top: 15px;
	}

	.tag-input {
		display: flex;
		gap: 8px;
		margin-bottom: 8px;
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

	.filter-section {
		margin-bottom: 20px;
	}

	.filter-group {
		margin-bottom: 15px;
	}

	.filter-group h3 {
		margin-bottom: 8px;
		font-size: 1rem;
	}

	.filter-options {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}

	.filter-btn {
		background-color: #f0f0f0;
		border: 1px solid #ddd;
		border-radius: 16px;
		padding: 4px 12px;
		font-size: 0.85rem;
		cursor: pointer;
	}

	.filter-btn.active {
		background-color: #4a6cf7;
		color: white;
		border-color: #4a6cf7;
	}

	.loading {
		text-align: center;
		padding: 20px;
		color: #666;
	}

	.empty-state {
		text-align: center;
		padding: 40px;
		color: #666;
		background-color: #f9f9f9;
		border-radius: 8px;
	}

	.notes-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
		gap: 20px;
	}

	.note-card,
	.search-result-card {
		background-color: white;
		border-radius: 8px;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
		padding: 16px;
		display: flex;
		flex-direction: column;
		transition:
			transform 0.2s,
			box-shadow 0.2s;
	}

	.note-card:hover,
	.search-result-card:hover {
		transform: translateY(-2px);
		box-shadow: 0 4px 8px rgba(0, 0, 0, 0.15);
	}

	.note-card.pinned,
	.search-result-card.pinned {
		border-top: 3px solid #4a6cf7;
	}

	.note-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: 10px;
	}

	.note-header h3 {
		margin: 0;
		font-size: 1.1rem;
		word-break: break-word;
	}

	.note-actions {
		display: flex;
		gap: 5px;
	}

	.icon-btn {
		background: none;
		border: none;
		cursor: pointer;
		font-size: 1rem;
		padding: 2px;
	}

	.note-content {
		flex-grow: 1;
		margin-bottom: 10px;
	}

	.note-content p {
		margin: 0;
		color: #555;
		word-break: break-word;
	}

	.empty-content {
		color: #999;
		font-style: italic;
	}

	.note-category {
		margin-bottom: 8px;
	}

	.note-category span {
		font-size: 0.85rem;
		color: #666;
	}

	.note-tags {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		margin-bottom: 10px;
	}

	.note-footer {
		font-size: 0.8rem;
		color: #888;
	}

	.search-results-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 20px;
	}

	.search-results-header h2 {
		margin: 0;
		font-size: 1.2rem;
	}

	.search-results {
		display: flex;
		flex-direction: column;
		gap: 15px;
	}

	.search-highlight mark {
		background-color: #ffffa0;
		padding: 0 2px;
		border-radius: 2px;
	}
</style>
