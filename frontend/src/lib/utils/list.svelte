<!-- screen.svelte -->
<script lang="ts">
  import { onMount,onDestroy } from 'svelte';
  import { writable,get} from 'svelte/store';
  import {queryInstanceRightClick} from '$lib/utils/rightClick.svelte'
  import type { Writable } from 'svelte/store';
  import type {Instance} from '$lib/core/types'
  import StreamCell from '$lib/utils/streamCell.svelte'
  import {changeChart} from '$lib/features/chart/interface'
  import {flagWatchlist} from '$lib/core/stores'
    import {flagSecurity} from '$lib/utils/flag'
  export let list: Writable<Instance[]> = writable([])
  export let columns: Array<string>;
  export let parentDelete = (v:Instance) => {}

    function isFlagged(instance:Instance, flagWatch: Instance[]){
      return flagWatch.some(item => item.ticker === instance.ticker);
  }

  let selectedRowIndex = -1;

    function rowRightClick(event:MouseEvent,watch:Instance){
        event.preventDefault();
        queryInstanceRightClick(event,watch,'list')
    }
    function deleteRow(event:MouseEvent,watch:Instance){
        event.stopPropagation()
        list.update((v:Instance[])=>{
            return v.filter(s => s !== watch)
        })
        parentDelete(watch)
    }
    function handleKeydown(event: KeyboardEvent,watch:Instance) {
    if (event.key === 'ArrowUp' || (event.key === ' ' && event.shiftKey)) {
      event.preventDefault();
      moveUp();
    }else if (event.key === 'ArrowDown' || event.key === ' ') {
      event.preventDefault();
      moveDown();
    }else{
        return
    }
  }
  function moveDown() {
    if (selectedRowIndex < $list.length - 1) {
      selectedRowIndex++;
      scrollToRow(selectedRowIndex);
    }
  }
  function moveUp() {
    if (selectedRowIndex > 0) {
      selectedRowIndex--;
    }else{
        selectedRowIndex = 0
    }
    scrollToRow(selectedRowIndex);
    }

  function scrollToRow(index: number) {
    const row = document.getElementById(`row-${index}`);
    if (row) {
      row.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        changeChart(get(list)[selectedRowIndex])
    }
  }
  onMount(() => {
    window.addEventListener('keydown', handleKeydown);
  });
  onDestroy(() => {
    window.removeEventListener('keydown', handleKeydown);
  });
  function clickHandler(event:MouseEvent,instance:Instance,index:number){
        event.preventDefault()
      if (event.button === 0) {
        selectedRowIndex = index;
        changeChart(instance)
      }else if (event.button === 1){
          flagSecurity(instance)
      }else if (event.button === 2){
        rowRightClick(event,instance)
      }
  }
</script>


  {#if Array.isArray($list) && $list.length > 0}
<div class="table-container">
  <table>
    <thead>
      <tr>
      {#each columns as col}
      <th>{col}</th>
      <th></th>
      {/each}
      </tr>
    </thead>
    <tbody>
        {#each $list as watch, i}
          <tr on:mousedown={(event)=>clickHandler(event,watch,i)}
          id="row-{i}"
          class:selected={i===selectedRowIndex}
          >
          <td>
            {#if isFlagged(watch,$flagWatchlist)}
              <span class="flag-icon">⚑</span> <!-- Example flag icon -->
            {/if}
          </td>
          {#each columns as col}
            {#if col === "change"}
                <StreamCell ticker={watch.ticker}/>
            {:else}
                <td>{watch[col]}</td>
            {/if}

          {/each}
          <td class="delete-cell">
                <button class="delete-btn" on:click={(event) => {deleteRow(event,watch)}}> ✕ </button>
              </td>
          </tr>
        {/each}
    </tbody>
  </table>
</div>
  {/if}


<style>
  @import "$lib/core/colors.css";
  .flag-icon {
    color: var(--c3);
    font-size: 16px;
    margin-right: 5px;
  }
  tr.selected {
    background-color: var(--c3);
    color: var(--f1);
  }

  .table-container {
    border: 1px solid var(--c4);
    border-radius: 4px;
    margin-top: 10px;
    width: 100%;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-family: Arial, sans-serif;
  }

  th, td {
    padding: 10px;
    text-align: left;
  }

  th {
    background-color: var(--c1);
    color: var(--f1);
  }
  .delete-btn {
    background-color: var(--c3);
    color: var(--f1);
    border: none;
    padding: 5px;
    font-size: 12px;
    cursor: pointer;
    border-radius: 3px;
  }
  .delete-cell {
    text-align: right;
    padding-right: 10px;
  }

  .delete-btn:hover {
    background-color: var(--c3-hover);
  }

  tr {
    border-bottom: 1px solid var(--c4);
  }

  tr:hover {
    background-color: var(--c2);
    cursor: pointer;
  }

</style>

