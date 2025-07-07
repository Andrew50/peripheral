<script lang="ts">
  import { chipIdeas } from './chipIdeas';
  import type { ChipIdea } from './chipIdeas';

  const chipsPerRow = 7;

  // Group chip ideas into rows of 7
  const rows: ChipIdea[][] = [];
  for (let i = 0; i < chipIdeas.length; i += chipsPerRow) {
    rows.push(chipIdeas.slice(i, i + chipsPerRow));
  }
</script>

<section class="chip-section">
  <h2 class="chip-title">Ask Anything</h2>
  <div class="chip-rows">
    {#each rows as row, index}
      <div class="chip-row {index % 2 === 1 ? 'reverse' : ''}">
        <div class="chip-track">
          {#each row as chip}
            <button class="chip">
              <span class="chip-icon">{chip.icon}</span>
              <span class="chip-text">{chip.text}</span>
            </button>
          {/each}
          <!-- Duplicate the chips for seamless loop -->
          {#each row as chip}
            <button class="chip">
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
    color: var(--color-dark);
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
    animation: scrollLeft 60s linear infinite;
    width: fit-content;
  }

  /* Reverse rows scroll in opposite direction */
  .chip-row.reverse .chip-track {
    animation: scrollRight 60s linear infinite;
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
    color: var(--color-dark);
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
    .chip-section {
      padding: 3rem 1rem;
    }

    .chip-track {
      animation-duration: 80s;
    }

    .chip {
      padding: 0.5rem 0.8rem;
      font-size: 0.85rem;
    }

    .chip-icon {
      font-size: 1rem;
    }
  }
</style> 