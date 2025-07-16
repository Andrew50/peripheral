package plotly

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

//go:embed template.html
var plotlyHTML string

// Renderer handles Plotly plot rendering using a headless browser
type Renderer struct {
	browser *rod.Browser
	mu      sync.RWMutex
	closed  bool
}

// PlotConfig contains configuration for the plot rendering
type PlotConfig struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// DefaultPlotConfig returns sensible defaults for Twitter images
func DefaultPlotConfig() PlotConfig {
	return PlotConfig{
		Width:  1200, // Twitter recommendation: 1200x675 for best display
		Height: 675,
	}
}

// New creates a new Renderer instance
func New() (*Renderer, error) {
	// Configure launcher with optimized settings for container environment
	l := launcher.New().
		Headless(true).
		NoSandbox(true). // Required in containers
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Set("disable-setuid-sandbox").
		Set("no-first-run").
		Set("no-zygote").
		Set("disable-background-timer-throttling").
		Set("disable-backgrounding-occluded-windows").
		Set("disable-renderer-backgrounding").
		Set("disable-features", "TranslateUI").
		Set("disable-ipc-flooding-protection")

	// Check if we're in a container and use the installed chromium
	if os.Getenv("IN_CONTAINER") == "true" {
		// Use the chromium-browser installed via apk in Alpine
		chromiumPath := "/usr/bin/chromium-browser"
		if _, err := os.Stat(chromiumPath); err == nil {
			l = l.Bin(chromiumPath)
		}
	}

	url, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(url).MustConnect()

	return &Renderer{
		browser: browser,
	}, nil
}

