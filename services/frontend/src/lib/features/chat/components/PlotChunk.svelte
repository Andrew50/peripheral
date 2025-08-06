<script lang="ts">
	import Plot from 'svelte-plotly.js';
	import type { PlotData } from '../interface';
	import { onDestroy } from 'svelte';

	export let plotData: PlotData;
	export const plotKey: string = ''; // Unique identifier for this plot

	// Reference to the entire plot chunk container
	let chunkContainer: HTMLDivElement;
	let copyImageFeedback = false;
	let copyImageTimeout: ReturnType<typeof setTimeout> | null = null;

	// Function to copy plot as image to clipboard
	async function copyPlotImage() {
		try {
			if (!chunkContainer) {
				console.error('Plot chunk container not available');
				return;
			}

			// Import html2canvas dynamically
			const html2canvas = await import('html2canvas');
			
			// Create a clone of the container for off-screen rendering
			const clone = chunkContainer.cloneNode(true) as HTMLDivElement;
			
			// Position clone off-screen
			clone.style.position = 'absolute';
			clone.style.left = '-9999px';
			clone.style.top = '0';
			clone.style.width = chunkContainer.offsetWidth + 'px';
			
			// Ensure the clone container has position relative for absolute watermark positioning
			const plotContainer = clone.querySelector('.chunk-plot-container') as HTMLDivElement;
			if (plotContainer) {
				plotContainer.style.position = 'relative';
			}
			
			// Create and add watermark to the clone
			const watermark = document.createElement('div');
			watermark.className = 'watermark-offscreen';
			watermark.innerHTML = 'Powered by <span class="watermark-brand">Peripheral.io</span>';
			
			// Remove plot-actions from clone to eliminate extra space
			const plotActions = clone.querySelector('.plot-actions');
			if (plotActions) {
				plotActions.remove();
			}
			
			// Append watermark to the clone
			clone.appendChild(watermark);
			
			// Append clone to body temporarily
			document.body.appendChild(clone);
			
			try {
				// Capture the cloned container with watermark
				const canvas = await html2canvas.default(clone, {
					backgroundColor: '#121212', // Match chat background color
					scale: 4, // Higher quality
					logging: false,
					useCORS: true,
					// Remove the y offset to ensure we capture the full container including bottom
				});

				// Convert canvas to blob
				const blob = await new Promise<Blob>((resolve) => {
					canvas.toBlob((blob) => {
						resolve(blob!);
					}, 'image/png');
				});

				// Copy to clipboard
				await navigator.clipboard.write([
					new ClipboardItem({
						'image/png': blob
					})
				]);

				// Show success feedback
				copyImageFeedback = true;
				if (copyImageTimeout) {
					clearTimeout(copyImageTimeout);
				}
				copyImageTimeout = setTimeout(() => {
					copyImageFeedback = false;
					copyImageTimeout = null;
				}, 2000);
			} finally {
				// Always cleanup the clone
				document.body.removeChild(clone);
			}
		} catch (error) {
			console.error('Failed to copy plot image:', error);
			// Could show an error message to user here
		}
	}

	// Cleanup timeout on component destroy
	onDestroy(() => {
		if (copyImageTimeout) {
			clearTimeout(copyImageTimeout);
		}
	});

	// Default styling that matches the app theme
	const defaultConfig = {
		displayModeBar: false,
		displaylogo: false,
		showTips: false
	};

	const defaultHoverLabel = {
		bgcolor: '#1e293b',
		bordercolor: '#475569',
		borderwidth: 1,
		font: {
			color: '#ffffff',
			size: 11,
			family: 'Geist, Inter, system-ui, sans-serif'
		},
		namelength: -1,
	};

	const defaultLayout = {
		font: {
			family: 'Geist, Inter, system-ui, sans-serif',
			size: 12,
			color: '#f8fafc' // text-slate-50 (less transparent)
		},
		paper_bgcolor: 'transparent', // Match chat background
		plot_bgcolor: 'transparent', // Match chat background
		margin: { l: 45, r: 60, t: 40, b: 40, autoexpand: true },
		autosize: true,
		showlegend: true,
		legend: {
			font: { color: '#f8fafc' }, // text-slate-50 (less transparent)
			bgcolor: 'transparent',
			borderwidth: 0,
			orientation: 'h' as const,
			x: 0.5,
			xanchor: 'center' as const,
			y: 1,
			yanchor: 'bottom' as const
		},
		hoverlabel: defaultHoverLabel,
		xaxis: {
			linecolor: 'rgba(71, 85, 105, 0.3)',
			tickfont: { color: '#f1f5f9', size: 11 },
			titlefont: { color: '#f8fafc' },
			automargin: true,
			title: {
				standoff: 25
			}
		},
		yaxis: {
			linecolor: 'rgba(71, 85, 105, 0.5)',
			tickfont: { color: '#f1f5f9', size: 11 },
			titlefont: { color: '#f8fafc' },
			automargin: true
		}
	};

	// Color palette for multiple traces
	const colorPalette = [
		'#FFD43B', // sunflower yellow
		'#64C9CF', // turquoise
		'#FC6B3F', // vivid orange
		'#A17BFE', // soft indigo
		'#03DAC6', // aqua
		'#F6BD60', // warm sand
		'#9DC2FF', // sky blue
		'#F95738', // vermilion
		'#45C4B0', // teal
		'#FF99C8'  // cotton-candy pink
	];

	// Helper function to create standard layout configurations
	const createStandardLayout = (baseLayout: any, userLayout: any, yAxisSide: 'left' | 'right' = 'right', gridAlpha = 0.03) => {
		// Calculate padded ranges if not explicitly set by user
		const xRange = userLayout.xaxis?.range || calculatePaddedRange(plotData.data, 'x', 0.02);
		const yRange = userLayout.yaxis?.range || calculatePaddedRange(plotData.data, 'y', 0.10);

		return {
			...baseLayout,
			xaxis: {
				...baseLayout.xaxis,
				...userLayout.xaxis,
				gridcolor: `rgba(255, 255, 255, ${gridAlpha})`,
				linecolor: 'rgba(255, 255, 255, 0.8)',
				...(xRange && { range: xRange }),
				title: capitalizeAxisTitle(userLayout.xaxis?.title || ''),
				tickfont: { 
					color: '#f1f5f9', 
					size: 11,
					family: 'Geist, Inter, system-ui, sans-serif'
				},
				titlefont: { 
					color: '#f8fafc',
					family: 'Geist, Inter, system-ui, sans-serif'
				},
			},
			hoverlabel: defaultHoverLabel,
			yaxis: {
				...baseLayout.yaxis,
				...userLayout.yaxis,
				side: yAxisSide,
				gridcolor: `rgba(255, 255, 255, ${gridAlpha})`,
				linecolor: 'rgba(255, 255, 255, 0.8)',
				...(yRange && { range: yRange }),
				title: capitalizeAxisTitle(userLayout.yaxis?.title || ''),
				tickfont: { 
					color: '#f1f5f9', 
					size: 11,
					family: 'Geist, Inter, system-ui, sans-serif'
				},
				titlefont: { 
					color: '#f8fafc',
					family: 'Geist, Inter, system-ui, sans-serif'
				},
			},
			legend: {
				...(baseLayout.legend ?? {}),
				...(userLayout.legend ?? {}),
				tickfont: { 
					color: '#f1f5f9', 
					size: 11,
					family: 'Geist, Inter, system-ui, sans-serif'
				},
				titlefont: { 
					color: '#f8fafc',
					family: 'Geist, Inter, system-ui, sans-serif'
				},
			}
		};
	};

	// Chart type configurations - consolidates all chart-specific logic
	const chartTypeConfigs = {
		line: {
			configureTrace: (trace: any, index: number) => {
				// Set trace type and mode
				if (!trace.type) trace.type = 'scatter';
				if (!trace.mode) trace.mode = 'lines';

				// Apply colors
				const color = colorPalette[index % colorPalette.length];
				if (!trace.line?.color) {
					if (!trace.line) trace.line = {};
					trace.line.color = color;
				}
				// Add value labels above points
				if (trace.y && Array.isArray(trace.y) && trace.y.length < 12 && !trace.text) {
					// Format values for display
					trace.text = trace.y.map((value: any) => {
						// Hide null, undefined, or NaN values
						if (value == null || (typeof value === 'number' && isNaN(value))) {
							return '';
						}
						
						if (typeof value === 'number') {
							// For scatter plots, typically show more precision for smaller values
							if (Math.abs(value) < 1 && Math.abs(value) > 0) {
								return value.toFixed(3);
							} else if (value % 1 === 0) {
								// Show whole numbers without decimals
								return value.toString();
							} else {
								// Show decimal values with appropriate precision
								return value.toFixed(2);
							}
						}
						return String(value);
					});
					// Smart positioning: above for local maxima, below for local minima
					trace.textposition = trace.y.map((value: number, index: number) => {
						const prevValue = index > 0 ? trace.y[index - 1] : null;
						const nextValue = index < trace.y.length - 1 ? trace.y[index + 1] : null;
						
						// Determine if this is a local min or max
						let isLocalMax = false;
						let isLocalMin = false;
						
						if (prevValue !== null && nextValue !== null) {
							// Middle points: compare with both neighbors
							isLocalMax = value >= prevValue && value >= nextValue;
							isLocalMin = value <= prevValue && value <= nextValue;
						} else if (prevValue !== null) {
							// Last point: compare with previous
							isLocalMax = value >= prevValue;
							isLocalMin = value <= prevValue;
						} else if (nextValue !== null) {
							// First point: compare with next
							isLocalMax = value >= nextValue;
							isLocalMin = value <= nextValue;
						}
						
						// Position text based on local extrema
						if (isLocalMin && !isLocalMax) {
							return 'bottom center'; // Text below for minima
						} else {
							return 'top center'; // Text above for maxima and neutral points
						}
					});
					
					trace.textfont = {
						color: '#ffffff',
						size: 12,
						family: 'Inter, system-ui, sans-serif',
					};
					
					// Update mode to include text display
					if (trace.mode === 'markers') {
						trace.mode = 'markers+text';
					} else if (trace.mode === 'lines') {
						trace.mode = 'lines+text';
					} else if (trace.mode === 'lines+markers') {
						trace.mode = 'lines+markers+text';
					} else if (!trace.mode.includes('text')) {
						trace.mode = trace.mode + '+text';
					}
				}
				return trace;
			},
			configureLayout: (baseLayout: any, userLayout: any) => {
				// For line charts, use 0 padding on x-axis and default padding on y-axis
				const xRange = userLayout.xaxis?.range || calculatePaddedRange(plotData.data, 'x', 0);
				const yRange = userLayout.yaxis?.range || calculatePaddedRange(plotData.data, 'y', 0.10);

				return {
					...baseLayout,
					xaxis: {
						...baseLayout.xaxis,
						...userLayout.xaxis,
						gridcolor: 'rgba(255, 255, 255, 0.03)',
						linecolor: 'rgba(255, 255, 255, 0.8)',
						...(xRange && { range: xRange }),
						title: capitalizeAxisTitle(userLayout.xaxis?.title || ''),
						tickfont: { 
							color: '#f1f5f9', 
							size: 11,
							family: 'Geist, Inter, system-ui, sans-serif'
						},
						titlefont: { 
							color: '#f8fafc',
							family: 'Geist, Inter, system-ui, sans-serif'
						},
					},
					hoverlabel: defaultHoverLabel,
					yaxis: {
						...baseLayout.yaxis,
						...userLayout.yaxis,
						side: 'right',
						gridcolor: 'rgba(255, 255, 255, 0.03)',
						linecolor: 'rgba(255, 255, 255, 0.8)',
						...(yRange && { range: yRange }),
						title: capitalizeAxisTitle(userLayout.yaxis?.title || ''),
						tickfont: { 
							color: '#f1f5f9', 
							size: 11,
							family: 'Geist, Inter, system-ui, sans-serif'
						},
						titlefont: { 
							color: '#f8fafc',
							family: 'Geist, Inter, system-ui, sans-serif'
						},
					},
					legend: {
						...(baseLayout.legend ?? {}),
						...(userLayout.legend ?? {}),
						tickfont: { 
							color: '#f1f5f9', 
							size: 11,
							family: 'Geist, Inter, system-ui, sans-serif'
						},
						titlefont: { 
							color: '#f8fafc',
							family: 'Geist, Inter, system-ui, sans-serif'
						},
					}
				};
			}
		},
		scatter: {
			configureTrace: (trace: any, index: number) => {
				// Set trace type and mode
				if (!trace.type) trace.type = 'scatter';
				if (!trace.mode) trace.mode = 'markers';

				// Apply colors to both line and marker
				const color = colorPalette[index % colorPalette.length];
				if (!trace.marker?.color && !trace.line?.color) {
					if (!trace.line) trace.line = {};
					trace.line.color = color;
					if (!trace.marker) trace.marker = {};
					trace.marker.color = color;
				}

				// Add value labels above points
				if (trace.y && Array.isArray(trace.y) && trace.y.length < 12 && !trace.text) {
					// Format values for display
					trace.text = trace.y.map((value: any) => {
						// Hide null, undefined, or NaN values
						if (value == null || (typeof value === 'number' && isNaN(value))) {
							return '';
						}
						
						if (typeof value === 'number') {
							// For scatter plots, typically show more precision for smaller values
							if (Math.abs(value) < 1 && Math.abs(value) > 0) {
								return value.toFixed(3);
							} else if (value % 1 === 0) {
								// Show whole numbers without decimals
								return value.toString();
							} else {
								// Show decimal values with appropriate precision
								return value.toFixed(2);
							}
						}
						return String(value);
					});
					// Smart positioning: above for local maxima, below for local minima
					trace.textposition = trace.y.map((value: number, index: number) => {
						const prevValue = index > 0 ? trace.y[index - 1] : null;
						const nextValue = index < trace.y.length - 1 ? trace.y[index + 1] : null;
						
						// Determine if this is a local min or max
						let isLocalMax = false;
						let isLocalMin = false;
						
						if (prevValue !== null && nextValue !== null) {
							// Middle points: compare with both neighbors
							isLocalMax = value >= prevValue && value >= nextValue;
							isLocalMin = value <= prevValue && value <= nextValue;
						} else if (prevValue !== null) {
							// Last point: compare with previous
							isLocalMax = value >= prevValue;
							isLocalMin = value <= prevValue;
						} else if (nextValue !== null) {
							// First point: compare with next
							isLocalMax = value >= nextValue;
							isLocalMin = value <= nextValue;
						}
						
						// Position text based on local extrema
						if (isLocalMin && !isLocalMax) {
							return 'bottom center'; // Text below for minima
						} else {
							return 'top center'; // Text above for maxima and neutral points
						}
					});
					
					trace.textfont = {
						color: '#ffffff',
						size: 12,
						family: 'Inter, system-ui, sans-serif',
					};
					
					// Update mode to include text display
					if (trace.mode === 'markers') {
						trace.mode = 'markers+text';
					} else if (trace.mode === 'lines') {
						trace.mode = 'lines+text';
					} else if (trace.mode === 'lines+markers') {
						trace.mode = 'lines+markers+text';
					} else if (!trace.mode.includes('text')) {
						trace.mode = trace.mode + '+text';
					}
				}

				return trace;
			},
			configureLayout: (baseLayout: any, userLayout: any) => createStandardLayout(baseLayout, userLayout, 'left', 0.1)
		},
		bar: {
			configureTrace: (trace: any, index: number, options?: { allTraces?: any[] }) => {
				// Set trace type
				if (!trace.type) trace.type = 'bar';

				// Apply colors and styling
				const color = colorPalette[index % colorPalette.length];
				if (!trace.marker) trace.marker = {};
				
				if (!trace.marker.color) trace.marker.color = color;
				trace.marker.opacity = 1;
				trace.opacity = 1;

				// Add feint border lines around bars
				if (!trace.marker.line) {
					trace.marker.line = {
						color: 'rgba(71, 85, 105, 0.4)',
						width: 0.5
					};
				}

				// Add value labels above bars
				if (trace.y && Array.isArray(trace.y) && !trace.text && trace.y.length < 12) {
					// Format values for display
					trace.text = trace.y.map((value: any) => {
						// Hide null, undefined, or NaN values
						if (value == null || (typeof value === 'number' && isNaN(value))) {
							return '';
						}
						
						if (typeof value === 'number') {
							// Only round values > 100,000 to 2 decimals
							if (Math.abs(value) > 100000) {
								if (Math.abs(value) >= 1000000) {
									// For millions, show 2 decimals
									return (value / 1000000).toFixed(2) + 'M';
								} else {
									// For values > 100K but < 1M, show 2 decimals in K format
									return (value / 1000).toFixed(2) + 'K';
								}
							} else {
								// For values ≤ 100,000, show full precision
								if (Math.abs(value) < 1 && Math.abs(value) > 0) {
									return value.toFixed(3);
								} else if (value % 1 === 0) {
									// Show whole numbers without decimals
									return value.toString();
								} else {
									// Show decimal values with appropriate precision
									return value.toFixed(2);
								}
							}
						}
						return String(value);
					});
					trace.textposition = 'outside';
					trace.textfont = {
						color: '#f1f5f9',
						size: 10,
						family: 'Inter, system-ui, sans-serif'
					};
				}

				// Handle bar positioning for dual y-axis charts (only if allTraces provided)
				if (options?.allTraces && options.allTraces.some((t) => t.yaxis === 'y2')) {
					const isSecondaryAxis = trace.yaxis === 'y2';
					const primaryBarTraces = options.allTraces.filter(
						(t) => (!t.yaxis || t.yaxis === 'y') && (t.type === 'bar' || (!t.type))
					);
					const secondaryBarTraces = options.allTraces.filter(
						(t) => t.yaxis === 'y2' && (t.type === 'bar' || (!t.type))
					);

					if (primaryBarTraces.length > 0 && secondaryBarTraces.length > 0) {
						const totalBarTraces = primaryBarTraces.length + secondaryBarTraces.length;
						const barWidth = 0.8 / totalBarTraces;

						if (isSecondaryAxis) {
							const secondaryIndex = secondaryBarTraces.findIndex((t) => t === trace);
							trace.width = barWidth;
							trace.offset = barWidth * (primaryBarTraces.length + secondaryIndex) - 0.4 + barWidth / 2;
						} else {
							const primaryIndex = primaryBarTraces.findIndex((t) => t === trace);
							trace.width = barWidth;
							trace.offset = barWidth * primaryIndex - 0.4 + barWidth / 2;
						}
					}
				}
				return trace;
			},
			configureLayout: (baseLayout: any, userLayout: any) => createStandardLayout(baseLayout, userLayout)
		},
		histogram: {
			configureTrace: (trace: any, index: number) => {
				// Validate data first
				const hasValidX = trace.x && Array.isArray(trace.x) && trace.x.length > 0;
				const hasValidY = trace.y && Array.isArray(trace.y) && trace.y.length > 0;
				if (!hasValidX && !hasValidY) return null;

				// Set trace type
				if (!trace.type) trace.type = 'histogram';

				// Apply colors and styling
				const color = colorPalette[index % colorPalette.length];
				if (!trace.marker) trace.marker = {};
				
				if (!trace.marker.color) trace.marker.color = color;
				trace.marker.opacity = 1;
				trace.opacity = 1;

				// Add feint border lines around histogram bars
				if (!trace.marker.line) {
					trace.marker.line = {
						color: 'rgba(71, 85, 105, 0.4)',
						width: 0.5
					};
				}

				// Clean up empty arrays
				if (trace.y && Array.isArray(trace.y) && trace.y.length === 0) {
					delete trace.y;
				}
				if (trace.z && Array.isArray(trace.z) && trace.z.length === 0) {
					delete trace.z;
				}

				// Configure automatic binning
				if (!trace.autobinx && !trace.xbins && trace.x && trace.x.length > 0) {
					trace.autobinx = true;
				}
				if (!trace.autobiny && !trace.ybins && trace.y && trace.y.length > 0) {
					trace.autobiny = true;
				}

				// Set default number of bins
				if (trace.x && trace.x.length > 0 && !trace.nbinsx && !trace.xbins) {
					trace.nbinsx = Math.min(30, Math.max(10, Math.floor(Math.sqrt(trace.x.length))));
				}
				if (trace.y && trace.y.length > 0 && !trace.nbinsy && !trace.ybins) {
					trace.nbinsy = Math.min(30, Math.max(10, Math.floor(Math.sqrt(trace.y.length))));
				}

				return trace;
			},
			configureLayout: (baseLayout: any, userLayout: any) => ({
				...createStandardLayout(baseLayout, userLayout),
				barmode: 'group' as const
			})
		},
		heatmap: {
			configureTrace: (trace: any, index: number) => {
				// Set trace type
				if (!trace.type) trace.type = 'heatmap';

				// Apply heatmap-specific styling
				trace.colorscale = [
					[0, '#d32f2f'], // Dark red for most negative
					[0.25, '#f44336'], // Medium red
					[0.5, '#424242'], // Dark neutral/gray
					[0.75, '#4caf50'], // Medium green
					[1, '#2e7d32'] // Dark green for most positive
				];
				trace.zmid = 0;

				// Configure colorbar positioning
				if (!trace.colorbar) {
					trace.colorbar = {
						x: -0.2,
						xanchor: 'left',
						thickness: 12,
						len: 0.8,
						xpad: 10
					};
				}

				return trace;
			},
			configureLayout: (baseLayout: any, userLayout: any) => createStandardLayout(baseLayout, userLayout)
		}
	};

	// Add a function to format field names in hovertemplate
	function formatFieldName(field: string): string {
		return field.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
	}

	// Helper function to properly capitalize axis titles
	function capitalizeAxisTitle(title: string): string {
		if (!title || typeof title !== 'string') return title;
		
		return title
			.replace(/_/g, ' ') // Replace underscores with spaces
			.split(' ') // Split into words
			.map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase()) // Capitalize each word
			.join(' '); // Join back together
	}

	// Helper function to calculate padded range for axes
	function calculatePaddedRange(data: any[], axis: 'x' | 'y', paddingPercent: number = 0.15): [number, number] | null {
		if (!data || data.length === 0) return null;

		let allValues: number[] = [];
		
		// Collect all values from all traces
		data.forEach(trace => {
			const values = axis === 'x' ? trace.x : trace.y;
			if (values && Array.isArray(values)) {
				const numericValues = values.filter(v => typeof v === 'number' && !isNaN(v));
				allValues.push(...numericValues);
			}
		});

		if (allValues.length === 0) return null;

		const min = Math.min(...allValues);
		const max = Math.max(...allValues);
		const range = max - min;
		
		// If range is 0 (all values are the same), add some default padding
		const padding = range === 0 ? Math.abs(min) * 0.1 || 1 : range * paddingPercent;
		
		return [min - padding, max + padding];
	}

	function processTraceData(trace: any, index: number): any {
		const processedTrace = { ...trace };

		// Format field names in hovertemplate for user-friendly tooltips
		if (processedTrace.hovertemplate) {
			processedTrace.hovertemplate = processedTrace.hovertemplate.replace(/([a-zA-Z_]+)=/g, (match: string, p1: string) => `${formatFieldName(p1)}=`);
		}

		// Add hover styling for trace names
		processedTrace.hoverlabel = {
			...defaultHoverLabel,
			...processedTrace.hoverlabel // Allow override if specified
		};

		// Determine which chart configuration to use - prioritize individual trace type over overall chart_type
		const traceType = processedTrace.type || plotData.chart_type;
		const chartConfig = chartTypeConfigs[traceType as keyof typeof chartTypeConfigs];
		
		if (chartConfig) {
			// Pass all traces for bar chart positioning logic (only needed for bar charts)
			const options = plotData.chart_type === 'bar' ? { allTraces: plotData.data } : undefined;
			const configuredTrace = chartConfig.configureTrace(processedTrace, index, options);
			
			// Return null if trace was filtered out (e.g., invalid histogram data)
			if (configuredTrace === null) {
				return null;
			}
			
			return configuredTrace;
		}

		// Fallback for unknown chart types (preserve original behavior)
		return processedTrace;
	}

	// Process trace data reactively
	$: processedData = plotData.data
		.map((trace: any, index: number) => processTraceData(trace, index))
		.filter((trace: any) => trace !== null);

	// Check if any traces use secondary y-axis
	$: hasSecondaryYAxis = plotData.data.some((trace: any) => trace.yaxis === 'y2');

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
			title: plotData.title ? '' : plotData.layout?.title || ''
		};

		if (hasSecondaryYAxis) {
			// Check if we have any bar traces for proper barmode setting
			const hasBarTraces = plotData.data.some(
				(trace: any) => trace.type === 'bar' || (!trace.type && plotData.chart_type === 'bar')
			);

			// Calculate padded ranges for dual y-axis
			const xRange = userLayoutWithoutDimensions.xaxis?.range || calculatePaddedRange(plotData.data, 'x', 0.02);
			const primaryYTraces = plotData.data.filter((trace: any) => !trace.yaxis || trace.yaxis === 'y');
			const secondaryYTraces = plotData.data.filter((trace: any) => trace.yaxis === 'y2');
			const primaryYRange = userLayoutWithoutDimensions.yaxis?.range || calculatePaddedRange(primaryYTraces, 'y', 0.10);
			const secondaryYRange = userLayoutWithoutDimensions.yaxis2?.range || calculatePaddedRange(secondaryYTraces, 'y', 0.10);

			// Configure dual y-axis layout
			layout = {
				...baseLayout,
				// Adjust margins for dual y-axis
				margin: { l: 60, r: 80, t: 10, b: 30, autoexpand: true },
				// For histograms, use 'group' mode to avoid transparency; for bar charts, use overlay for positioning
				barmode: plotData.chart_type === 'histogram' ? ('group' as const) : (hasBarTraces ? ('overlay' as const) : userLayoutWithoutDimensions.barmode),
				// X-axis with feint gridlines
				xaxis: {
					...baseLayout.xaxis,
					...userLayoutWithoutDimensions.xaxis,
					gridcolor: 'rgba(255, 255, 255, 0.08)',
					linecolor: 'rgba(255, 255, 255, 0.3)',
					...(xRange && { range: xRange }),
				},
				// Primary y-axis (left side)
				yaxis: {
					...defaultLayout.yaxis,
					...userLayoutWithoutDimensions.yaxis,
					side: 'left' as const,
					gridcolor: 'rgba(255, 255, 255, 0.08)',
					...(primaryYRange && { range: primaryYRange }),
				},
				// Secondary y-axis (right side)
				yaxis2: {
					...defaultLayout.yaxis,
					...userLayoutWithoutDimensions.yaxis2,
					side: 'right' as const,
					overlaying: 'y' as const,
					...(secondaryYRange && { range: secondaryYRange }),
					// Ensure grid lines don't overlap by disabling on secondary axis
					showgrid: false
				}
			};
		} else {
			// Single y-axis layout - use chart type configuration if available
			const chartConfig = chartTypeConfigs[plotData.chart_type];
			if (chartConfig) {
				layout = chartConfig.configureLayout(baseLayout, userLayoutWithoutDimensions);
			} else {
				// Fallback for unknown chart types
				layout = createStandardLayout(baseLayout, userLayoutWithoutDimensions);
			}
		}
	}
