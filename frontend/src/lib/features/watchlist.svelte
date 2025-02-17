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
		} else {
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
			<div class="watchlist-container">
				<select id="watchlists" bind:value={currentWatchlistId} on:change={handleWatchlistChange}>
					{#each $watchlists as watchlist}
						<option value={watchlist.watchlistId}>
							{watchlist.watchlistName}
						</option>
					{/each}
					<hr />
					<option value="new">Create New</option>
				</select>
			</div>
			<button
				class="square-btn"
				on:click={(e) => {
					e.stopPropagation();
					deleteWatchlist(currentWatchlistId);
				}}>x</button
			>
			<button class="square-btn" on:click={addInstance}>+</button>
			{#if showWatchlistInput}
				<input
					class="input"
					bind:this={newNameInput}
					on:keydown={(event) => {
						if (event.key == 'Enter') {
							newWatchlist();
						}
					}}
					bind:value={newWatchlistName}
					placeholder="New Watchlist Name"
				/>
			{/if}
		{/if}
	</div>

	<!-- Shortcut buttons between controls and list -->
	<div class="shortcut-container">
		{#if Array.isArray($watchlists)}
			{#each $watchlists as watchlist}
				<button
					class="shortcut-btn {currentWatchlistId === watchlist.watchlistId ? 'active' : ''}"
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
	.shortcut-container {
		display: flex;
		gap: 8px;
		padding: 8px;
		flex-wrap: wrap;
	}

	.shortcut-btn {
		width: 32px;
		height: 32px;
		border-radius: 50%;
		background: #1e222d;
		border: 1px solid #363a45;
		color: #fff;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		font-size: 14px;
		transition: all 0.2s ease;
	}

	.shortcut-btn:hover {
		background: #2a2e39;
		border-color: #4a4e58;
	}

	.shortcut-btn.active {
		background: #2962ff;
		border-color: #2962ff;
	}

	/* Ensure existing styles remain */
	.feature-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
</style>
