package main

import (
	"flag"
	"fmt"
	"log"
	"math"

	"github.com/HMasataka/conic/internal/video"
)

var (
	output    = flag.String("output", "sample.yuv", "Output YUV file")
	width     = flag.Uint("width", 640, "Video width")
	height    = flag.Uint("height", 480, "Video height")
	fps       = flag.Uint("fps", 30, "Frames per second")
	duration  = flag.Uint("duration", 10, "Duration in seconds")
	pattern   = flag.String("pattern", "gradient", "Pattern type: gradient, checkerboard, bounce, color")
)

func main() {
	flag.Parse()

	frameCount := uint32(*fps * *duration)
	
	log.Printf("Generating YUV video: %dx%d, %d fps, %d seconds (%d frames)",
		*width, *height, *fps, *duration, frameCount)

	writer, err := video.NewYUVWriter(*output, uint32(*width), uint32(*height), uint32(*fps))
	if err != nil {
		log.Fatal("Failed to create YUV writer:", err)
	}
	defer writer.Close()

	frameSize := int(*width * *height * 3 / 2)

	for frame := uint32(0); frame < frameCount; frame++ {
		data := make([]byte, frameSize)

		switch *pattern {
		case "gradient":
			generateGradientFrame(data, uint32(*width), uint32(*height), frame, frameCount)
		case "checkerboard":
			generateCheckerboardFrame(data, uint32(*width), uint32(*height), frame, frameCount)
		case "bounce":
			generateBouncingBallFrame(data, uint32(*width), uint32(*height), frame, uint32(*fps))
		case "color":
			generateColorFrame(data, uint32(*width), uint32(*height), frame, frameCount)
		default:
			generateGradientFrame(data, uint32(*width), uint32(*height), frame, frameCount)
		}

		if err := writer.WriteFrame(data); err != nil {
			log.Fatal("Failed to write frame:", err)
		}

		if frame%uint32(*fps) == 0 {
			log.Printf("Progress: %d/%d frames", frame, frameCount)
		}
	}

	log.Printf("Successfully generated %s", *output)
}

// generateGradientFrame creates a moving gradient pattern
func generateGradientFrame(data []byte, width, height, frame, totalFrames uint32) {
	ySize := width * height
	
	// Y plane - diagonal gradient that moves
	for y := uint32(0); y < height; y++ {
		for x := uint32(0); x < width; x++ {
			offset := float64(frame) / float64(totalFrames) * 255
			value := uint8((float64(x+y)/float64(width+height)*255 + offset)) % 255
			data[y*width+x] = value
		}
	}

	// U and V planes - add some color variation
	uvWidth := width / 2
	uvHeight := height / 2
	for y := uint32(0); y < uvHeight; y++ {
		for x := uint32(0); x < uvWidth; x++ {
			uOffset := ySize + y*uvWidth + x
			vOffset := ySize + ySize/4 + y*uvWidth + x
			
			// Subtle color variation
			data[uOffset] = uint8(128 + 20*math.Sin(float64(frame)/10))
			data[vOffset] = uint8(128 + 20*math.Cos(float64(frame)/10))
		}
	}
}

// generateCheckerboardFrame creates an animated checkerboard pattern
func generateCheckerboardFrame(data []byte, width, height, frame, totalFrames uint32) {
	ySize := width * height
	squareSize := uint32(32)
	
	// Animate by shifting the pattern
	shift := (frame * 2) % (squareSize * 2)
	
	// Y plane
	for y := uint32(0); y < height; y++ {
		for x := uint32(0); x < width; x++ {
			if ((x+shift)/squareSize+(y+shift)/squareSize)%2 == 0 {
				data[y*width+x] = 255 // white
			} else {
				data[y*width+x] = 0 // black
			}
		}
	}

	// U and V planes - neutral gray
	for i := ySize; i < uint32(len(data)); i++ {
		data[i] = 128
	}
}

// generateBouncingBallFrame creates a bouncing ball animation
func generateBouncingBallFrame(data []byte, width, height, frame, fps uint32) {
	ySize := width * height
	
	// Background - gray
	for i := uint32(0); i < ySize; i++ {
		data[i] = 128
	}

	// Ball parameters
	ballRadius := uint32(30)
	time := float64(frame) / float64(fps)
	
	// Ball position with physics
	ballX := width/2 + uint32(200*math.Sin(time*2))
	ballY := height/2 + uint32(100*math.Sin(time*3+math.Pi/4))
	
	// Draw ball on Y plane
	for y := uint32(0); y < height; y++ {
		for x := uint32(0); x < width; x++ {
			dx := int32(x) - int32(ballX)
			dy := int32(y) - int32(ballY)
			distSq := uint32(dx*dx + dy*dy)
			
			if distSq <= ballRadius*ballRadius {
				// Inside the ball - make it white
				data[y*width+x] = 255
			}
		}
	}

	// U and V planes - add color to the ball
	uvWidth := width / 2
	uvHeight := height / 2
	ballRadiusUV := ballRadius / 2
	ballXUV := ballX / 2
	ballYUV := ballY / 2
	
	for y := uint32(0); y < uvHeight; y++ {
		for x := uint32(0); x < uvWidth; x++ {
			dx := int32(x) - int32(ballXUV)
			dy := int32(y) - int32(ballYUV)
			distSq := uint32(dx*dx + dy*dy)
			
			uOffset := ySize + y*uvWidth + x
			vOffset := ySize + ySize/4 + y*uvWidth + x
			
			if distSq <= ballRadiusUV*ballRadiusUV {
				// Inside the ball - make it colorful
				data[uOffset] = 64  // blue tint
				data[vOffset] = 192 // red tint
			} else {
				// Outside - neutral
				data[uOffset] = 128
				data[vOffset] = 128
			}
		}
	}
}

// generateColorFrame creates frames that cycle through colors
func generateColorFrame(data []byte, width, height, frame, totalFrames uint32) {
	ySize := width * height
	
	// Calculate color based on frame
	hue := float64(frame) / float64(totalFrames) * 360.0
	
	// Convert HSV to YUV (simplified)
	// Y = brightness (constant)
	// U, V = color components
	
	// Y plane - constant brightness
	for i := uint32(0); i < ySize; i++ {
		data[i] = 180
	}
	
	// U and V planes - color based on hue
	u := uint8(128 + 100*math.Sin(hue*math.Pi/180))
	v := uint8(128 + 100*math.Cos(hue*math.Pi/180))
	
	for i := ySize; i < ySize+ySize/4; i++ {
		data[i] = u
	}
	for i := ySize + ySize/4; i < uint32(len(data)); i++ {
		data[i] = v
	}
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options]\n", flag.CommandLine.Name())
		fmt.Fprintf(flag.CommandLine.Output(), "\nGenerates YUV420 video files with various test patterns.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nPattern types:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  gradient     - Moving diagonal gradient\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  checkerboard - Animated checkerboard pattern\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  bounce       - Bouncing ball animation\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  color        - Color cycling animation\n")
	}
}