<script lang="ts">
	import { goto } from '$app/navigation';
	import '$lib/styles/global.css';
	import { onMount } from 'svelte';

	let isScrolled = false;

	onMount(() => {
		window.addEventListener('scroll', () => {
			isScrolled = window.scrollY > 20;
		});
	});

	function navigateTo(page: string) {
		goto(page);
	}
</script>

<header class="header" class:scrolled={isScrolled}>
	<div class="header-content">
		<a href="/" class="logo" on:click={() => navigateTo('/')}>
			<img src="/atlantis_logo_transparent.png" alt="Atlantis Logo" class="logo-image" />
		</a>
	</div>
</header>

<style>
	.header {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		height: 80px;
		z-index: 1000;
		transition: all 0.3s ease;
		background: transparent;
	}

	.header.scrolled {
		background: rgba(0, 0, 0, 0.95);
		backdrop-filter: blur(10px);
		box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
		height: 60px;
	}

	.header-content {
		max-width: 1200px;
		margin: 0 auto;
		padding: 0 2rem;
		height: 100%;
		display: flex;
		justify-content: center;
		align-items: center;
	}

	.logo {
		display: flex;
		align-items: center;
	}

	.logo-image {
		height: 80px;
		width: auto;
		display: block;
		transition:
			height 0.3s ease,
			transform 0.3s ease;
		transform: translateY(5px);
	}

	.logo:hover .logo-image {
		transform: translateY(5px) scale(1.1);
	}

	.header.scrolled .logo-image {
		height: 60px;
	}
</style>
