package plotly

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	"backend/internal/app/helpers"
	"backend/internal/data"
)

func RenderTwitterPlotToBase64(conn *data.Conn, plot interface{}, watermark bool) (string, error) {
	if plot == nil {
		return "", nil
	}
	if plotMap, ok := plot.(map[string]interface{}); ok {
		if titleTicker, exists := plotMap["titleTicker"].(string); exists && titleTicker != "" {
			titleIcon, _ := helpers.GetIcon(conn, titleTicker)
			plotMap["titleIcon"] = titleIcon
		}
		if _, hasData := plotMap["data"]; hasData {

			// Create renderer
			renderer, err := New()
			if err != nil {
				log.Printf("Failed to create Plotly renderer: %v", err)
			} else {
				defer func() {
					if err := renderer.Close(); err != nil {
						log.Printf("warning: failed to close plotly renderer: %v", err)
					}
				}()

				// Render the plot
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				var base64PNG string
				var err error
				if watermark {
					base64PNG, err = renderer.RenderPlot(ctx, plot, nil)
					if err != nil {
						log.Printf("Failed to render plot: %v", err)
					}
				} else {
					base64PNG, err = renderer.RenderPlotNoWatermark(ctx, plot)
					if err != nil {
						log.Printf("Failed to render plot: %v", err)
					}
				}

				saveImageToContainer(base64PNG)
				return base64PNG, nil
			}
		}
	}
	return "", fmt.Errorf("plot is nil")
}

// saveImageToContainer saves base64 image data to container filesystem for debugging

func saveImageToContainer(base64Data string) {
	if base64Data == "" {
		return
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		log.Printf("Failed to decode base64 image: %v", err)
		return
	}

	// Use fixed filename
	filename := "/tmp/peripheral_plot.png"

	// Write to file
	err = os.WriteFile(filename, data, 0600)
	if err != nil {
		log.Printf("Failed to save image to %s: %v", filename, err)
		return
	}

	log.Printf("✅ Plot image saved to container at: %s", filename)
	fmt.Printf("🚀 One-liner: docker cp $(docker ps --format 'table {{.Names}}' | grep backend | head -n1):/tmp/peripheral_plot.png ~/Desktop/ && open ~/Desktop/peripheral_plot.png\n")
}
