import type {
	ICustomSeriesPaneRenderer,
	ICustomSeriesPaneView,
	CustomData,
	CustomSeriesOptions,
	PaneRendererCustomData,
	CustomSeriesWhitespaceData
} from 'lightweight-charts';
import type { Time, CustomSeriesPricePlotValues } from 'lightweight-charts';
import { ColorType } from 'lightweight-charts';

// Define your custom data type for event markers.
export interface EventMarker extends CustomData<Time> {
	time: Time; // timestamp (must be in the chart's time format)
	events: Array<{
		type: 'filing'; // We can add more event types later if needed
		title: string; // e.g., "10-K", "8-K", etc.
		url: string; // Add URL for clicking through to filing
	}>;
}

interface MarkerPosition {
	x: number;
	y: number;
	events: EventMarker['events'];
}

// Helper function to draw event markers
function drawEventMarker(
	ctx: CanvasRenderingContext2D,
	x: number,
	y: number,
	size: number,
	count: number
) {
	// Draw a purple circle for SEC filings
	ctx.fillStyle = '#9C27B0'; // Purple color
	ctx.strokeStyle = 'white';
	ctx.lineWidth = 1;

	ctx.beginPath();
	ctx.arc(x, y, size * 1.5, 0, 2 * Math.PI); // 50% bigger
	ctx.fill();
	ctx.stroke();

	// If there are multiple events, add a count
	if (count > 1) {
		ctx.fillStyle = 'white';
		ctx.font = '11px sans-serif'; // Slightly larger font for the count
		ctx.textAlign = 'center';
		ctx.textBaseline = 'middle';
		ctx.fillText(count.toString(), x, y);
	}
}

// Custom series view for event markers.
export class EventMarkersPaneView
	implements ICustomSeriesPaneView<Time, EventMarker, CustomSeriesOptions>
{
	private markers: EventMarker[] = [];
	private markerPositions: MarkerPosition[] = [];
	private options: CustomSeriesOptions = this.defaultOptions();
	private visibleRange: { from: number; to: number } = { from: 0, to: 0 };
	private clickCallback?: (events: EventMarker['events'], x: number, y: number) => void;

	// Add method to set click handler
	public setClickCallback(callback: (events: EventMarker['events'], x: number, y: number) => void) {
		this.clickCallback = callback;
	}

	// Add method to handle clicks
	public handleClick(x: number, y: number) {
		const clickRadius = 10; // Area around marker that's clickable

		for (const marker of this.markerPositions) {
			const distance = Math.sqrt(Math.pow(marker.x - x, 2) + Math.pow(marker.y - y, 2));

			if (distance <= clickRadius) {
				this.clickCallback?.(marker.events, marker.x, marker.y);
				return true;
			}
		}
		return false;
	}

	renderer(): ICustomSeriesPaneRenderer {
		return {
			draw: (target, priceToCoordinate, visibleRange) => {
				target.useMediaCoordinateSpace(({ context, mediaSize }) => {
					const { width, height } = mediaSize;

					if (this.markers.length === 0) {
						return;
					}

					// Clear previous positions
					this.markerPositions = [];

					// Only iterate over visible markers
					for (
						let i = Math.floor(this.visibleRange.from);
						i < Math.ceil(this.visibleRange.to);
						i++
					) {
						if (i < 0 || i >= this.markers.length) continue;

						const marker = this.markers[i];
						const x = marker.x;
						if (x === null) {
							continue;
						}

						if (marker.originalData.events?.length) {
							const y = height - 20; // 20px from bottom
							drawEventMarker(context, x, y, 5, marker.originalData.events.length);

							// Store marker position for click detection
							this.markerPositions.push({
								x,
								y,
								events: marker.originalData.events
							});
						}
					}
				});
			}
		};
	}

	update(
		data: PaneRendererCustomData<Time, EventMarker>,
		seriesOptions: CustomSeriesOptions
	): void {
		this.markers = data.bars;
		this.options = seriesOptions;
		this.visibleRange = data.visibleRange;
	}

	priceValueBuilder(plotRow: EventMarker): CustomSeriesPricePlotValues {
		const prices: number[] = [];
		if (plotRow.events) prices.push(...plotRow.events.map((e) => e.price));
		return prices;
	}

	isWhitespace(
		data: EventMarker | CustomSeriesWhitespaceData<Time>
	): data is CustomSeriesWhitespaceData<Time> {
		return false;
	}

	defaultOptions(): CustomSeriesOptions {
		return {
			color: '#9C27B0',
			lastValueVisible: false, // Don't show the last value label
			priceLineVisible: false, // Don't show the price line
			baseLineVisible: false // Don't show the base line
		};
	}

	destroy(): void {}
}
