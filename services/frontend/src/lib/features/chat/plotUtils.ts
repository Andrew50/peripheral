import type { PlotData } from './interface';

/**
 * Type guard to check if content is PlotData
 */
export function isPlotData(content: any): content is PlotData {
	return (
		content &&
		typeof content === 'object' &&
		typeof content.chart_type === 'string' &&
		['line', 'bar', 'scatter', 'histogram', 'heatmap'].includes(content.chart_type) &&
		Array.isArray(content.data)
	);
}

/**
 * Get plot data safely with validation
 */
export function getPlotData(content: any): PlotData | null {
	if (isPlotData(content)) {
		return content;
	}
	return null;
}

/**
 * Validate plot data structure
 */
export function validatePlotData(plotData: PlotData): string[] {
	const errors: string[] = [];

	// Check chart type
	if (!['line', 'bar', 'scatter', 'histogram', 'heatmap'].includes(plotData.chart_type)) {
		errors.push(`Invalid chart type: ${plotData.chart_type}`);
	}

	// Check data array
	if (!Array.isArray(plotData.data) || plotData.data.length === 0) {
		errors.push('Plot data must contain at least one trace');
	}

	// Validate each trace
	plotData.data.forEach((trace, index) => {
		if (!trace || typeof trace !== 'object') {
			errors.push(`Trace ${index} is not a valid object`);
			return;
		}

		// Check for required data based on chart type
		switch (plotData.chart_type) {
			case 'line':
			case 'scatter':
				if (!trace.x && !trace.y) {
					errors.push(`Trace ${index}: Line and scatter plots require x or y data`);
				}
				break;
			case 'bar':
				if (!trace.x && !trace.y) {
					errors.push(`Trace ${index}: Bar plots require x or y data`);
				}
				break;
			case 'histogram':
				if (!trace.x && !trace.y) {
					errors.push(`Trace ${index}: Histogram plots require x or y data`);
				}
				break;
			case 'heatmap':
				if (!trace.z || !Array.isArray(trace.z)) {
					errors.push(`Trace ${index}: Heatmap plots require z data as a 2D array`);
				}
				break;
		}
	});

	return errors;
}

/**
 * Extract text content from plot data for copying
 */
export function plotDataToText(plotData: PlotData): string {
	let text = '';
	
	if (plotData.title) {
		text += `${plotData.title}\n\n`;
	}

	text += `Chart Type: ${plotData.chart_type}\n\n`;

	plotData.data.forEach((trace, index) => {
		if (trace.name) {
			text += `${trace.name}:\n`;
		} else {
			text += `Trace ${index + 1}:\n`;
		}

		// Extract data based on chart type
		if (trace.x && trace.y) {
			// For x,y plots
			const length = Math.min(trace.x.length, trace.y.length);
			for (let i = 0; i < Math.min(length, 10); i++) { // Limit to first 10 points
				text += `  ${trace.x[i]}, ${trace.y[i]}\n`;
			}
			if (length > 10) {
				text += `  ... and ${length - 10} more points\n`;
			}
		} else if (trace.z) {
			// For heatmaps
			text += `  Heatmap data: ${trace.z.length} rows x ${trace.z[0]?.length || 0} columns\n`;
		} else if (trace.x) {
			// For histograms or single-axis data
			const length = trace.x.length;
			for (let i = 0; i < Math.min(length, 10); i++) {
				text += `  ${trace.x[i]}\n`;
			}
			if (length > 10) {
				text += `  ... and ${length - 10} more values\n`;
			}
		} else if (trace.y) {
			const length = trace.y.length;
			for (let i = 0; i < Math.min(length, 10); i++) {
				text += `  ${trace.y[i]}\n`;
			}
			if (length > 10) {
				text += `  ... and ${length - 10} more values\n`;
			}
		}
		text += '\n';
	});

	return text.trim();
}

/**
 * Generate a unique key for a plot based on its content
 */
export function generatePlotKey(messageId: string, chunkIndex: number): string {
	return `${messageId}-plot-${chunkIndex}`;
}

/**
 * Default theme configuration for plots
 */
export const defaultPlotTheme = {
	colorPalette: [
		'#60a5fa', // blue-400
		'#34d399', // emerald-400
		'#f87171', // red-400
		'#fbbf24', // amber-400
		'#c084fc', // purple-400
		'#fb7185', // rose-400
		'#38bdf8', // sky-400
		'#4ade80'  // green-400
	],
	backgroundColor: 'rgba(15, 23, 42, 0.8)', // slate-900
	gridColor: 'rgba(71, 85, 105, 0.3)', // slate-600
	textColor: '#e2e8f0', // text-slate-200
	axisTitleColor: '#e2e8f0',
	tickColor: '#cbd5e1' // text-slate-300
};