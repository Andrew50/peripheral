<script>
	let testId = 'preview-test';
	let ogUrl = `/og/${testId}`;
</script>

<div class="container">
	<h1>Open Graph Image Preview</h1>
	
	<div class="preview-section">
		<h2>Generated OG Image:</h2>
		<img src={ogUrl} alt="Open Graph Preview" class="og-image" />
		
		<div class="controls">
			<label>
				Test ID: 
				<input bind:value={testId} on:input={() => ogUrl = `/og/${testId}`} />
			</label>
			<p>URL: <code>{ogUrl}</code></p>
			<button on:click={() => window.open(ogUrl, '_blank')}>Open Image in New Tab</button>
		</div>
	</div>
</div>

<style>
	.container {
		padding: 2rem;
		max-width: 800px;
		margin: 0 auto;
		font-family: system-ui, sans-serif;
	}
	
	.og-image {
		max-width: 100%;
		border: 2px solid #ccc;
		border-radius: 8px;
		box-shadow: 0 4px 12px rgb(0 0 0 / 10%);
	}
	
	.controls {
		margin-top: 1rem;
		padding: 1rem;
		background: #f5f5f5;
		border-radius: 8px;
	}
	
	.controls label {
		display: block;
		margin-bottom: 0.5rem;
	}
	
	.controls input {
		margin-left: 0.5rem;
		padding: 0.25rem 0.5rem;
		border: 1px solid #ccc;
		border-radius: 4px;
	}
	
	.controls button {
		margin-top: 0.5rem;
		padding: 0.5rem 1rem;
		background: #007bff;
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}
	
	.controls button:hover {
		background: #0056b3;
	}
	
	code {
		background: #e9ecef;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		font-family: monospace;
	}
</style> 