// RenderPlot renders a Plotly plot specification to a base64 PNG
func (r *Renderer) RenderPlot(ctx context.Context, plotSpec interface{}, config *PlotConfig) (string, error) {
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return "", fmt.Errorf("renderer is closed")
	}
	r.mu.RUnlock()

	// Use default config if not provided
	if config == nil {
		cfg := DefaultPlotConfig()
		config = &cfg
	}

	// Create a new page for this render
	page := r.browser.MustPage()
	defer page.MustClose()

	// Set viewport for consistent rendering - add extra height for padding and watermark
	page.MustSetViewport(config.Width, config.Height+60, 1, false) // +40 for horizontal padding, +60 for vertical padding

	// Navigate to a blank page first
	if err := page.Navigate("about:blank"); err != nil {
		return "", fmt.Errorf("failed to navigate to blank page: %w", err)
	}

	// Set the HTML content
	if err := page.SetDocumentContent(plotlyHTML); err != nil {
		return "", fmt.Errorf("failed to set document content: %w", err)
	}

	// Wait for the document to be ready
	page.MustWaitLoad()

	// Wait for Plotly to load from CDN with retry logic
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		plotlyCheck := page.MustEval(`() => typeof Plotly !== 'undefined'`)
		if plotlyCheck.Bool() {
			break
		}
		if i == maxRetries-1 {
			return "", fmt.Errorf("Plotly.js failed to load after %d attempts", maxRetries)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Convert plot spec to JSON
	plotJSON, err := json.Marshal(plotSpec)
	if err != nil {
		return "", fmt.Errorf("failed to marshal plot spec: %w", err)
	}

	// Render the plot using proper parameter passing
	renderScript := `(plotDataJSON, width, height) => {
		try {
			// Parse the JSON data
			const plotSpec = JSON.parse(plotDataJSON);
			
			// Extract data and layout
			const plotData = plotSpec.data || [];
			const plotLayout = plotSpec.layout || {};
			
			// Apply dimensions
			plotLayout.width = width;
			plotLayout.height = height;
			
			// Frontend color palette
			const colorPalette = [
				'#64C9CF', // turquoise
				'#FC6B3F', // vivid orange
				'#A17BFE', // soft indigo
				'#03DAC6', // aqua
				'#F6BD60', // warm sand
				'#9DC2FF', // sky blue
				'#F95738', // vermilion
				'#45C4B0', // teal
				'#FF99C8', // cotton-candy pink
				'#FFD43B', // sunflower yellow
			];
			
			// Helper function to capitalize axis titles
			const capitalizeAxisTitle = (title) => {
				if (!title || typeof title !== 'string') return title;
				return title
					.replace(/_/g, ' ')
					.split(' ')
					.map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
					.join(' ');
			};
			
			// Apply frontend theme styling
			plotLayout.paper_bgcolor = '#121212';  // Match chat background
			plotLayout.plot_bgcolor = '#121212';   // Match chat background
			
			// Font configuration to match frontend
			plotLayout.font = {
				family: 'Inter, system-ui, sans-serif',
				size: 12,
				color: '#f8fafc' // text-slate-50
			};
			
			// Title configuration with larger size
			if (plotLayout.title) {
				const titleIcon = plotSpec.titleIcon;
				
				if (typeof plotLayout.title === 'string') {
					plotLayout.title = {
						text: plotLayout.title,
						font: {
							family: 'Inter, system-ui, sans-serif',
							size: 36,
							color: '#f8fafc'
						},
						xref: 'paper',
						x: 0.5,
						xanchor: 'center'
					};
				} else {
					// If title is already an object, update the font
					plotLayout.title.font = {
						family: 'Inter, system-ui, sans-serif',
						size: 36,
						color: '#f8fafc'
					};
				}
				
				// If we have a title icon, we'll handle title rendering manually later
				if (titleIcon) {
					// Store the title text and remove it from layout for manual rendering
					window.titleText = plotLayout.title.text || plotLayout.title;
					window.titleIcon = titleIcon;
					plotLayout.title = '';  // Remove title from plotly layout
				}
			}
			
			// Margin configuration - shift plot leftward while keeping legend position
			if (!plotLayout.margin) {
				plotLayout.margin = { l: 80, r: 180, t: 90, b: 130, autoexpand: true };
			} else {
				plotLayout.margin.t = 90; // Moderate increase for title
				plotLayout.margin.b = 130; // Ensure enough space for watermark
				plotLayout.margin.l = 80; // Reduced left margin to shift plot left
				plotLayout.margin.r = 180; // Increased right margin to maintain legend position
			}
			
			// Legend styling
			if (!plotLayout.showlegend === undefined) plotLayout.showlegend = true;
			plotLayout.legend = {
				...plotLayout.legend,
				font: { color: '#f8fafc', size: 16 },
				bgcolor: 'transparent',
				borderwidth: 0,
				orientation: 'v',
				x: -0.2,
				xanchor: 'left',
				y: -0.3,
				yanchor: 'bottom',
				xref: 'paper',
				yref: 'paper',
				itemwidth: 50
			};
			
			// X-axis styling
			plotLayout.xaxis = {
				...plotLayout.xaxis,
				gridcolor: 'rgba(255, 255, 255, 0.05)',
				linecolor: 'rgba(71, 85, 105, 0.8)',
				tickfont: { color: '#f1f5f9', size: 16 },
				titlefont: { color: '#f8fafc', size: 20 },
				automargin: true
			};
			if (plotLayout.xaxis.title) {
				plotLayout.xaxis.title = capitalizeAxisTitle(plotLayout.xaxis.title);
			}
			
			// Y-axis styling
			plotLayout.yaxis = {
				...plotLayout.yaxis,
				gridcolor: 'rgba(255, 255, 255, 0.05)',
				linecolor: 'rgba(71, 85, 105, 0.8)',
				tickfont: { color: '#f1f5f9', size: 16 },
				titlefont: { color: '#f8fafc', size: 20 },
				automargin: true
			};
			if (plotLayout.yaxis.title) {
				plotLayout.yaxis.title = capitalizeAxisTitle(plotLayout.yaxis.title);
			}
			
			// Process traces to apply colors and styling
			plotData.forEach((trace, index) => {
				// Randomly select color from palette
				const randomIndex = Math.floor(Math.random() * colorPalette.length);
				const color = colorPalette[randomIndex];
				
				// Add default trace name if not provided
				if (!trace.name) {
					trace.name = 'Series ' + (index + 1);
				}
				
				if (trace.type === 'bar') {
					if (!trace.marker) trace.marker = {};
					if (!trace.marker.color) trace.marker.color = color;
					trace.marker.opacity = 1;
					// Add border lines to bars
					if (!trace.marker.line) {
						trace.marker.line = {
							color: 'rgba(71, 85, 105, 0.4)',
							width: 0.5
						};
					}
					// Add value labels above bars
					if (trace.y && Array.isArray(trace.y) && !trace.text) {
						trace.text = trace.y.map((value) => {
							if (typeof value === 'number') {
								// Format numbers with appropriate precision
								if (Math.abs(value) >= 1000000) {
									return (value / 1000000).toFixed(1) + 'M';
								} else if (Math.abs(value) >= 1000) {
									return (value / 1000).toFixed(1) + 'K';
								} else if (Math.abs(value) < 1 && Math.abs(value) > 0) {
									return value.toFixed(3);
								} else {
									return value.toFixed(1);
								}
							}
							return String(value);
						});
						trace.textposition = 'outside';
						trace.textfont = {
							color: '#f1f5f9',
							size: 14,
							family: 'Inter, system-ui, sans-serif'
						};
					}
				} else if (trace.type === 'scatter' || !trace.type) {
					if (!trace.line) trace.line = {};
					if (!trace.line.color) trace.line.color = color;
					if (trace.marker && !trace.marker.color) trace.marker.color = color;
				}
				
				// Hover label styling
				trace.hoverlabel = {
					bgcolor: '#1e293b',
					bordercolor: '#475569',
					borderwidth: 1,
					font: {
						color: '#ffffff',
						size: 13,
						family: 'Inter, system-ui, sans-serif'
					},
					namelength: -1
				};
			});
			
			// Configuration
			const config = {
				responsive: false,
				displayModeBar: false,
				displaylogo: false,
				staticPlot: true
			};
			
			// Check if we have valid data
			if (!plotData || plotData.length === 0) {
				throw new Error('No plot data provided');
			}
			
			// Render the plot
			Plotly.newPlot('plot', plotData, plotLayout, config);
			
			// Manually render title with icon if provided
			if (window.titleIcon && window.titleText) {
				const titleContainer = document.createElement('div');
				titleContainer.style.position = 'absolute';
				titleContainer.style.top = '20px';
				titleContainer.style.left = '50%';
				titleContainer.style.transform = 'translateX(-50%)';
				titleContainer.style.display = 'flex';
				titleContainer.style.alignItems = 'center';
				titleContainer.style.gap = '12px';
				titleContainer.style.zIndex = '1000';
				titleContainer.style.whiteSpace = 'nowrap';
				titleContainer.style.minWidth = 'max-content';
				titleContainer.style.maxWidth = '90%';
				
				// Add ticker icon
				const iconImg = document.createElement('img');
				iconImg.src = window.titleIcon.startsWith('data:') ? window.titleIcon : 'data:image/png;base64,' + window.titleIcon;
				iconImg.style.width = '40px';
				iconImg.style.height = '40px';
				iconImg.style.borderRadius = '6px';
				iconImg.style.objectFit = 'cover';
				
				// Add title text
				const titleText = document.createElement('span');
				titleText.textContent = window.titleText;
				titleText.style.fontFamily = 'Inter, system-ui, sans-serif';
				titleText.style.fontSize = '34px';
				titleText.style.fontWeight = '600';
				titleText.style.color = '#f8fafc';
				
				titleContainer.appendChild(iconImg);
				titleContainer.appendChild(titleText);
				document.getElementById('plot').appendChild(titleContainer);
			}

			const watermark = document.createElement('div');
			watermark.style.position = 'absolute';
			watermark.style.bottom = '25px';  // Increased from 10px
			watermark.style.right = '25px';   // Increased from 10px
			watermark.style.fontFamily = 'Inter, system-ui, sans-serif';
			watermark.style.fontSize = '16px';
			watermark.style.color = 'rgba(255, 255, 255, 1)';
			watermark.style.zIndex = '1000';
			watermark.innerHTML = 'Powered by <span style="font-size: 28px; font-weight: 600;">Peripheral.io</span>';
			document.getElementById('plot').appendChild(watermark);
			
			return true;
		} catch (e) {
			console.error('Plot error:', e.message);
			console.error('Stack:', e.stack);
			return false;
		}
	}`

	result, err := page.Eval(renderScript, string(plotJSON), config.Width, config.Height)
	if err != nil {
		return "", fmt.Errorf("failed to execute render script: %w", err)
	}
	if !result.Value.Bool() {
		return "", fmt.Errorf("failed to render plot")
	}

	// Wait for plot to fully render
	page.MustWaitIdle()

	// Capture screenshot of the plot element
	plotElement, err := page.Element("#plot")
	if err != nil {
		return "", fmt.Errorf("failed to find plot element: %w", err)
	}

	screenshot, err := plotElement.Screenshot(proto.PageCaptureScreenshotFormatPng, 100)
	if err != nil {
		return "", fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Convert to base64
	base64Img := base64.StdEncoding.EncodeToString(screenshot)

	return base64Img, nil
}

// Close shuts down the renderer and browser
func (r *Renderer) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	return r.browser.Close()
}
