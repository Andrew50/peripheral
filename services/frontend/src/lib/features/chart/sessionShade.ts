import { CanvasRenderingTarget2D } from 'fancy-canvas';
import type {
	Coordinate,
	DataChangedScope,
	ISeriesPrimitive,
	ISeriesPrimitivePaneRenderer,
	ISeriesPrimitivePaneView,
	SeriesAttachedParameter,
	SeriesDataItemTypeMap,
	SeriesPrimitivePaneViewZOrder,
	SeriesType,
	Time
} from 'lightweight-charts';
import { PluginBase } from './plugin-base';

interface SessionHighlightingRendererData {
	x: Coordinate | number;
	color: string;
}

class SessionHighlightingPaneRenderer implements ISeriesPrimitivePaneRenderer {
	_viewData: SessionHighlightingViewData;
	constructor(data: SessionHighlightingViewData) {
		this._viewData = data;
	}
	draw(target: CanvasRenderingTarget2D) {
		const points: SessionHighlightingRendererData[] = this._viewData.data;

		// Skip drawing if no visible points
		if (points.length === 0) {
			return;
		}

		target.useBitmapCoordinateSpace((scope) => {
			const ctx = scope.context;
			const yTop = 0;
			const height = scope.bitmapSize.height;
			const halfWidth = (scope.horizontalPixelRatio * this._viewData.barWidth) / 2;
			const cutOff = -1 * (halfWidth + 1);
			const maxX = scope.bitmapSize.width;

			// Batch consecutive bars with the same color
			let currentColor = '';
			let startX = 0;
			let lastX = 0;

			points.forEach((point, index) => {
				// Skip transparent points completely
				if (point.color === 'rgba(0, 0, 0, 0)') return;

				const xScaled = point.x * scope.horizontalPixelRatio;
				if (xScaled < cutOff) return;

				const x1 = Math.max(0, Math.round(xScaled - halfWidth));
				const x2 = Math.min(maxX, Math.round(xScaled + halfWidth));

				if (
					point.color !== currentColor ||
					index === points.length - 1 ||
					(index < points.length - 1 && Math.abs(x1 - lastX) > 1)
				) {
					// Draw the previous batch if color changes
					if (currentColor && startX < lastX) {
						ctx.fillStyle = currentColor;
						ctx.fillRect(startX, yTop, lastX - startX, height);
					}
					// Start a new batch
					currentColor = point.color;
					startX = x1;
				}
				lastX = x2;

				// Draw the final point if it's the last one
				if (index === points.length - 1 && currentColor !== 'rgba(0, 0, 0, 0)') {
					ctx.fillStyle = currentColor;
					ctx.fillRect(x1, yTop, x2 - x1, height);
				}
			});
		});
	}
}

interface SessionHighlightingViewData {
	data: SessionHighlightingRendererData[];
	options: Required<SessionHighlightingOptions>;
	barWidth: number;
}

class SessionHighlightingPaneView implements ISeriesPrimitivePaneView {
	_source: SessionHighlighting;
	_data: SessionHighlightingViewData;

	constructor(source: SessionHighlighting) {
		this._source = source;
		this._data = {
			data: [],
			barWidth: 6,
			options: this._source._options
		};
	}

	update() {
		const timeScale = this._source.chart.timeScale();
		this._data.data = this._source._backgroundColors.map((d) => {
			return {
				x: timeScale.timeToCoordinate(d.time) ?? -100,
				color: d.color
			};
		});
		if (this._data.data.length > 1) {
			this._data.barWidth = this._data.data[1].x - this._data.data[0].x;
		} else {
			this._data.barWidth = 6;
		}
	}

	renderer() {
		return new SessionHighlightingPaneRenderer(this._data);
	}

	zOrder(): SeriesPrimitivePaneViewZOrder {
		return 'bottom';
	}
}

export interface SessionHighlightingOptions {
	skipRegularHours?: boolean;
}

const defaults: Required<SessionHighlightingOptions> = {
	skipRegularHours: true
};

