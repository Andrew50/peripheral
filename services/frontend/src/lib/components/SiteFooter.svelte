<script lang="ts">
	import { goto } from '$app/navigation';

	function navigateToLogin() {
		goto('/login');
	}

	function navigateToSignup() {
		goto('/signup');
	}

	function navigateToPricing() {
		goto('/pricing');
	}

	function navigateToApp() {
		goto('/app');
	}
</script>

<!-- Footer -->
<!-- Big Centered Tagline Section -->
	<section class="tagline-section">
	<div class="tagline-inner">
		<p class="tagline-pretext">JUMP INTO</p>
		<h2 class="tagline-text">The Final Trading Terminal.</h2>
		<button class="tagline" on:click={() => goto('/signup')}>Get Started</button>
	</div>
</section>
<footer class="landing-footer">
	<div class="footer-content">
		<div class="footer-section footer-left"> 
			<div class="footer-section">
				<h4 class="footer-title">Platform</h4>
				<ul class="footer-links">
					<li><button on:click={navigateToPricing}>Pricing</button></li>
					<li><button on:click={navigateToApp}>Dashboard</button></li>
				</ul>
			</div>
			<div class="footer-section">
				<h4 class="footer-title">Account</h4>
				<ul class="footer-links">
					<li><button on:click={navigateToSignup}>Sign Up</button></li>
					<li><a href="mailto:info@peripheral.io" class="footer-sales-link">Contact Sales</a></li>
					<li><button on:click={navigateToLogin}>Login</button></li>
				</ul>
			</div>
			<div class="footer-section">
				<h4 class="footer-title">Connect</h4>
				<div class="footer-social-row">
					<a href="https://twitter.com/peripheralio" target="_blank" rel="noopener noreferrer" class="footer-social-icon" aria-label="X (Twitter)">
						<img src="/x-logo-white.png" alt="X (Twitter)" style="width: 18px; height: 18px; object-fit: contain; display: block;" />
					</a>
					<a href="https://discord.gg/HpQ8ZbvDkV" target="_blank" rel="noopener noreferrer" class="footer-social-icon" aria-label="Discord">
						<img src="/Discord-Symbol-White.png" alt="Discord" style="width: 18px; height: 18px; object-fit: contain; display: block;" />
					</a>
					<a href="https://www.linkedin.com/company/peripheralio" target="_blank" rel="noopener noreferrer" class="footer-social-icon" aria-label="LinkedIn">
						<img src="/InBug-White.png" alt="LinkedIn" style="width: 18px; height: 18px; object-fit: contain; display: block;" />
					</a>
				</div>
			</div>
		</div>
	</div>
	<div class="footer-bottom">
		<p>Charts powered by <a href="https://tradingview.com/" target="_blank" rel="noopener noreferrer">TradingView</a></p>
		<p>2025 Atlantis Labs, Inc.</p>
	</div>
	<div class="footer-brand">Peripheral</div>
</footer>

<style>
	.tagline-section {
		width: 100vw;
		padding: 4rem 0 8rem 0;
		display: flex;
		justify-content: center;
		align-items: center;
		background: none;
	}
	.tagline-inner {
		display: flex;
		flex-direction: column;
		align-items: center;
	}
	.tagline-text {
		font-size: clamp(2.5rem, 7vw, 5rem);
		font-weight: 900;
		color: #ffffff;
		text-align: center;
		margin: 0;
		letter-spacing: -0.04em;
		line-height: 1.1;
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
	}
	.tagline {
		margin-top: 3rem;
		font-size: 1.2rem;
		padding: 1.1rem 2.5rem;
		background: rgb(0, 0, 0);
		color: #f5f9ff;
		border: 1px solid transparent;
		border-radius: 999px;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.1s ease;
		box-shadow: none;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		text-decoration: none;
		white-space: nowrap;
	}
	.tagline:hover {
		transform: translateY(-3px);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
	}

	.tagline-pretext {
		font-size: 1.2rem;
		font-weight: 500;
		color: var(--color-primary);
		margin: 0 0 0.5rem 0;
		text-align: center;
	}
	.landing-footer {
		position: relative;
		z-index: 10;
		background: var(--color-dark);
		border-top: 1px solid rgba(255, 255, 255, 0.1);
		width: 100vw;
		left: 50%;
		transform: translateX(-50%);
		padding: 3rem 0 2rem 0;
		flex-shrink: 0;
		margin-top: auto;
		margin-bottom: 0;
	}

	.footer-content {
		display: flex;
		flex-direction: row;
		justify-content: flex-start;
		gap: 2rem;
		padding: 0 2rem;
	}

	.footer-left {
		display: flex;
		flex-direction: row;
		gap: 2rem;
	}

	.footer-section {
		margin-bottom: 0;
	}

	.footer-title {
		color: var(--color-light);
		font-size: 1.1rem;
		font-weight: 600;
		margin: 0 0 1rem 0;
	}

	.footer-links {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	.footer-links li {
		margin-bottom: 0.5rem;
	}

	.footer-links button {
		background: none;
		border: none;
		color: #ffffff;
		cursor: pointer;
		font-size: 0.9rem;
		padding: 0;
		text-align: left;
		transition: color 0.3s ease;
	}

	.footer-links button:hover {
		color: #f5f9ff;
	}

	.footer-links a {
		color: #9ca3af;
		text-decoration: none;
		font-size: 0.9rem;
		transition: color 0.3s ease;
	}

	.footer-links a:hover {
		color: #f5f9ff;
	}

	.footer-bottom {
		text-align: left;
		color: var(--color-primary);
		font-size: 0.9rem;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		row-gap: 0.25rem;
		padding: 0 2rem;
	}

	.footer-bottom a {
		color: inherit;
		text-decoration: none;
	}

	.footer-brand {
		width: 100vw;
		position: relative;
		left: 50%;
		transform: translateX(-50%);
		text-align: center;
		font-size: clamp(4rem, 14vw, 12rem);
		font-weight: 900;
		color: #f5f9ff;
		padding: 0.5rem 0 0 0;
		margin-bottom: 0;
		line-height: 1;
		letter-spacing: -0.06em;
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		z-index: 1;
		pointer-events: none;
		user-select: none;
	}

	.footer-social-row {
		display: flex;
		gap: 0.3rem;
		align-items: center;
		justify-content: center;
		margin-top: 0.5rem;
	}

	.footer-social-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		background: none;
		border-radius: 50%;
		transition: none;
		font-size: 2rem;
	}

	.footer-social-icon:hover {
		color: inherit;
		background: none;
	}

	.footer-social-icon img {
		filter: grayscale(1) brightness(0.8);
		opacity: 0.7;
		transition: filter 0.2s, opacity 0.2s;
	}

	.footer-social-icon:hover img {
		filter: none;
		opacity: 1;
	}

	/* Responsive Design */
	@media (max-width: 768px) {
		.footer-content {
			flex-direction: column;
			align-items: stretch;
		}
	}

	@media (max-width: 480px) {
		.footer-content {
			flex-direction: column;
		}
	}
</style> 