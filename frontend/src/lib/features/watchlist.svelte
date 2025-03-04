<script lang="ts">
	import List from '$lib/utils/modules/list.svelte';
	import type { Writable } from 'svelte/store';
	import { writable, get } from 'svelte/store';
	import type { Instance, Watchlist } from '$lib/core/types';
	import { onMount, tick } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { flagWatchlistId, watchlists, flagWatchlist } from '$lib/core/stores';
	import '$lib/core/global.css';

	let activeList: Writable<Instance[]> = writable([]);
	let newWatchlistName = '';
	let currentWatchlistId: number;
	let previousWatchlistId: number;
	let container: HTMLDivElement;
	let newNameInput: HTMLInputElement;
	let showWatchlistInput = false;
	let confirmingDelete = false;

	function closeNewWatchlistWindow() {
		showWatchlistInput = false;
		newWatchlistName = '';
		currentWatchlistId = previousWatchlistId;
		tick().then(() => {
			selectWatchlist(String(previousWatchlistId));
		});
	}

	onMount(() => {
		selectWatchlist(flagWatchlistId);
	});

	function addInstance() {
		const inst = { ticker: '', timestamp: 0 };
		queryInstanceInput(['ticker'], ['ticker', 'timestamp'], inst).then((i: Instance) => {
			const aList = get(activeList);
			const empty = !Array.isArray(aList);
			if (empty || !aList.find((l: Instance) => l.ticker === i.ticker)) {
				privateRequest<number>('newWatchlistItem', {
					watchlistId: parseInt(currentWatchlistId, 10),
					securityId: i.securityId
				}).then((watchlistItemId: number) => {
					activeList.update((v: Instance[]) => {
						i.watchlistItemId = watchlistItemId;
						if (empty) {
							return [i];
						} else {
							return [...v, i];
						}
					});
				});
			}
			setTimeout(() => {
				addInstance();
			}, 1);
		});
	}

	function newWatchlist() {
		if (newWatchlistName === '') return;

		// Check for duplicate names
		const existingWatchlist = get(watchlists).find(
			(w) => w.watchlistName.toLowerCase() === newWatchlistName.toLowerCase()
		);

		if (existingWatchlist) {
			alert('A watchlist with this name already exists');
			return;
		}

		privateRequest<number>('newWatchlist', { watchlistName: newWatchlistName }).then(
			(newId: number) => {
				watchlists.update((v: Watchlist[]) => {
					const w: Watchlist = {
						watchlistName: newWatchlistName,
						watchlistId: newId
					};
					selectWatchlist(String(newId));
					newWatchlistName = '';
					showWatchlistInput = false;
					if (!Array.isArray(v)) {
						return [w];
					}
					return [w, ...v];
				});
			}
		);
	}

	function deleteItem(item: Instance) {
		if (!item.watchlistItemId) {
			throw new Error('missing id on delete');
		}
		privateRequest<void>('deleteWatchlistItem', { watchlistItemId: item.watchlistItemId }).then(
			() => {
				activeList.update((items) => {
					return items.filter((i) => i.watchlistItemId !== item.watchlistItemId);
				});
			}
		);
	}

	function selectWatchlist(watchlistIdString: string) {
		if (!watchlistIdString) return;

		if (watchlistIdString === 'new') {
			previousWatchlistId = currentWatchlistId;
			showWatchlistInput = true;
			tick().then(() => {
				newNameInput.focus();
			});
			return;
		}

		if (watchlistIdString === 'delete') {
			if (confirmingDelete) {
				deleteWatchlist(currentWatchlistId);
				confirmingDelete = false;
			} else {
				confirmingDelete = true;
				const watchlist = get(watchlists).find((w) => w.watchlistId === currentWatchlistId);
				if (!confirm(`Are you sure you want to delete "${watchlist?.watchlistName}"?`)) {
					selectWatchlist(String(currentWatchlistId));
					return;
				}
				deleteWatchlist(currentWatchlistId);
			}
			return;
		}

		showWatchlistInput = false;
		newWatchlistName = '';
		const watchlistId = parseInt(watchlistIdString);
		if (watchlistId === flagWatchlistId) {
			activeList = flagWatchlist;
		} else {
			activeList = writable<Instance[]>([]);
		}
		currentWatchlistId = watchlistId;
		privateRequest<Instance[]>('getWatchlistItems', { watchlistId: watchlistId }).then(
			(v: Instance[]) => {
				activeList.set(v);
			}
		);
	}

	function deleteWatchlist(id: number) {
		privateRequest<void>('deleteWatchlist', { watchlistId: id }).then(() => {
			watchlists.update((v: Watchlist[]) => {
				return v.filter((v: Watchlist) => v.watchlistId !== id);
			});
			if (id === flagWatchlistId) {
				flagWatchlist.set([]);
			}
		});
	}

	// Helper function to get first letter of watchlist name
	function getWatchlistInitial(name: string): string {
		return name.charAt(0).toUpperCase();
	}

	function handleWatchlistChange(event: Event) {
		const target = event.target as HTMLSelectElement;
		if (target.value !== 'new') {
			previousWatchlistId = parseInt(target.value);
		}
		selectWatchlist(target.value);
	}
</script>

