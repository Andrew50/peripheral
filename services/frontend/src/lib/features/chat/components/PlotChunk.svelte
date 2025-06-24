<script lang="ts">
	import Plot from 'svelte-plotly.js';
	import type { PlotData } from '../interface';
	
	export let plotData: PlotData;
	export let plotKey: string; // Unique identifier for this plot

	// Default styling that matches the app theme
	const defaultConfig = {
		displayModeBar: false,
		displaylogo: false,
		showTips: false
	};

	const defaultLayout = {
		font: {
			family: 'Inter, system-ui, sans-serif',
			size: 12,
			color: '#e2e8f0' // text-slate-200
		},
		paper_bgcolor: 'transparent', // Match chat background
		plot_bgcolor: 'transparent', // Match chat background
		margin: { l: 45, r: 60, t: 10, b: 30, autoexpand: true },
		autosize: true,
		showlegend: true,
		legend: {
			font: { color: '#e2e8f0' },
			bgcolor: 'transparent',
			borderwidth: 0,
			orientation: 'h' as const,
			x: 0.5,
			xanchor: 'center' as const,
			y: -0.3,
			yanchor: 'top' as const
		},
		xaxis: {
			gridcolor: 'rgba(71, 85, 105, 0.3)',
			linecolor: 'rgba(71, 85, 105, 0.5)',
			tickfont: { color: '#cbd5e1', size: 11 }, // text-slate-300
			titlefont: { color: '#e2e8f0' },
			automargin: true,
			tickangle: -45,
			title: {
				standoff: 25
			}
		},
		yaxis: {
			gridcolor: 'rgba(71, 85, 105, 0.3)',
			linecolor: 'rgba(71, 85, 105, 0.5)',
			tickfont: { color: '#cbd5e1', size: 11 },
			titlefont: { color: '#e2e8f0' },
			automargin: true
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
		
		// Filter out malformed traces for histograms
		if (plotData.chart_type === 'histogram') {
			// For histograms, we need either x data or y data with actual values
			const hasValidX = processedTrace.x && Array.isArray(processedTrace.x) && processedTrace.x.length > 0;
			const hasValidY = processedTrace.y && Array.isArray(processedTrace.y) && processedTrace.y.length > 0;
			
			// Skip traces that don't have any valid data
			if (!hasValidX && !hasValidY) {
				return null;
			}
		}
		
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

		// Set trace type - respect individual trace types, fall back to chart_type
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
					
					// Clean up empty arrays that might confuse Plotly
					if (processedTrace.y && Array.isArray(processedTrace.y) && processedTrace.y.length === 0) {
						delete processedTrace.y;
					}
					if (processedTrace.z && Array.isArray(processedTrace.z) && processedTrace.z.length === 0) {
						delete processedTrace.z;
					}
					
					// Configure automatic binning if not specified
					if (!processedTrace.autobinx && !processedTrace.xbins && processedTrace.x && processedTrace.x.length > 0) {
						processedTrace.autobinx = true;
					}
					if (!processedTrace.autobiny && !processedTrace.ybins && processedTrace.y && processedTrace.y.length > 0) {
						processedTrace.autobiny = true;
					}
					// Set default number of bins if using x data
					if (processedTrace.x && processedTrace.x.length > 0 && !processedTrace.nbinsx && !processedTrace.xbins) {
						processedTrace.nbinsx = Math.min(30, Math.max(10, Math.floor(Math.sqrt(processedTrace.x.length))));
					}
					// Set default number of bins if using y data  
					if (processedTrace.y && processedTrace.y.length > 0 && !processedTrace.nbinsy && !processedTrace.ybins) {
						processedTrace.nbinsy = Math.min(30, Math.max(10, Math.floor(Math.sqrt(processedTrace.y.length))));
					}
					break;
				case 'heatmap':
					processedTrace.type = 'heatmap';
					// Clean red-green colorscale without orange
					processedTrace.colorscale = [
						[0, '#d32f2f'],    // Dark red for most negative
						[0.25, '#f44336'], // Medium red
						[0.5, '#424242'],  // Dark neutral/gray
						[0.75, '#4caf50'], // Medium green
						[1, '#2e7d32']     // Dark green for most positive
					];
					// Ensure zero is always at the center (gray) so negative=red, positive=green
					processedTrace.zmid = 0;
					// Configure colorbar positioning for heatmaps
					if (!processedTrace.colorbar) {
						processedTrace.colorbar = {
							x: -0.2, // Position colorbar further to the right
							xanchor: 'left',
							thickness: 12,
							len: 0.8,
							xpad: 10 // Add padding between plot and colorbar
						};
					}
					break;
			}
		}

		// Handle bar positioning for dual y-axis charts (only for actual bar traces)
		if (processedTrace.type === 'bar' && plotData.data.some(trace => trace.yaxis === 'y2')) {
			// Determine if this trace is on primary or secondary axis
			const isSecondaryAxis = processedTrace.yaxis === 'y2';
			const primaryBarTraces = plotData.data.filter(trace => (!trace.yaxis || trace.yaxis === 'y') && (trace.type === 'bar' || (!trace.type && plotData.chart_type === 'bar')));
			const secondaryBarTraces = plotData.data.filter(trace => trace.yaxis === 'y2' && (trace.type === 'bar' || (!trace.type && plotData.chart_type === 'bar')));
			
			if (primaryBarTraces.length > 0 && secondaryBarTraces.length > 0) {
				// Calculate bar width and offset for grouping
				const totalBarTraces = primaryBarTraces.length + secondaryBarTraces.length;
				const barWidth = 0.8 / totalBarTraces; // Total width divided by number of bar traces
				
				if (isSecondaryAxis) {
					// Secondary axis bar traces get offset to the right
					const secondaryIndex = secondaryBarTraces.findIndex(t => t === plotData.data.find(d => d === trace));
					processedTrace.width = barWidth;
					processedTrace.offset = barWidth * (primaryBarTraces.length + secondaryIndex) - 0.4 + barWidth/2;
				} else {
					// Primary axis bar traces
					const primaryIndex = primaryBarTraces.findIndex(t => t === plotData.data.find(d => d === trace));
					processedTrace.width = barWidth;
					processedTrace.offset = barWidth * primaryIndex - 0.4 + barWidth/2;
				}
			}
		}

		return processedTrace;
	}

	// Process trace data reactively
	$: processedData = plotData.data
		.map((trace, index) => processTraceData(trace, index))
		.filter(trace => trace !== null);
	
	// Check if any traces use secondary y-axis
	$: hasSecondaryYAxis = plotData.data.some(trace => trace.yaxis === 'y2');
	
	// Declare layout variable
	let layout: any;

	// Merge layouts (user layout takes precedence, but handle dual y-axis)
	// Destructure width and height out of plotData.layout to prevent them from overriding fillParent
	$: {
		const { width, height, ...userLayoutWithoutDimensions } = plotData.layout || {};
		
		// Base layout configuration
		const baseLayout = {
			...defaultLayout,
			...userLayoutWithoutDimensions,
			// Don't set title in layout if we're showing it separately
			title: plotData.title ? '' : (plotData.layout?.title || ''),
		};

		if (hasSecondaryYAxis) {
			// Check if we have any bar traces for proper barmode setting
			const hasBarTraces = plotData.data.some(trace => trace.type === 'bar' || (!trace.type && plotData.chart_type === 'bar'));
			
			// Configure dual y-axis layout
			layout = {
				...baseLayout,
				// Adjust margins for dual y-axis
				margin: { l: 60, r: 80, t: 10, b: 30, autoexpand: true },
				// For charts with bar traces and dual y-axis, use overlay mode to allow manual positioning
				barmode: hasBarTraces ? 'overlay' as const : userLayoutWithoutDimensions.barmode,
				// Primary y-axis (left side)
				yaxis: {
					...defaultLayout.yaxis,
					...userLayoutWithoutDimensions.yaxis,
					side: 'left' as const,
					title: userLayoutWithoutDimensions.yaxis?.title || ''
				},
				// Secondary y-axis (right side)
				yaxis2: {
					...defaultLayout.yaxis,
					...userLayoutWithoutDimensions.yaxis2,
					side: 'right' as const,
					overlaying: 'y' as const,
					title: userLayoutWithoutDimensions.yaxis2?.title || '',
					// Ensure grid lines don't overlap by disabling on secondary axis
					showgrid: false
				}
			};
		} else {
			// Single y-axis layout (keep existing behavior)
			layout = {
				...baseLayout,
				yaxis: {
					...defaultLayout.yaxis,
					...userLayoutWithoutDimensions.yaxis,
					side: 'right' as const
				}
			};
		}
	}
</script>

<div class="chunk-plot-container">
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
			fillParent={true}
			debounce={250}
		/>
	</div>
</div>

<style>
	.chunk-plot-container {
		margin-bottom: 1rem;
	}

	.plot-title {
		font-size: 1rem;
		font-weight: 600;
		color: var(--text-primary, #fff);
		margin-bottom: 0.75rem;
		padding-bottom: 0.25rem;
		border-bottom: 1px solid rgba(71, 85, 105, 0.2);
		line-height: 1.4;
	}

	.plot-container {
		min-height: 350px;
		height: 350px;
		width: 100%;
		overflow: hidden;
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