interface BackgroundData {
	time: Time;
	color: string;
}

export type SessionHighlighter = (date: Time) => string;

export class SessionHighlighting extends PluginBase implements ISeriesPrimitive<Time> {
	_paneViews: SessionHighlightingPaneView[];
	_seriesData: SeriesDataItemTypeMap[SeriesType][] = [];
	_backgroundColors: BackgroundData[] = [];
	_options: Required<SessionHighlightingOptions>;
	_highlighter: SessionHighlighter;

	constructor(highlighter: SessionHighlighter, options: SessionHighlightingOptions = {}) {
		super();
		this._highlighter = highlighter;
		this._options = { ...defaults, ...options };
		this._paneViews = [new SessionHighlightingPaneView(this)];
	}

	updateAllViews() {
		this._paneViews.forEach((pw) => pw.update());
	}

	paneViews() {
		return this._paneViews;
	}

	attached(p: SeriesAttachedParameter<Time>): void {
		super.attached(p);
		this.dataUpdated('full');
	}

	dataUpdated(scope: DataChangedScope) {
		// Optimize by only updating what's needed based on scope
		if (scope === 'update') {
			// Only update the last data point
			const data = this.series.data();
			if (data.length > 0) {
				const lastPoint = data[data.length - 1];
				// Update existing point or add new one
				if (
					this._backgroundColors.length > 0 &&
					this._backgroundColors[this._backgroundColors.length - 1].time === lastPoint.time
				) {
					this._backgroundColors[this._backgroundColors.length - 1].color = this._highlighter(
						lastPoint.time
					);
				} else {
					const color = this._highlighter(lastPoint.time);
					// Only add non-transparent colors if skipRegularHours is enabled
					if (!this._options.skipRegularHours || color !== 'rgba(0, 0, 0, 0)') {
						this._backgroundColors.push({
							time: lastPoint.time,
							color
						});
					}
				}
			}
		} else {
			// Full update needed
			if (this._options.skipRegularHours) {
				// Filter out regular market hours when creating the background colors
				this._backgroundColors = this.series
					.data()
					.map((dataPoint) => {
						const color = this._highlighter(dataPoint.time);
						return { time: dataPoint.time, color };
					})
					.filter((item) => item.color !== 'rgba(0, 0, 0, 0)');
			} else {
				// Include all points
				this._backgroundColors = this.series.data().map((dataPoint) => ({
					time: dataPoint.time,
					color: this._highlighter(dataPoint.time)
				}));
			}
		}
		this.requestUpdate();
	}
}

// Convenient default highlighter for dark theme
export function createDefaultSessionHighlighter(): SessionHighlighter {
	return createDetailedSessionHighlighter();
}

export function createDetailedSessionHighlighter(): SessionHighlighter {
	return (timestamp: Time) => {
		// Convert timestamp to Date without caching
		const date = new Date(
			typeof timestamp === 'number'
				? timestamp * 1000
				: typeof timestamp === 'string'
					? timestamp
					: new Date().setFullYear(
						(timestamp as any).year,
						(timestamp as any).month - 1,
						(timestamp as any).day
					)
		);

		// Calculate time in minutes using UTC methods
		const hours = date.getUTCHours();
		const timeInMinutes = hours * 60 + date.getUTCMinutes();

		// Pre-market: 4:00 AM - 9:30 AM UTC (240-570 minutes)
		if (timeInMinutes >= 240 && timeInMinutes < 570) {
			return 'rgba(92, 80, 59, 0.3)'; // Light orange for post-market
		}
		// Post-market: 4:00 PM - 8:00 PM UTC (960-1200 minutes)
		else if (timeInMinutes >= 960 && timeInMinutes < 1200) {
			return 'rgba(50, 50, 80, 0.3)'; // Dark blue for pre-market
		}
		// Regular market hours or closed
		else {
			return 'rgba(0, 0, 0, 0)'; // Transparent for regular hours
		}
	};
}