<div tabindex="-1" class="feature-container" bind:this={container}>
	<!-- Controls container first -->
	<div class="controls-container">
		{#if Array.isArray($watchlists)}
			<div class="watchlist-selector">
				<select
					class="default-select"
					id="watchlists"
					bind:value={currentWatchlistId}
					on:change={handleWatchlistChange}
				>
					<optgroup label="My Watchlists">
						{#each $watchlists as watchlist}
							<option value={watchlist.watchlistId}>
								{watchlist.watchlistName}
							</option>
						{/each}
					</optgroup>
					<optgroup label="Actions">
						<option value="new">+ Create New Watchlist</option>
						{#if currentWatchlistId}
							<option value="delete">- Delete Current Watchlist</option>
						{/if}
					</optgroup>
				</select>
				{#if !showWatchlistInput}
					<button class="utility-button" title="Add Symbol" on:click={addInstance}>+</button>
				{/if}
			</div>

			{#if showWatchlistInput}
				<div class="new-watchlist-container">
					<input
						class="input"
						bind:this={newNameInput}
						on:keydown={(event) => {
							if (event.key === 'Enter') {
								newWatchlist();
							} else if (event.key === 'Escape') {
								closeNewWatchlistWindow();
							}
						}}
						bind:value={newWatchlistName}
						placeholder="New Watchlist Name"
					/>
					<div class="new-watchlist-buttons">
						<button class="utility-button" on:click={newWatchlist}>✓</button>
						<button class="utility-button" on:click={closeNewWatchlistWindow}>✕</button>
					</div>
				</div>
			{/if}
		{/if}
	</div>

	<!-- Shortcut buttons between controls and list -->
	<div class="shortcut-container">
		{#if Array.isArray($watchlists)}
			{#each $watchlists as watchlist}
				<button
					class="shortcut-button {currentWatchlistId === watchlist.watchlistId ? 'active' : ''}"
					on:click={() => selectWatchlist(String(watchlist.watchlistId))}
				>
					{getWatchlistInitial(watchlist.watchlistName)}
				</button>
			{/each}
		{/if}
	</div>

	<List
		parentDelete={deleteItem}
		columns={['Ticker', 'Price', 'Chg', 'Chg%', 'Ext']}
		list={activeList}
	/>
</div>

<style>
	.watchlist-selector {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 4px;
		background: var(--ui-bg-secondary);
		border-radius: 6px;
		border: 1px solid var(--ui-border);
	}

	.watchlist-selector select {
		flex: 1;
		min-width: 200px;
	}

	.watchlist-selector .utility-button {
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 18px;
		transition: all 0.2s ease;
	}

	.watchlist-selector .utility-button:hover {
		background: var(--ui-bg-hover);
		transform: translateY(-1px);
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	.new-watchlist-container {
		margin-top: 12px;
		padding: 16px;
		background: var(--ui-bg-secondary);
		border-radius: 8px;
		border: 1px solid var(--ui-border);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
		animation: slideDown 0.2s ease-out;
	}

	@keyframes slideDown {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.new-watchlist-container .input {
		width: 100%;
		margin-bottom: 12px;
		padding: 10px 12px;
		border-radius: 6px;
		border: 1px solid var(--ui-border);
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		font-size: 14px;
		transition: all 0.2s ease;
	}

	.new-watchlist-container .input:focus {
		border-color: var(--accent-color);
		box-shadow: 0 0 0 2px rgba(var(--accent-color-rgb), 0.1);
	}

	.new-watchlist-buttons {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}

	.new-watchlist-buttons .utility-button {
		padding: 8px 16px;
		border-radius: 4px;
		border: 1px solid var(--ui-border);
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		font-size: 14px;
		transition: all 0.2s ease;
		min-width: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.new-watchlist-buttons .utility-button:hover {
		background: var(--ui-bg-hover);
		transform: translateY(-1px);
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	.shortcut-container {
		display: flex;
		gap: 8px;
		padding: 12px 16px;
		flex-wrap: wrap;
		border-bottom: 1px solid var(--ui-border);
		background: var(--ui-bg-secondary);
	}

	.shortcut-button {
		padding: 8px 12px;
		border-radius: 6px;
		border: 1px solid var(--ui-border);
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		font-size: 14px;
		font-weight: 500;
		transition: all 0.2s ease;
		min-width: 36px;
		height: 36px;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.shortcut-button:hover {
		background: var(--ui-bg-hover);
		transform: translateY(-1px);
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	.shortcut-button.active {
		background: var(--accent-color);
		color: var(--text-on-accent);
		border-color: var(--accent-color);
	}

	.feature-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
		height: 100%;
		background: var(--ui-bg-primary);
		border-radius: 8px;
		overflow: hidden;
	}

	.controls-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 16px;
		background: var(--ui-bg-primary);
		border-bottom: 1px solid var(--ui-border);
	}

	:global(.default-select) {
		padding: 8px 12px;
		border-radius: 6px;
		border: 1px solid var(--ui-border);
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		font-size: 14px;
		transition: all 0.2s ease;
	}

	:global(.default-select:hover) {
		border-color: var(--accent-color);
	}

	:global(.default-select:focus) {
		border-color: var(--accent-color);
		box-shadow: 0 0 0 2px rgba(var(--accent-color-rgb), 0.1);
	}

	:global(.default-select option) {
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		padding: 8px;
	}

	:global(.default-select optgroup) {
		font-weight: 600;
		color: var(--text-secondary);
	}
</style>
