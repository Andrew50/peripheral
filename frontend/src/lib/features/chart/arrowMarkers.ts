import type {
	ICustomSeriesPaneRenderer,
	ICustomSeriesPaneView,
	CustomData,
	CustomSeriesOptions,
	PaneRendererCustomData,
	CustomSeriesWhitespaceData,
} from 'lightweight-charts';
import type { Time, CustomSeriesPricePlotValues } from 'lightweight-charts';
import { ColorType } from 'lightweight-charts';

// Define your custom data type for arrow markers.
export interface ArrowMarker extends CustomData<Time> {
	time: Time;               // timestamp (must be in the chart's time format)
	entries?: Array<{
		price: number;
		isLong: boolean;  // true for long entries, false for short entries
	}>;
	exits?: Array<{
		price: number;
		isLong: boolean;  // true for long exits, false for short exits
	}>;
}

// Helper functions to draw the arrows.
function drawArrowUp(ctx: CanvasRenderingContext2D, x: number, y: number, size: number) {
	// Draw white outline
	ctx.strokeStyle = 'white';
	ctx.lineWidth = 1;
	ctx.beginPath();
	ctx.moveTo(x, y - size);
	ctx.lineTo(x - size, y + size);
	ctx.lineTo(x + size, y + size);
	ctx.closePath();
	ctx.stroke();

	// Fill with color
	ctx.fill();
}

function drawArrowDown(ctx: CanvasRenderingContext2D, x: number, y: number, size: number) {
	// Draw white outline
	ctx.strokeStyle = 'white';
	ctx.lineWidth = 1;
	ctx.beginPath();
	ctx.moveTo(x, y + size);
	ctx.lineTo(x - size, y - size);
	ctx.lineTo(x + size, y - size);
	ctx.closePath();
	ctx.stroke();

	// Fill with color
	ctx.fill();
}

// Custom series view for arrow markers.
export class ArrowMarkersPaneView implements ICustomSeriesPaneView<Time, ArrowMarker, CustomSeriesOptions> {
	private markers: ArrowMarker[] = [];
	private options: CustomSeriesOptions = this.defaultOptions();

	renderer(): ICustomSeriesPaneRenderer {
		return {
			draw: (target, priceToCoordinate, visibleRange) => {
				target.useMediaCoordinateSpace(({ context, mediaSize }) => {
					const { width, height } = mediaSize;
					
					if (this.markers.length === 0) {
						return;
					}

					// Draw each marker
					for (const marker of this.markers) {
						const x = marker.x;
						
						// Draw entry arrows
						if (marker.originalData.entries?.length) {
							marker.originalData.entries.forEach(entry => {
								// For long entries: green up arrow
								// For short entries: red down arrow
								context.fillStyle = entry.isLong ? 'green' : 'red';
								const y = priceToCoordinate(entry.price);
								if (entry.isLong) {
									drawArrowUp(context, x, y, 7);
								} else {
									drawArrowDown(context, x, y, 7);
								}
							});
						}

						// Draw exit arrows
						if (marker.originalData.exits?.length) {
							marker.originalData.exits.forEach(exit => {
								// For long exits: red down arrow
								// For short exits: green up arrow
								context.fillStyle = exit.isLong ? 'red' : 'green';
								const y = priceToCoordinate(exit.price);
								if (exit.isLong) {
									drawArrowDown(context, x, y, 7);
								} else {
									drawArrowUp(context, x, y, 7);
								}
							});
						}
					}
				});
			}
		};
	}

	// Converts a time value to an x-coordinate using the visible range.
	timeToX(
		markerTime: Time,
		data: ArrowMarker[],
		visibleRange: { from: number; to: number },
		width: number
	): number {
		console.log(`timeToX called with markerTime: ${markerTime}, visibleRange: ${JSON.stringify(visibleRange)}, width: ${width}`);
		const markerIndex = data.findIndex(d => d.time === markerTime);
		console.log(`Found marker index: ${markerIndex} for markerTime: ${markerTime}`);
		if (markerIndex < 0) {
			console.error("Marker time not found in data. Data:", data);
			return -100; // Sentinel value
		}

		const { from, to } = visibleRange; // these are indexes
		const range = to - from;
		if (range <= 0) {
			console.error("Invalid visible range. Range <= 0", visibleRange);
			return -100;
		}
		const relativePos = (markerIndex - from) / range;
		const xPos = relativePos * width;
		console.log(`Calculated relativePos: ${relativePos}, resulting x position: ${xPos}`);
		return xPos;
	}

	// Called whenever new data or options are provided.
	update(data: PaneRendererCustomData<Time, ArrowMarker>, seriesOptions: CustomSeriesOptions): void {
		//console.log("ArrowMarkersPaneView update called with data:", data, "and seriesOptions:", seriesOptions);
		this.markers = data.bars; // Assumes your data is in the "bars" property.
		this.options = seriesOptions;
	}

	// Update price value builder to handle new structure
	priceValueBuilder(plotRow: ArrowMarker): CustomSeriesPricePlotValues {
		const prices: number[] = [];
		if (plotRow.entries) prices.push(...plotRow.entries.map(e => e.price));
		if (plotRow.exits) prices.push(...plotRow.exits.map(e => e.price));
		return prices;
	}

	// No marker is considered whitespace in this example.
	isWhitespace(data: ArrowMarker | CustomSeriesWhitespaceData<Time>): data is CustomSeriesWhitespaceData<Time> {
		return false;
	}

	// Default options.
	defaultOptions(): CustomSeriesOptions {
		const defaultOpts = { color: 'green' };
		console.log("ArrowMarkersPaneView defaultOptions called, returning:", defaultOpts);
		return defaultOpts;
	}

	// Cleanup, if necessary.
	destroy(): void {
		console.log("ArrowMarkersPaneView destroy called");
	}
}
