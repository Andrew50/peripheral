import type {
	ICustomSeriesPaneRenderer,
	ICustomSeriesPaneView,
	CustomData,
	CustomSeriesOptions,
	PaneRendererCustomData,
	CustomSeriesWhitespaceData,
	DeepPartial,
	SeriesOptionsCommon,
	PriceLineSource
} from 'lightweight-charts';
import type { Time, CustomSeriesPricePlotValues } from 'lightweight-charts';
import { ColorType } from 'lightweight-charts';

// Define your custom data type for arrow markers.
export interface ArrowMarker extends CustomData<Time> {
	time: Time; // timestamp (must be in the chart's time format)
	entries?: Array<{
		price: number;
		isLong: boolean; // true for long entries, false for short entries
	}>;
	exits?: Array<{
		price: number;
		isLong: boolean; // true for long exits, false for short exits
	}>;
	x?: number | null;
	originalData?: {
		entries?: Array<{
			price: number;
			isLong: boolean;
		}>;
		exits?: Array<{
			price: number;
			isLong: boolean;
		}>;
	};
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
export class ArrowMarkersPaneView
	implements ICustomSeriesPaneView<Time, ArrowMarker, CustomSeriesOptions>
{
	private markers: ArrowMarker[] = [];
	private options: CustomSeriesOptions = this.defaultOptions();
	private visibleRange: { from: number; to: number } = { from: 0, to: 0 };

	renderer(): ICustomSeriesPaneRenderer {
		return {
			draw: (target, priceToCoordinate, visibleRange) => {
				target.useMediaCoordinateSpace(({ context, mediaSize }) => {
					const { width, height } = mediaSize;

					if (this.markers.length === 0) {
						return;
					}

					// Only iterate over visible markers
					for (
						let i = Math.floor(this.visibleRange.from);
						i < Math.ceil(this.visibleRange.to);
						i++
					) {
						if (i < 0 || i >= this.markers.length) continue;

						const marker = this.markers[i];
						const x = marker.x;
						if (x === null || x === undefined) {
							continue;
						}

						// Draw entry arrows
						if (marker.originalData?.entries?.length) {
							marker.originalData.entries.forEach((entry) => {
								context.fillStyle = entry.isLong ? 'green' : 'red';
								const y = priceToCoordinate(entry.price);
								if (y === null) return;
								if (entry.isLong) {
									drawArrowUp(context, x as number, y, 6);
								} else {
									drawArrowDown(context, x as number, y, 6);
								}
							});
						}

						// Draw exit arrows
						if (marker.originalData?.exits?.length) {
							marker.originalData.exits.forEach((exit) => {
								context.fillStyle = exit.isLong ? 'red' : 'green';
								const y = priceToCoordinate(exit.price);
								if (y === null) return;
								if (exit.isLong) {
									drawArrowDown(context, x as number, y, 6);
								} else {
									drawArrowUp(context, x as number, y, 6);
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
		const markerIndex = data.findIndex((d) => d.time === markerTime);
		if (markerIndex < 0) {
			console.error('Marker time not found in data. Data:', data);
			return -100; // Sentinel value
		}

		const { from, to } = visibleRange; // these are indexes
		const range = to - from;
		if (range <= 0) {
			console.error('Invalid visible range. Range <= 0', visibleRange);
			return -100;
		}
		const relativePos = (markerIndex - from) / range;
		const xPos = relativePos * width;
		return xPos;
	}

	// Called whenever new data or options are provided.
	update(
		data: PaneRendererCustomData<Time, ArrowMarker>,
		seriesOptions: CustomSeriesOptions
	): void {
		// Use type assertion with unknown as intermediate step
		this.markers = [...data.bars] as unknown as ArrowMarker[];
		this.options = seriesOptions;
		this.visibleRange = data.visibleRange || { from: 0, to: 0 }; // Provide default if null
	}

	// Update price value builder to handle new structure
	priceValueBuilder(plotRow: ArrowMarker): CustomSeriesPricePlotValues {
		const prices: number[] = [];
		if (plotRow.entries) prices.push(...plotRow.entries.map((e) => e.price));
		if (plotRow.exits) prices.push(...plotRow.exits.map((e) => e.price));
		return prices;
	}

	// No marker is considered whitespace in this example.
	isWhitespace(
		data: ArrowMarker | CustomSeriesWhitespaceData<Time>
	): data is CustomSeriesWhitespaceData<Time> {
		return false;
	}

	// Default options.
	defaultOptions(): CustomSeriesOptions {
		return {
			color: '#FF5722',
			lastValueVisible: false,
			title: 'Arrow Markers',
			visible: true,
			priceLineVisible: false,
			priceLineSource: 'lastVisible' as unknown as PriceLineSource,
			priceLineWidth: 1,
			priceFormat: {
				type: 'price',
				precision: 2,
				minMove: 0.01
			}
		} as CustomSeriesOptions;
	}

	// Cleanup, if necessary.
	destroy(): void {}
}
