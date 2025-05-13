<script lang="ts">
	import { goto } from '$app/navigation';
	import '$lib/styles/global.css';
	import { onMount } from 'svelte';

	let isScrolled = false;
	let isMobileMenuOpen = false;

	onMount(() => {
		window.addEventListener('scroll', () => {
			isScrolled = window.scrollY > 20;
		});
	});

	function navigateTo(page: string) {
		goto(page);
		isMobileMenuOpen = false;
	}

	function toggleMobileMenu() {
		isMobileMenuOpen = !isMobileMenuOpen;
	}
</script>

<header class="header" class:scrolled={isScrolled}>
	<div class="header-content">
		<div class="left">
			<a href="/" class="logo" on:click={() => navigateTo('/')}>
				<img src="/atlantis_logo_transparent.png" alt="Atlantis Logo" class="logo-image" />
			</a>
		</div>

		<button class="mobile-menu-button" on:click={toggleMobileMenu}>
			<span class="hamburger"></span>
		</button>

		<nav class="right" class:active={isMobileMenuOpen}>
			<!--<a href="/features">Features</a>
			<a href="/pricing">Pricing</a>-->
			<a href="/login" on:click={() => navigateTo('/login')}>Sign In</a>
			<a href="/signup" class="cta-button" on:click={() => navigateTo('/signup')}>Get Started</a>
		</nav>
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
		justify-content: space-between;
		align-items: center;
	}

	.left {
		display: flex;
		align-items: center;
	}

	.logo-image {
		height: 80px;
		width: auto;
		display: block;
		transition: height 0.3s ease, transform 0.3s ease;
		transform: translateY(5px);
	}

	.logo:hover .logo-image {
		transform: translateY(5px) scale(1.1);
	}

	.header.scrolled .logo-image {
		height: 60px;
	}

	.right {
		display: flex;
		align-items: center;
		gap: 2rem;
	}

	.right a {
		color: white;
		text-decoration: none;
		font-weight: 500;
		transition: color 0.3s ease;
	}

	.right a:hover {
		color: #3b82f6;
	}

	.cta-button {
		background: #3b82f6;
		padding: 0.5rem 1.5rem;
		border-radius: 6px;
		color: white !important;
		transition: all 0.3s ease !important;
	}

	.cta-button:hover {
		background: #2563eb;
		transform: translateY(-2px);
	}

	.mobile-menu-button {
		display: none;
		background: none;
		border: none;
		cursor: pointer;
		padding: 0.5rem;
	}

	.hamburger {
		display: block;
		width: 24px;
		height: 2px;
		background: white;
		position: relative;
		transition: all 0.3s ease;
	}

	.hamburger::before,
	.hamburger::after {
		content: '';
		position: absolute;
		width: 24px;
		height: 2px;
		background: white;
		transition: all 0.3s ease;
	}

	.hamburger::before {
		top: -6px;
	}

	.hamburger::after {
		bottom: -6px;
	}

	@media (max-width: 768px) {
		.mobile-menu-button {
			display: block;
		}

		.right {
			position: absolute;
			top: 100%;
			left: 0;
			right: 0;
			background: rgba(0, 0, 0, 0.95);
			padding: 1rem;
			flex-direction: column;
			align-items: center;
			gap: 1rem;
			transform: translateY(-100%);
			opacity: 0;
			pointer-events: none;
			transition: all 0.3s ease;
		}

		.right.active {
			transform: translateY(0);
			opacity: 1;
			pointer-events: all;
		}
	}
</style>
