import type {
	ICustomSeriesPaneRenderer,
	ICustomSeriesPaneView,
	CustomData,
	CustomSeriesOptions,
	PaneRendererCustomData,
	CustomSeriesWhitespaceData,
	PriceLineSource
} from 'lightweight-charts';
import type { Time, CustomSeriesPricePlotValues } from 'lightweight-charts';

// Define your custom data type for event markers.
export interface EventMarker extends CustomData<Time> {
	time: Time; // timestamp (must be in the chart's time format)
	events: Array<{
		type: string; // 'sec_filing', 'split', 'dividend'
		title: string;
		url?: string; // URL for clicking through (optional)
		value?: string; // Additional data like split ratio or dividend amount
		exDate?: string;
		payoutDate?: string;
	}>;
	// Add missing properties
	x?: number | null;
	originalData?: {
		events?: Array<{
			type: string;
			title: string;
			url?: string;
			value?: string;
			exDate?: string;
			payoutDate?: string;
		}>;
	};
}

interface MarkerPosition {
	x: number;
	y: number;
	events: EventMarker['events'];
	radius: number;
	time: number; // Timestamp of the marker (EST seconds)
}

// Helper function to draw event markers with different colors based on type
function drawEventMarker(
	ctx: CanvasRenderingContext2D,
	x: number,
	y: number,
	size: number,
	events: EventMarker['events'],
	isHovered: boolean = false
) {
	// Get the first event to determine the color (all events at same timestamp should be same type)
	const eventType = events[0]?.type || 'sec_filing';

	// Color mapping for different event types
	const colorMap: Record<string, string> = {
		sec_filing: '#9C27B0', // Purple for filings
		split: '#FFD700', // Yellow for splits
		dividend: '#2196F3' // Blue for dividends
	};

	// Get the color based on event type or default to filing color (purple)
	const color = colorMap[eventType] || colorMap['sec_filing'];

	// Increase size if hovered
	const markerSize = isHovered ? size * 2 : size * 1.5;

	// Draw the circle with the appropriate color
	ctx.fillStyle = color;
	ctx.strokeStyle = isHovered ? 'white' : 'rgba(255, 255, 255, 0.7)';
	ctx.lineWidth = isHovered ? 2 : 1;

	// Add a subtle glow effect when hovered
	if (isHovered) {
		ctx.shadowColor = color;
		ctx.shadowBlur = 8;
	}

	ctx.beginPath();
	ctx.arc(x, y, markerSize, 0, 2 * Math.PI);
	ctx.fill();
	ctx.stroke();

	// Reset shadow
	ctx.shadowColor = 'transparent';
	ctx.shadowBlur = 0;

	// If there are multiple events, add a count
	if (events.length > 1) {
		ctx.fillStyle = 'white';
		ctx.font = isHovered ? '12px sans-serif' : '11px sans-serif';
		ctx.textAlign = 'center';
		ctx.textBaseline = 'middle';
		ctx.fillText(events.length.toString(), x, y);
	} else {
		// For single events, add a letter indicator based on type
		if (eventType === 'dividend') {
			ctx.fillStyle = 'white';
			ctx.font = isHovered ? '12px sans-serif' : '10px sans-serif';
			ctx.textAlign = 'center';
			ctx.textBaseline = 'middle';
			ctx.fillText('D', x, y);
		} else if (eventType === 'split') {
			ctx.fillStyle = 'white';
			ctx.font = isHovered ? '12px sans-serif' : '10px sans-serif';
			ctx.textAlign = 'center';
			ctx.textBaseline = 'middle';
			ctx.fillText('S', x, y);
		}
	}
}