</script>

<div class="chunk-plot-container" bind:this={chunkContainer}>
	{#if plotData.title}
		<div class="plot-title">
			{#if plotData.titleIcon}
				<div class="plot-title-with-icon">
					<img 
						src={plotData.titleIcon.startsWith('data:') ? plotData.titleIcon : `data:image/png;base64,${plotData.titleIcon}`}
						alt="Ticker icon"
						class="plot-ticker-icon"
					/>
					<span class="plot-title-text">{plotData.title}</span>
				</div>
			{:else}
				{plotData.title}
			{/if}
		</div>
	{/if}

	<div class="plot-container">
		<Plot data={processedData} {layout} config={defaultConfig} fillParent={true} debounce={250} />
	</div>
	
	<div class="plot-actions">
		<button 
			class="copy-image-btn glass glass--small glass--responsive {copyImageFeedback ? 'copied' : ''}"
			on:click={copyPlotImage}
			title="Copy plot as image"
		>
			{#if copyImageFeedback}
				<svg viewBox="0 0 24 24" width="14" height="14">
					<path d="M9,20.42L2.79,14.21L5.62,11.38L9,14.77L18.88,4.88L21.71,7.71L9,20.42Z" fill="currentColor"/>
				</svg>
			{:else}
				<svg viewBox="0 0 24 24" width="14" height="14">
					<path d="M19,21H8V7H19M19,5H8A2,2 0 0,0 6,7V21A2,2 0 0,0 8,23H19A2,2 0 0,0 21,21V7A2,2 0 0,0 19,5M16,1H4A2,2 0 0,0 2,3V17H4V3H16V1Z" fill="currentColor"/>
				</svg>
			{/if}
		</button>
	</div>
</div>

<style>
	.chunk-plot-container {
		margin-bottom: 1rem;
		position: relative;
	}

	.plot-actions {
		display: flex;
		justify-content: flex-end;
		align-items: center;
		margin-top: 8px;
		margin-right: 8px;
		opacity: 0;
		transition: opacity 0.2s ease;
	}

	.chunk-plot-container:hover .plot-actions {
		opacity: 1;
	}

	.plot-title {
		font-family: Geist, Inter, system-ui, sans-serif;
		font-size: 1.2rem;
		font-weight: 600;
		color: var(--text-primary, #fff);
		text-align: center;
	}

	.plot-title-with-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.5rem;
		width: 100%;
	}

	.plot-ticker-icon {
		width: 28px;
		height: 28px;
		border-radius: 6px;
		object-fit: cover;
		flex-shrink: 0;
	}

	.plot-title-text {
		text-align: center;
		flex-grow: 0;
		flex-shrink: 1;
		font-size: 1.2rem;
	}

	.plot-container {
		min-height: 450px;
		height: 450px;
		width: 100%;
		overflow: hidden;
	}

	/* Watermark styles - only used in cloned/copied version */
	:global(.watermark-offscreen) {
		text-align: right;
		font-family: Geist, Inter, system-ui, sans-serif;
		font-size: 13px;
		color: rgb(255 255 255 / 90%);
		padding-bottom: 4px;
		padding-right: 8px;
		margin-top: 8px;
		border-radius: 4px;
	}

	:global(.watermark-offscreen .watermark-brand) {
		font-size: 16px;
		font-weight: 600;
		color: #fff;
	}

	.copy-image-btn {
		/* Glass effect provided by global .glass classes */
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0.3rem;
		color: #fff;
		cursor: pointer;
		transition: all 0.2s ease;
		font-size: 0.75rem;
	}

	.copy-image-btn:hover {
		--glass-bg: rgb(255 255 255 / 10%);
		--glass-border: #fff;

		color: var(--text-primary, #fff);
		border-color: #fff;
	}

	.copy-image-btn.copied {
		--glass-bg: rgb(76 175 80 / 20%);
		--glass-border: #4caf50;

		color: #4caf50;
		animation: copySuccess 0.3s ease;
	}

	.copy-image-btn.copied:hover {
		--glass-bg: rgb(76 175 80 / 30%);
		--glass-border: #4caf50;

		color: #4caf50;
	}

	.copy-image-btn svg {
		width: 0.75rem;
		height: 0.75rem;
	}

	@keyframes copySuccess {
		0% {
			transform: scale(1);
		}

		50% {
			transform: scale(1.1);
		}

		100% {
			transform: scale(1);
		}
	}

	/* Responsive adjustments */
	@media (width <= 768px) {
		.plot-container {
			min-height: 380px;
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
		background: rgb(15 23 42 / 80%) !important;
		border: 1px solid rgb(71 85 105 / 30%) !important;
		border-radius: 4px !important;
	}

	:global(.plot-container .plotly .modebar-btn) {
		color: #cbd5e1 !important;
	}

	:global(.plot-container .plotly .modebar-btn:hover) {
		background: rgb(71 85 105 / 30%) !important;
		color: #e2e8f0 !important;
	}

	/* Fix hover label trace name background */
	:global(.plot-container .plotly .hoverlayer .hovertext) {
		background: #1e293b !important;
		border: 1px solid #475569 !important;
		color: #fff !important;
		font-family: Geist, Inter, system-ui, sans-serif !important;
		font-size: 11px !important;
		border-radius: 2px !important;
		padding: 4px 6px !important;
		line-height: 1.2 !important;
	}

	:global(.plot-container .plotly .hoverlayer .hovertext rect) {
		fill: #1e293b !important;
		fill-opacity: 1 !important;
		stroke: #475569 !important;
		stroke-width: 1 !important;
	}

	:global(.plot-container .plotly .hoverlayer .hovertext path) {
		fill: #1e293b !important;
		stroke: #475569 !important;
		stroke-width: 1 !important;
	}

	/* Make hover labels more compact */
	:global(.plot-container .plotly .hoverlayer .hovertext text) {
		font-size: 11px !important;
		font-family: Geist, Inter, system-ui, sans-serif !important;
	}

	:global(.plot-container .plotly .hoverlayer .hovertext tspan) {
		font-size: 11px !important;
	}


</style>
