import type{ IChartApi, Time, UTCTimestamp } from 'lightweight-charts';

interface SessionBackgroundOptions {
    preMarketColor: string;
    regularHoursColor: string;
    afterHoursColor: string;
    visible: boolean;
}

export class SessionBackground {
    private _chart: IChartApi;
    private _canvas: HTMLCanvasElement | null = null;
    private _ctx: CanvasRenderingContext2D | null = null;
    private _options: SessionBackgroundOptions = {
        preMarketColor: 'rgba(0, 100, 255, 0.1)',    // Light blue
        regularHoursColor: 'transparent',
        afterHoursColor: 'rgba(255, 150, 0, 0.1)',   // Light orange
        visible: true
    };

    constructor(chart: IChartApi) {
        this._chart = chart;
        this._init();
    }

    private _init(): void {
        const container = this._chart.chartElement();
        if (!container) return;

        // Create canvas element
        this._canvas = document.createElement('canvas');
        this._canvas.style.position = 'absolute';
        this._canvas.style.zIndex = '1';
        this._canvas.style.pointerEvents = 'none';
        container.appendChild(this._canvas);

        this._ctx = this._canvas.getContext('2d');

        // Subscribe to chart events
        this._chart.subscribeCrosshairMove(this._drawBackground.bind(this));
        this._chart.timeScale().subscribeVisibleTimeRangeChange(this._drawBackground.bind(this));
        
        // Handle resize
        const resizeObserver = new ResizeObserver(() => {
            this._updateCanvasSize();
            this._drawBackground();
        });
        resizeObserver.observe(container);
    }

    private _updateCanvasSize(): void {
        if (!this._canvas || !this._chart.chartElement()) return;
        const rect = this._chart.chartElement().getBoundingClientRect();
        this._canvas.width = rect.width;
        this._canvas.height = rect.height;
        this._canvas.style.left = '0';
        this._canvas.style.top = '0';
    }

    private _isPreMarket(timestamp: number): boolean {
        const date = new Date(timestamp);
        const hours = date.getHours();
        const minutes = date.getMinutes();
        return hours < 9 || (hours === 9 && minutes < 30);
    }

    private _isAfterHours(timestamp: number): boolean {
        const date = new Date(timestamp);
        const hours = date.getHours();
        return hours >= 16;
    }

    private _drawBackground(): void {
        if (!this._ctx || !this._canvas || !this._options.visible) return;

        const ctx = this._ctx;
        const timeScale = this._chart.timeScale();
        const visibleRange = timeScale.getVisibleRange();
        
        if (!visibleRange) return;

        // Clear the canvas
        ctx.clearRect(0, 0, this._canvas.width, this._canvas.height);

        const fromTime = (visibleRange.from as UTCTimestamp) * 1000;
        const toTime = (visibleRange.to as UTCTimestamp) * 1000;
        
        // Get coordinates for the visible range
        const fromCoord = timeScale.timeToCoordinate(visibleRange.from as Time);
        const toCoord = timeScale.timeToCoordinate(visibleRange.to as Time);
        
        if (fromCoord === null || toCoord === null) return;

        // Draw session backgrounds
        for (let time = fromTime; time <= toTime; time += 24 * 60 * 60 * 1000) {
            const date = new Date(time);
            if (date.getDay() === 0 || date.getDay() === 6) continue; // Skip weekends

            const dayStart = new Date(date);
            dayStart.setHours(0, 0, 0, 0);
            
            // Pre-market (4:00 AM - 9:30 AM)
            const preMarketStart = new Date(dayStart);
            preMarketStart.setHours(4, 0, 0, 0);
            const preMarketEnd = new Date(dayStart);
            preMarketEnd.setHours(9, 30, 0, 0);
            
            // Regular hours (9:30 AM - 4:00 PM)
            const regularStart = preMarketEnd;
            const regularEnd = new Date(dayStart);
            regularEnd.setHours(16, 0, 0, 0);
            
            // After hours (4:00 PM - 8:00 PM)
            const afterHoursStart = regularEnd;
            const afterHoursEnd = new Date(dayStart);
            afterHoursEnd.setHours(20, 0, 0, 0);

            // Draw backgrounds
            this._drawSessionBackground(preMarketStart.getTime(), preMarketEnd.getTime(), this._options.preMarketColor);
            this._drawSessionBackground(regularStart.getTime(), regularEnd.getTime(), this._options.regularHoursColor);
            this._drawSessionBackground(afterHoursStart.getTime(), afterHoursEnd.getTime(), this._options.afterHoursColor);
        }
    }

    private _drawSessionBackground(fromTime: number, toTime: number, color: string): void {
        if (!this._ctx || !this._canvas) return;

        const timeScale = this._chart.timeScale();
        const fromCoord = timeScale.timeToCoordinate(fromTime / 1000 as UTCTimestamp);
        const toCoord = timeScale.timeToCoordinate(toTime / 1000 as UTCTimestamp);

        if (fromCoord === null || toCoord === null) return;

        this._ctx.fillStyle = color;
        this._ctx.fillRect(
            fromCoord,
            0,
            toCoord - fromCoord,
            this._canvas.height
        );
    }

    public setOptions(options: Partial<SessionBackgroundOptions>): void {
        this._options = { ...this._options, ...options };
        this._drawBackground();
    }

    public destroy(): void {
        if (this._canvas && this._canvas.parentElement) {
            this._canvas.parentElement.removeChild(this._canvas);
        }
    }
} 