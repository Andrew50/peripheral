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
		Width:  1600, // Twitter recommendation: 1200x675 for best display
		Height: 1133,
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
	page.MustSetViewport(config.Width, config.Height+80, 1, false) // +80 for additional vertical padding with larger canvas

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
				size: 16,
				color: '#f8fafc' // text-slate-50
			};
			
			// Title configuration with larger size
			const titleText = plotLayout.title || plotSpec.title;
			const titleIcon = plotSpec.titleIcon;
			
			if (titleText || titleIcon) {
				// If we have a title icon or need manual rendering, handle it later
				if (titleIcon) {
					// Store the title text and remove it from layout for manual rendering
					window.titleText = typeof titleText === 'string' ? titleText : (titleText.text || titleText);
					window.titleIcon = titleIcon;
					plotLayout.title = '';  // Remove title from plotly layout to render manually
				} else if (titleText) {
					// No icon, use Plotly's native title rendering
					if (typeof titleText === 'string') {
						plotLayout.title = {
							text: titleText,
							font: {
								family: 'Inter, system-ui, sans-serif',
								size: 48,
								color: '#f8fafc'
							},
							xref: 'paper',
							x: 0.5,
							xanchor: 'center'
						};
					} else {
						// If title is already an object, ensure proper formatting
						plotLayout.title = {
							...titleText,
							font: {
								family: 'Inter, system-ui, sans-serif',
								size: 48,
								color: '#f8fafc'
							},
							x: 0.5,
							xanchor: 'center'
						};
					}
				}
			}
			
			// Margin configuration - better centered for larger canvas (1600x1133)
			if (!plotLayout.margin) {
				plotLayout.margin = { l: 220, r: 220, t: 120, b: 180, autoexpand: true };
			} else {
				plotLayout.margin.t = 120; // Increased for title and better vertical centering
				plotLayout.margin.b = 180; // Increased for watermark and better vertical centering
				plotLayout.margin.l = 220; // Increased left margin for better horizontal centering
				plotLayout.margin.r = 220; // Increased right margin to accommodate legend
			}
			
			// Legend styling - positioned outside plot area to avoid shifting plot center
			plotLayout.legend = {
				...plotLayout.legend,
				font: { color: '#f8fafc', size: 18 },
				bgcolor: 'transparent',
				borderwidth: 0,
				orientation: 'v',
				x: 0,
				xanchor: 'left',
				y: -0.1,
				yanchor: 'middle',
				xref: 'paper',
				yref: 'paper',
				itemwidth: 60
			};
			
			// X-axis styling
			plotLayout.xaxis = {
				...plotLayout.xaxis,
				gridcolor: 'rgba(255, 255, 255, 0.05)',
				linecolor: 'rgba(248, 250, 252, 0.8)',
				linewidth: 2,
				tickfont: { color: '#f1f5f9', size: 20 },
				titlefont: { color: '#f8fafc', size: 32 },
				automargin: true,
				ticklen: 20,
				tickcolor: 'rgba(248, 250, 252, 0.8)',
				tickwidth: 2
			};
			if (plotLayout.xaxis.title) {
				plotLayout.xaxis.title = capitalizeAxisTitle(plotLayout.xaxis.title);
			}
			
			// Y-axis styling
			plotLayout.yaxis = {
				...plotLayout.yaxis,
				gridcolor: 'rgba(255, 255, 255, 0.05)',
				linecolor: 'rgba(248, 250, 252, 0.8)',
				linewidth: 2,
				tickfont: { color: '#f1f5f9', size: 20 },
				titlefont: { color: '#f8fafc', size: 32 },
				automargin: true
			};
			if (plotLayout.yaxis.title) {
				// Handle both string and object title formats
				if (typeof plotLayout.yaxis.title === 'string') {
					plotLayout.yaxis.title = {
						text: capitalizeAxisTitle(plotLayout.yaxis.title),
						standoff: 30
					};
				} else if (typeof plotLayout.yaxis.title === 'object') {
					// If it's already an object, ensure it has proper formatting
					if (plotLayout.yaxis.title.text) {
						plotLayout.yaxis.title.text = capitalizeAxisTitle(plotLayout.yaxis.title.text);
					}
					// Add standoff if not already specified
					if (!plotLayout.yaxis.title.standoff) {
						plotLayout.yaxis.title.standoff = 30;
					}
				}
			}
			
			// Process traces to apply colors and styling
			plotData.forEach((trace, index) => {
				// Use index to ensure different colors for each trace
				const color = colorPalette[index % colorPalette.length];
				
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
							size: 23,
							family: 'Inter, system-ui, sans-serif'
						};
					}
				} else if (trace.type === 'line') {
					// Handle line charts specifically
					if (!trace.line) trace.line = {};
					if (!trace.line.color) trace.line.color = color;
					if (!trace.mode) trace.mode = 'lines';
					
					// Add value labels with smart positioning for line plots (if less than 12 points)
					if (trace.y && Array.isArray(trace.y) && trace.y.length < 12 && !trace.text) {
						// Format values for display
						trace.text = trace.y.map((value) => {
							if (typeof value === 'number') {
								// For line plots, typically show more precision for smaller values
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
						trace.textposition = trace.y.map((value, index) => {
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
							size: 18,
							family: 'Inter, system-ui, sans-serif'
						};
						
						// Update mode to include text display
						if (trace.mode === 'lines') {
							trace.mode = 'lines+text';
						} else if (trace.mode === 'lines+markers') {
							trace.mode = 'lines+markers+text';
						} else if (trace.mode && !trace.mode.includes('text')) {
							trace.mode = trace.mode + '+text';
						} else {
							trace.mode = 'lines+text';
						}
					}
				} else if (trace.type === 'scatter' || !trace.type) {
					if (!trace.line) trace.line = {};
					if (!trace.line.color) trace.line.color = color;
					if (trace.marker && !trace.marker.color) trace.marker.color = color;
					
					// Add value labels with smart positioning for scatter plots (if less than 12 points)
					if (trace.y && Array.isArray(trace.y) && trace.y.length < 12 && !trace.text) {
						// Format values for display
						trace.text = trace.y.map((value) => {
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
						trace.textposition = trace.y.map((value, index) => {
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
							size: 18,
							family: 'Inter, system-ui, sans-serif'
						};
						
						// Update mode to include text display
						if (trace.mode === 'markers') {
							trace.mode = 'markers+text';
						} else if (trace.mode === 'lines') {
							trace.mode = 'lines+text';
						} else if (trace.mode === 'lines+markers') {
							trace.mode = 'lines+markers+text';
						} else if (trace.mode && !trace.mode.includes('text')) {
							trace.mode = trace.mode + '+text';
						} else if (!trace.mode) {
							trace.mode = 'markers+text';
						}
					}
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
			
			// Manually render title if we stored it for icon display
			if (window.titleText) {
				const titleContainer = document.createElement('div');
				titleContainer.style.position = 'absolute';
				titleContainer.style.top = '30px';  // Adjusted for larger canvas
				titleContainer.style.left = '50%';
				titleContainer.style.transform = 'translateX(-50%)';
				titleContainer.style.display = 'flex';
				titleContainer.style.alignItems = 'center';
				titleContainer.style.gap = '15px';  // Increased gap for larger canvas
				titleContainer.style.zIndex = '1000';
				titleContainer.style.whiteSpace = 'nowrap';
				titleContainer.style.minWidth = 'max-content';
				titleContainer.style.maxWidth = '90%';
				
				// Add ticker icon if available
				if (window.titleIcon) {
					const iconImg = document.createElement('img');
					
					// Support both PNG and JPEG formats
					if (window.titleIcon.startsWith('data:')) {
						// Already has data URI prefix
						iconImg.src = window.titleIcon;
					} else {
						// Try to detect format from base64 data
						// PNG starts with: iVBORw0KGgo
						// JPEG starts with: /9j/
						const isPNG = window.titleIcon.startsWith('iVBORw0KGgo');
						const isJPEG = window.titleIcon.startsWith('/9j/');
						
						if (isPNG) {
							iconImg.src = 'data:image/png;base64,' + window.titleIcon;
						} else if (isJPEG) {
							iconImg.src = 'data:image/jpeg;base64,' + window.titleIcon;
						} else {
							// Default to PNG if can't detect
							iconImg.src = 'data:image/png;base64,' + window.titleIcon;
						}
					}
					
					iconImg.style.width = '44px';  // Slightly larger for bigger canvas
					iconImg.style.height = '44px';
					iconImg.style.borderRadius = '6px';
					iconImg.style.objectFit = 'cover';
					titleContainer.appendChild(iconImg);
				}
				
				// Add title text
				const titleText = document.createElement('span');
				titleText.textContent = window.titleText;
				titleText.style.fontFamily = 'Inter, system-ui, sans-serif';
				titleText.style.fontSize = '38px';  // Slightly larger for bigger canvas
				titleText.style.fontWeight = '600';
				titleText.style.color = '#f8fafc';
				
				titleContainer.appendChild(titleText);
				document.getElementById('plot').appendChild(titleContainer);
			}

			const watermark = document.createElement('div');
			watermark.style.position = 'absolute';
			watermark.style.bottom = '35px';  // Adjusted for larger canvas
			watermark.style.right = '35px';   // Adjusted for larger canvas
			watermark.style.fontFamily = 'Inter, system-ui, sans-serif';
			watermark.style.fontSize = '18px';
			watermark.style.color = 'rgba(255, 255, 255, 1)';
			watermark.style.zIndex = '1000';
			watermark.innerHTML = 'Powered by <span style="font-size: 44px; font-weight: 600;">Peripheral.io</span>';
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

	// Wait for Plotly animations to complete idk if this is really necessary but just in case lmao
	time.Sleep(100 * time.Millisecond)

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
