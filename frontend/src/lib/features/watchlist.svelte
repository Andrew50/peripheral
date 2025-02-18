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
	let container: HTMLDivElement;
	let newNameInput: HTMLInputElement;
	let showWatchlistInput = false;
	let confirmingDelete = false;

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
					watchlistId: currentWatchlistId,
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
								showWatchlistInput = false;
								selectWatchlist(String(currentWatchlistId));
							}
						}}
						bind:value={newWatchlistName}
						placeholder="New Watchlist Name"
					/>
					<div class="new-watchlist-buttons">
						<button class="utility-button" on:click={newWatchlist}>✓</button>
						<button
							class="utility-button"
							on:click={() => {
								showWatchlistInput = false;
								selectWatchlist(String(currentWatchlistId));
							}}>✕</button
						>
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
		columns={['ticker', 'price', 'change', 'change %', 'change % extended']}
		list={activeList}
	/>
</div>

<style>
	.watchlist-selector {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.new-watchlist-container {
		margin-top: 8px;
		padding: 8px;
		background: var(--ui-bg-secondary);
		border-radius: 4px;
		border: 1px solid var(--ui-border);
	}

	.new-watchlist-container .input {
		width: 100%;
		margin-bottom: 8px;
	}

	.new-watchlist-buttons {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}

	.shortcut-container {
		display: flex;
		gap: 8px;
		padding: 8px 8px 8px 16px;
		flex-wrap: wrap;
	}

	/* Ensure existing styles remain */
	.feature-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	/* Update existing style */
	.controls-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 8px 8px 8px 16px;
	}
</style>
