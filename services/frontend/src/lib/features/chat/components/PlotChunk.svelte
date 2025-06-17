<script lang="ts">
	import Plot from 'svelte-plotly.js';
	import type { PlotData } from '../interface';
	
	export let plotData: PlotData;
	export let plotKey: string; // Unique identifier for this plot

	// Default styling that matches the app theme
	const defaultConfig = {
		displayModeBar: true,
		modeBarButtonsToRemove: ['pan2d', 'select2d', 'lasso2d', 'autoScale2d'] as any[],
		displaylogo: false,
		responsive: true
	};

	const defaultLayout = {
		font: {
			family: 'Inter, system-ui, sans-serif',
			size: 12,
			color: '#e2e8f0' // text-slate-200
		},
		paper_bgcolor: 'rgba(15, 23, 42, 0.8)', // slate-900 with opacity
		plot_bgcolor: 'rgba(30, 41, 59, 0.5)', // slate-800 with opacity
		margin: { l: 60, r: 30, t: 50, b: 50 },
		showlegend: true,
		legend: {
			font: { color: '#e2e8f0' },
			bgcolor: 'rgba(15, 23, 42, 0.8)',
			bordercolor: 'rgba(71, 85, 105, 0.5)',
			borderwidth: 1
		},
		xaxis: {
			gridcolor: 'rgba(71, 85, 105, 0.3)',
			linecolor: 'rgba(71, 85, 105, 0.5)',
			tickfont: { color: '#cbd5e1' }, // text-slate-300
			titlefont: { color: '#e2e8f0' }
		},
		yaxis: {
			gridcolor: 'rgba(71, 85, 105, 0.3)',
			linecolor: 'rgba(71, 85, 105, 0.5)',
			tickfont: { color: '#cbd5e1' },
			titlefont: { color: '#e2e8f0' }
		}
	};

	// Color palette for multiple traces
	const colorPalette = [
		'#60a5fa', // blue-400
		'#34d399', // emerald-400
		'#f87171', // red-400
		'#fbbf24', // amber-400
		'#c084fc', // purple-400
		'#fb7185', // rose-400
		'#38bdf8', // sky-400
		'#4ade80'  // green-400
	];

	function processTraceData(trace: any, index: number): any {
		const processedTrace = { ...trace };
		
		// Apply default colors if not specified
		if (!processedTrace.marker?.color && !processedTrace.line?.color) {
			const color = colorPalette[index % colorPalette.length];
			
			if (plotData.chart_type === 'line' || plotData.chart_type === 'scatter') {
				if (!processedTrace.line) processedTrace.line = {};
				processedTrace.line.color = color;
				
				if (plotData.chart_type === 'scatter' && !processedTrace.marker) {
					processedTrace.marker = { color };
				}
			} else {
				if (!processedTrace.marker) processedTrace.marker = {};
				processedTrace.marker.color = color;
			}
		}

		// Set trace type based on chart_type if not specified
		if (!processedTrace.type) {
			switch (plotData.chart_type) {
				case 'line':
					processedTrace.type = 'scatter';
					processedTrace.mode = 'lines';
					break;
				case 'scatter':
					processedTrace.type = 'scatter';
					processedTrace.mode = 'markers';
					break;
				case 'bar':
					processedTrace.type = 'bar';
					break;
				case 'histogram':
					processedTrace.type = 'histogram';
					break;
				case 'heatmap':
					processedTrace.type = 'heatmap';
					processedTrace.colorscale = 'Viridis';
					break;
			}
		}

		return processedTrace;
	}

	// Process trace data reactively
	$: processedData = plotData.data.map((trace, index) => 
		processTraceData(trace, index)
	);

	// Merge layouts (user layout takes precedence)
	$: layout = {
		...defaultLayout,
		...plotData.layout,
		title: plotData.title || plotData.layout?.title || ''
	};
</script>

<div class="plot-chunk-wrapper glass glass--rounded glass--responsive">
	{#if plotData.title}
		<div class="plot-title">
			{plotData.title}
		</div>
	{/if}
	
	<div class="plot-container">
		<Plot 
			data={processedData} 
			{layout} 
			config={defaultConfig}
			fillParent="width"
			debounce={250}
		/>
	</div>
</div>

<style>
	.plot-chunk-wrapper {
		margin: 1rem 0;
		overflow: hidden;
	}

	.plot-title {
		padding: 1rem 1rem 0.5rem 1rem;
		font-weight: 600;
		font-size: 1.1rem;
		color: #e2e8f0;
		border-bottom: 1px solid rgba(71, 85, 105, 0.3);
		margin-bottom: 0.5rem;
	}

	.plot-container {
		min-height: 400px;
		width: 100%;
		padding: 0.5rem;
	}

	/* Responsive adjustments */
	@media (max-width: 768px) {
		.plot-container {
			min-height: 300px;
		}
		
		.plot-title {
			font-size: 1rem;
		}
	}

	/* Override Plotly's default styles to match our theme */
	:global(.plot-container .plotly) {
		background: transparent !important;
	}
	
	:global(.plot-container .plotly .modebar) {
		background: rgba(15, 23, 42, 0.8) !important;
		border: 1px solid rgba(71, 85, 105, 0.3) !important;
		border-radius: 4px !important;
	}
	
	:global(.plot-container .plotly .modebar-btn) {
		color: #cbd5e1 !important;
	}
	
	:global(.plot-container .plotly .modebar-btn:hover) {
		background: rgba(71, 85, 105, 0.3) !important;
		color: #e2e8f0 !important;
	}
</style>