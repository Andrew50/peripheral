<!-- screen.svelte -->
<script lang='ts' context='module'>
  export interface Watch extends Instance {
      flagged: boolean
  }
  </script>
<script lang="ts">
  import { onMount,onDestroy } from 'svelte';
  import { writable,get} from 'svelte/store';
  import {queryInstanceRightClick} from '$lib/utils/rightClick.svelte'
  import type {RightClickResult} from "$lib/utils/rightClick.svelte"
  import type { Writable } from 'svelte/store';
  import type {Instance} from '$lib/core/types'
  import {changeChart} from '$lib/features/chart/interface'

  export let list: Writable<Watch[]> = writable([])
  export let columns: Array<string>;
  export let parentDelete = (v:Instance) => {}
  import type {Watch} from '$lib/core/types'


    

  let selectedRowIndex = -1;

    function rowRightClick(event:MouseEvent,watch:Watch){
        event.preventDefault();
        queryInstanceRightClick(event,watch,'list').then((v:RightClickResult)=>{
            if (v === "flag"){
                watch.flagged = ! watch.flagged
                list.update(s=>s)
            }

        })
    }
    function deleteRow(event:MouseEvent,watch:Watch){
        event.stopPropagation()
        list.update((v:Watch[])=>{
            return v.filter(s => s !== watch)
        })
        parentDelete(watch)
    }
    function handleKeydown(event: KeyboardEvent,watch:Watch) {
        console.log(event)
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

  // Move selection down
  function moveDown() {
    if (selectedRowIndex < $list.length - 1) {
      selectedRowIndex++;
      scrollToRow(selectedRowIndex);
    }
  }

  // Move selection up
  function moveUp() {
    if (selectedRowIndex > 0) {
      selectedRowIndex--;
    }else{
        selectedRowIndex = 0
    }
    scrollToRow(selectedRowIndex);
    }


  // Scroll to selected row
  function scrollToRow(index: number) {
    const row = document.getElementById(`row-${index}`);
    if (row) {
      row.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        changeChart(get(list)[selectedRowIndex])
    }
  }

  // Attach keydown event listener on mount
  onMount(() => {
    window.addEventListener('keydown', handleKeydown);
  });

  // Remove event listener on destroy
  onDestroy(() => {
    window.removeEventListener('keydown', handleKeydown);
  });
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
          <tr on:click={()=>{selectedRowIndex = i;changeChart(watch)}}
          id="row-{i}"
          class:selected={i===selectedRowIndex}
            on:contextmenu={(event)=>rowRightClick(event,watch)}
          >
          {#each columns as col}
                <td>{watch[col]}</td>
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

