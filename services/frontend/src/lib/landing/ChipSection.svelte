<script lang="ts">
	import { goto } from '$app/navigation';
  import { chipIdeas } from './chipIdeas';
  import type { ChipIdea } from './chipIdeas';
  import { onMount } from 'svelte';
  import { browser } from '$app/environment';
  
  let isMobile = false;
  
  onMount(() => {
    if (browser) {
      // Check if screen is mobile size
      const checkMobile = () => {
        isMobile = window.innerWidth <= 768;
      };
      checkMobile();
      window.addEventListener('resize', checkMobile);
      
      return () => window.removeEventListener('resize', checkMobile);
    }
  });

  // Reactive statement to create rows based on screen size
  $: {
    // Clear existing rows
    rows.length = 0;
    
    if (isMobile) {
      // Mobile: 4 chips per row, double the rows (8 rows total)
      const chipsPerRow = 4;
      const targetRows = 8;
      const chipsNeeded = targetRows * chipsPerRow; // 32 chips needed
      
      // Create extended chip array by repeating chips if needed
      const extendedChips = [];
      for (let i = 0; i < chipsNeeded; i++) {
        extendedChips.push(chipIdeas[i % chipIdeas.length]);
      }
      
      // Create rows
      for (let i = 0; i < extendedChips.length; i += chipsPerRow) {
        rows.push(extendedChips.slice(i, i + chipsPerRow));
      }
    } else {
      // Desktop: 7 chips per row (original behavior)
      const chipsPerRow = 7;
      for (let i = 0; i < chipIdeas.length; i += chipsPerRow) {
        rows.push(chipIdeas.slice(i, i + chipsPerRow));
      }
    }
  }
  
  let rows: ChipIdea[][] = [];
</script>

<section class="chip-section">
  <h2 class="chip-title">Ask Anything</h2>
  <div class="chip-rows">
    {#each rows as row, index}
      <div class="chip-row {index % 2 === 1 ? 'reverse' : ''}">
        <div class="chip-track">
          {#each row as chip}
            <button class="chip" on:click={() => {
              goto('/signup');
            }}>
              <span class="chip-icon">{chip.icon}</span>
              <span class="chip-text">{chip.text}</span>
            </button>
          {/each}
          <!-- Duplicate the chips for seamless loop -->
          {#each row as chip}
            <button class="chip" on:click={() => {
              goto('/signup');
            }}>
              <span class="chip-icon">{chip.icon}</span>
              <span class="chip-text">{chip.text}</span>
            </button>
          {/each}
        </div>
      </div>
    {/each}
  </div>
</section>

<style>
  .chip-section {
    width: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2rem;
  }

  .chip-title {
    font-size: clamp(2rem, 4vw, 3rem);
    font-weight: 800;
    margin: 0;
    color: #ffffff;
    position: relative;
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  }

  .chip-rows {
    display: flex;
    flex-direction: column;
    gap: 1.2rem;
    width: 100%;
    overflow: hidden;
  }

  .chip-row {
    width: 100%;
    overflow: hidden;
    white-space: nowrap;
  }

  .chip-track {
    display: flex;
    gap: 0.75rem;
    animation: scrollLeft 70s linear infinite;
    width: fit-content;
  }

  /* Reverse rows scroll in opposite direction */
  .chip-row.reverse .chip-track {
    animation: scrollRight 70s linear infinite;
  }

  /* Pause animation on hover */
  .chip-row:hover .chip-track {
    animation-play-state: paused;
  }

  .chip {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.4rem 0.7rem;
    background: white;
    border: 1px solid rgba(11, 46, 51, 0.15);
    border-radius: 9999px;
    font-size: 0.95rem;
    color: black;
    cursor: pointer;
    transition: transform 0.15s ease, box-shadow 0.15s ease;
    white-space: nowrap;
    flex-shrink: 0;
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  }

  .chip:hover {
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  }

  .chip-icon {
    font-size: 1.1rem;
  }

  .chip-text {
    line-height: 1.3;
    color: black;
  }

  /* Animation keyframes */
  @keyframes scrollLeft {
    0% {
      transform: translateX(0);
    }
    100% {
      transform: translateX(-50%);
    }
  }

  @keyframes scrollRight {
    0% {
      transform: translateX(-50%);
    }
    100% {
      transform: translateX(0);
    }
  }

  /* Responsive: slower animation and smaller chips on mobile */
  @media (max-width: 768px) {
    .chip-rows {
      gap: 0.8rem; /* Reduced gap for more rows */
    }

    .chip-track {
      animation-duration: 70s;
    }

    .chip {
      padding: 0.35rem 0.6rem;
      font-size: 0.75rem;
    }

    .chip-icon {
      font-size: 0.9rem;
    }
  }
</style> 