// Custom series view for event markers.
export class EventMarkersPaneView
	implements ICustomSeriesPaneView<Time, EventMarker, CustomSeriesOptions> {
	private markers: EventMarker[] = [];
	private markerPositions: MarkerPosition[] = [];
	private options: CustomSeriesOptions = this.defaultOptions();
	private visibleRange: { from: number; to: number } = { from: 0, to: 0 };
	private clickCallback?: (
		events: EventMarker['events'],
		x: number,
		y: number,
		time: number
	) => void;
	private hoverCallback?: (events: EventMarker['events'] | null, x: number, y: number) => void;
	private hoveredMarkerIndex: number = -1;
	private lastMousePosition: { x: number; y: number } = { x: 0, y: 0 };

	// Add method to set click handler
	public setClickCallback(
		callback: (events: EventMarker['events'], x: number, y: number, time: number) => void
	) {
		this.clickCallback = callback;
	}

	// Add method to set hover handler
	public setHoverCallback(
		callback: (events: EventMarker['events'] | null, x: number, y: number) => void
	) {
		this.hoverCallback = callback;
	}

	// Method to handle mouse move for hover detection
	public handleMouseMove(x: number, y: number) {
		this.lastMousePosition = { x, y };

		// Find the closest marker within detection radius
		let closestMarker = -1;
		let closestDistance = Number.POSITIVE_INFINITY;
		const hoverRadius = 15; // Area around marker that's considered hoverable

		for (let i = 0; i < this.markerPositions.length; i++) {
			const marker = this.markerPositions[i];
			const distance = Math.sqrt(Math.pow(marker.x - x, 2) + Math.pow(marker.y - y, 2));

			if (distance <= hoverRadius && distance < closestDistance) {
				closestDistance = distance;
				closestMarker = i;
			}
		}

		// Only trigger callback if hover state changed
		if (closestMarker !== this.hoveredMarkerIndex) {
			this.hoveredMarkerIndex = closestMarker;

			if (closestMarker >= 0) {
				const marker = this.markerPositions[closestMarker];
				this.hoverCallback?.(marker.events, marker.x, marker.y);
			} else {
				this.hoverCallback?.(null, x, y);
			}

			return true; // State changed, request redraw
		}

		return false; // No change
	}

	// Method to clear hover state when mouse leaves chart
	public clearHover() {
		if (this.hoveredMarkerIndex !== -1) {
			this.hoveredMarkerIndex = -1;
			this.hoverCallback?.(null, 0, 0);
			return true; // State changed, request redraw
		}
		return false; // No change
	}

	// Improved click detection that prioritizes closest marker
	public handleClick(x: number, y: number) {
		// Use smaller click radius for better precision
		const clickRadius = 15;

		// Find the closest marker within detection radius
		let closestMarker = -1;
		let closestDistance = Number.POSITIVE_INFINITY;

		for (let i = 0; i < this.markerPositions.length; i++) {
			const marker = this.markerPositions[i];
			const distance = Math.sqrt(Math.pow(marker.x - x, 2) + Math.pow(marker.y - y, 2));

			if (distance <= clickRadius && distance < closestDistance) {
				closestDistance = distance;
				closestMarker = i;
			}
		}

		// If we found a marker to click
		if (closestMarker >= 0) {
			const marker = this.markerPositions[closestMarker];
			this.clickCallback?.(marker.events, marker.x, marker.y, marker.time);
			return true;
		}

		return false;
	}

	renderer(): ICustomSeriesPaneRenderer {
		return {
			draw: (target) => {
				target.useMediaCoordinateSpace(({ context, mediaSize }) => {
					const { height } = mediaSize;

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
						if (x === null || x === undefined) {
							continue;
						}

						if (marker.originalData?.events?.length) {
							const y = height - 20; // 20px from bottom
							const markerSize = 5;

							// Store marker position for click/hover detection BEFORE drawing
							// so we can use the index for hover detection
							const positionIndex = this.markerPositions.length;
							this.markerPositions.push({
								x: x as number,
								y,
								events: marker.originalData.events,
								radius: markerSize * 1.5, // Store the radius for accurate detection
								time:
									typeof marker.time === 'number' ? marker.time : (marker.time as unknown as number)
							});

							// Check if THIS marker is the one being hovered
							const isHovered = positionIndex === this.hoveredMarkerIndex;

							drawEventMarker(
								context,
								x as number,
								y,
								markerSize,
								marker.originalData.events,
								isHovered
							);
						}
					}

					// Check if mouse is over any marker and update hover state
					if (this.lastMousePosition.x && this.lastMousePosition.y) {
						this.handleMouseMove(this.lastMousePosition.x, this.lastMousePosition.y);
					}
				});
			}
		};
	}

	update(
		data: PaneRendererCustomData<Time, EventMarker>,
		seriesOptions: CustomSeriesOptions
	): void {
		this.markers = [...data.bars] as unknown as EventMarker[]; // Use type assertion with unknown as intermediate step
		this.options = seriesOptions;
		this.visibleRange = data.visibleRange || { from: 0, to: 0 }; // Handle null case
	}

	priceValueBuilder(): CustomSeriesPricePlotValues {
		const prices: number[] = [];
		return prices; // Return empty array as we're not showing price-related data
	}

	isWhitespace(
		data: EventMarker | CustomSeriesWhitespaceData<Time>
	): data is CustomSeriesWhitespaceData<Time> {
		return false;
	}

	defaultOptions(): CustomSeriesOptions {
		return {
			color: '#9C27B0',
			lastValueVisible: false,
			title: 'Event Markers',
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

	destroy(): void { }
}
