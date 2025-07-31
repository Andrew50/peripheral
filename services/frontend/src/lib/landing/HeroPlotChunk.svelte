<script lang="ts">
	import Plot from 'svelte-plotly.js';
	import type { PlotData } from '$lib/features/chat/interface';

	export let plotData: PlotData;
	export const plotKey: string = ''; // Unique identifier for this plot

	// Hero-specific config - more minimal
	const heroConfig = {
		displayModeBar: false,
		displaylogo: false,
		showTips: false,
		responsive: true
	};

	// Hero-specific layout optimized for smaller spaces
	const heroLayout = {
		font: {
			family: 'Geist, system-ui, sans-serif',
			size: 10,
			color: '#1a1a1a' // Dark color for hero background
		},
		paper_bgcolor: 'transparent',
		plot_bgcolor: 'transparent', 
		margin: { l: 35, r: 35, t: 15, b: 25, autoexpand: true },
		autosize: true,
		showlegend: false, // Disable legend in hero for space
		hoverlabel: {
			bgcolor: '#ffffff',
			bordercolor: '#666666',
			borderwidth: 1,
			font: {
				color: '#1a1a1a',
				size: 11,
				family: 'Geist, system-ui, sans-serif'
			},
			namelength: -1,
			align: 'left' as const
		},
		xaxis: {
			gridcolor: 'rgba(26, 26, 26, 0.15)',
			linecolor: 'rgba(26, 26, 26, 0.3)',
			tickfont: { color: '#1a1a1a', size: 9 },
			titlefont: { color: '#1a1a1a', size: 10 },
			automargin: true,
			tickangle: 0, // Keep horizontal for space
			title: {
				standoff: 15
			}
		},
		yaxis: {
			gridcolor: 'rgba(26, 26, 26, 0.15)',
			linecolor: 'rgba(26, 26, 26, 0.3)',
			tickfont: { color: '#1a1a1a', size: 9 },
			titlefont: { color: '#1a1a1a', size: 10 },
			automargin: true
		}
	};

	// Hero color palette - neutral theme
	const heroColorPalette = [
		'#666666', // Primary neutral color
		'#999999', // Light gray
		'#FF9800', // Orange
		'#4CAF50', // Green
		'#9C27B0', // Purple
		'#F44336', // Red
		'#777777', // Medium gray
		'#FFC107'  // Amber
	];

	function processHeroTraceData(trace: any, index: number): any {
		const processedTrace = { ...trace };

		// Apply hero colors
		if (!processedTrace.marker?.color && !processedTrace.line?.color) {
			const color = heroColorPalette[index % heroColorPalette.length];

			if (plotData.chart_type === 'line' || plotData.chart_type === 'scatter') {
				if (!processedTrace.line) processedTrace.line = {};
				processedTrace.line.color = color;
				processedTrace.line.width = 2; // Slightly thicker for visibility

				if (plotData.chart_type === 'scatter' && !processedTrace.marker) {
					processedTrace.marker = { color, size: 6 };
				}
			} else {
				if (!processedTrace.marker) processedTrace.marker = {};
				processedTrace.marker.color = color;
			}
		}

		// Set trace type
		if (!processedTrace.type) {
			switch (plotData.chart_type) {
				case 'line':
					processedTrace.type = 'scatter';
					if (!processedTrace.mode) processedTrace.mode = 'lines';
					break;
				case 'scatter':
					processedTrace.type = 'scatter';
					if (!processedTrace.mode) processedTrace.mode = 'markers';
					break;
				case 'bar':
					processedTrace.type = 'bar';
					break;
				case 'histogram':
					processedTrace.type = 'histogram';
					if (!processedTrace.nbinsx) {
						processedTrace.nbinsx = Math.min(20, Math.max(8, Math.floor(Math.sqrt(processedTrace.x?.length || 10))));
					}
					break;
				case 'heatmap':
					processedTrace.type = 'heatmap';
					processedTrace.colorscale = [
						[0, '#f44336'],
						[0.5, '#ffffff'], 
						[1, '#4caf50']
					];
					break;
			}
		}

		return processedTrace;
	}

	// Process trace data for hero
	$: heroProcessedData = plotData.data
		.map((trace, index) => processHeroTraceData(trace, index))
		.filter((trace) => trace !== null);

	// Hero layout (simpler, no dual y-axis support)
	$: {
		const { width, height, ...userLayoutWithoutDimensions } = plotData.layout || {};
		
		layout = {
			...heroLayout,
			...userLayoutWithoutDimensions,
			title: '' // Always hide title in layout, show it separately if needed
		};
	}

	let layout: any;
</script>

<div class="hero-plot-container">
	{#if plotData.title}
		<div class="hero-plot-title">
			{plotData.title}
		</div>
	{/if}

	<div class="hero-plot-content">
		<Plot data={heroProcessedData} {layout} config={heroConfig} fillParent={false} debounce={200} />
	</div>
</div>

<style>
	.hero-plot-container {
		margin: 0.5rem 0;
		max-width: 100%;
		background: none;
		border: none;
		overflow: hidden;
	}

	.hero-plot-title {
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--color-dark);
		padding: 0.5rem 0.75rem 0.25rem;
		text-align: center;
	}

	.hero-plot-content {
		padding: 0.5rem;
		display: flex;
		justify-content: center;
		align-items: center;
		min-height: 250px;
		height: 250px;
		width: 100%;
		overflow: visible;
	}

	/* Mobile adjustments for hero plots */
	@media (width <= 768px) {
		.hero-plot-content {
			min-height: 200px;
			height: 200px;
		}

		.hero-plot-title {
			font-size: 0.8rem;
			padding: 0.4rem 0.6rem 0.2rem;
		}
	}

	@media (width <= 480px) {
		.hero-plot-content {
			min-height: 180px;
			height: 180px;
			padding: 0.3rem;
		}

		.hero-plot-title {
			font-size: 0.75rem;
		}
	}

	/* Override Plotly's default styles for hero theme */
	:global(.hero-plot-content .plotly) {
		background: transparent !important;
	}

	:global(.hero-plot-content .js-plotly-plot) {
		background: transparent !important;
	}
</style> 