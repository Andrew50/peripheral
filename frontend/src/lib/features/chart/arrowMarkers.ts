import type {
    ICustomSeriesPaneRenderer,
    ICustomSeriesPaneView,
    CustomData,
    CustomSeriesOptions,
    PaneRendererCustomData,
    CustomSeriesWhitespaceData,
} from 'lightweight-charts';
import type { Time, CustomSeriesPricePlotValues } from 'lightweight-charts';

// Define your custom data type for arrow markers.
export interface ArrowMarker extends CustomData<Time> {
    time: Time;               // timestamp (must be in the chart's time format)
    price: number;            // y-coordinate (price)
    type: 'entry' | 'exit';   // marker type to decide arrow direction
}
class TradeMarkersRenderer implements ICustomSeriesPaneRenderer {
    constructor() {
        this._bars = [];
        this._options = {};
    }
    setData(bars) {
        this._bars = bars;
    }
    setOptions(options) {
        this._options = options;
    }
    draw(target, priceConverter, isHovered, hitTestData) {
        const ctx = target.getContext();
        ctx.save();
        const arrowSize = this._options.arrowSize || 10;
        // Draw each trade as an arrow
        for (const bar of this._bars) {
            const { originalData } = bar;  // our TradeMarkerData
            const x = bar.x;
            const y = priceConverter(originalData.price);
            if (originalData.direction === 'buy') {
                // Draw an upward pointing arrow below the price point
                ctx.fillStyle = this._options.upColor || 'green';
                drawArrowUp(ctx, x, y, arrowSize);
            } else if (originalData.direction === 'sell') {
                ctx.fillStyle = this._options.downColor || 'red';
                drawArrowDown(ctx, x, y, arrowSize);
            }
        }
        ctx.restore();
    }
}


class TradeMarkersSeries implements ICustomSeriesPaneView {
    constructor() {
        this._renderer = new TradeMarkersRenderer();
        this._data = null;
    }
    renderer() {
        return this._renderer;
    }
    update(data, seriesOptions) {
        // Store the latest prepared data (especially the bars list)
        this._data = data;
        this._renderer.setData(data.bars); 
        // (Assume TradeMarkersRenderer has a setData method to accept bars)
    }
    priceValueBuilder(plotRow) {
        // Return an array of price values for this data point.
        // Here, each trade has a single price, so return [price].
        return [ plotRow.price ];
    }
    isWhitespace(data) {
        // All our data points are actual trades (no placeholders), so always false.
        return false;
    }
    defaultOptions() {
        // Provide default styling options (colors for up/down arrows, etc.)
        return {
            upColor: 'green',
            downColor: 'red',
            arrowSize: 10,  // hypothetical option for size of arrow
        };
    }
}


// Implement the custom series view.
export class ArrowMarkersPaneView implements ICustomSeriesPaneView<Time, ArrowMarker, CustomSeriesOptions> {
    private markers: ArrowMarker[] = [];
    private options: CustomSeriesOptions = this.defaultOptions();

    renderer(): ICustomSeriesPaneRenderer {
        return {
            draw(target, priceToCoordinate, visibleRange) {
                const ctx = target.context;
                if (!ctx || !target.mediaSize) return;
                const { width, height } = target.mediaSize;
            
                ctx.clearRect(0, 0, width, height);
            
                for (const marker of this.markers) {
                    // Use the above helper *with the array of markers* so you can map
                    // the marker’s “time” to its index, then to an X position:
                    const x = this.timeToX(marker.time, this.markers, visibleRange, width);
                    const y = priceToCoordinate(marker.price);
                    console.log(`Drawing marker at (${x}, ${y}) with type: ${marker.type}`);
                    ctx.beginPath();
                    if (marker.type === 'entry') {
                        // Up-pointing triangle
                        ctx.moveTo(x, y - 10);
                        ctx.lineTo(x - 10, y + 10);
                        ctx.lineTo(x + 10, y + 10);
                    } else {
                        // Down-pointing triangle
                        ctx.moveTo(x, y + 10);
                        ctx.lineTo(x - 10, y - 10);
                        ctx.lineTo(x + 10, y - 10);
                    }
                    ctx.closePath();
                    ctx.fillStyle = marker.type === 'entry' ? 'green' : 'red';
                    ctx.fill();
                }
            }
            
        };
    }
    // Helper: converts a time value to an x coordinate using the visible range.
    timeToX(
        markerTime: Time,
        data: ArrowMarker[],
        visibleRange: { from: number; to: number },
        width: number
    ): number {
        // Find which index this marker’s time corresponds to.
        const markerIndex = data.findIndex(d => d.time === markerTime);
        if (markerIndex < 0) {
            return -100; // or some sentinel for “not found”
        }
    
        // Because visibleRange is in indexes, do:
        const { from, to } = visibleRange; // these are indexes
        const range = to - from;
        if (range <= 0) {
            return -100;
        }
        // The fraction of the way from left (from) to right (to).
        const relativePos = (markerIndex - from) / range;
        return relativePos * width;
    }
    

    
    // Called whenever new data or options are provided.
    update(data: PaneRendererCustomData<Time, ArrowMarker>, seriesOptions: CustomSeriesOptions): void {
        this.markers = data.bars; // Assumes your data is in the "bars" property.
        this.options = seriesOptions;
    }
    
    // For autoscaling and crosshair positioning, return an array with the price value.
    priceValueBuilder(plotRow: ArrowMarker): CustomSeriesPricePlotValues {
        return [plotRow.price];
    }
    
    // In this example, we do not consider any marker as whitespace.
    isWhitespace(data: ArrowMarker | CustomSeriesWhitespaceData<Time>): data is CustomSeriesWhitespaceData<Time> {
        return false;
    }
    
    // Default options can be extended as needed.
    defaultOptions(): CustomSeriesOptions {
        return { color: 'green' };
    }
    
    // Optional cleanup when the series is removed.
    destroy(): void {
        // If any event listeners or timers are set, clear them here.
    }
    
    
}
