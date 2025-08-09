package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
)

var (
	output     = flag.String("output", "sample.wav", "Output WAV file name")
	duration   = flag.Float64("duration", 5.0, "Duration in seconds")
	frequency  = flag.Float64("freq", 440.0, "Frequency in Hz (440 = A4)")
	sampleRate = flag.Int("rate", 48000, "Sample rate (48000 for Opus)")
)

// WAV file header structure
type wavHeader struct {
	ChunkID       [4]byte // "RIFF"
	ChunkSize     uint32  // File size - 8
	Format        [4]byte // "WAVE"
	Subchunk1ID   [4]byte // "fmt "
	Subchunk1Size uint32  // 16 for PCM
	AudioFormat   uint16  // 1 for PCM
	NumChannels   uint16  // 2 for stereo
	SampleRate    uint32  // Sample rate
	ByteRate      uint32  // SampleRate * NumChannels * BitsPerSample/8
	BlockAlign    uint16  // NumChannels * BitsPerSample/8
	BitsPerSample uint16  // 16 bits
	Subchunk2ID   [4]byte // "data"
	Subchunk2Size uint32  // NumSamples * NumChannels * BitsPerSample/8
}

func main() {
	flag.Parse()

	log.Printf("Generating %s: %.1f seconds of %.1f Hz sine wave at %d Hz sample rate",
		*output, *duration, *frequency, *sampleRate)

	// Calculate parameters
	numChannels := uint16(2) // Stereo
	bitsPerSample := uint16(16)
	numSamples := int(*duration * float64(*sampleRate))

	// Create WAV header
	header := wavHeader{
		ChunkID:       [4]byte{'R', 'I', 'F', 'F'},
		Format:        [4]byte{'W', 'A', 'V', 'E'},
		Subchunk1ID:   [4]byte{'f', 'm', 't', ' '},
		Subchunk1Size: 16,
		AudioFormat:   1, // PCM
		NumChannels:   numChannels,
		SampleRate:    uint32(*sampleRate),
		ByteRate:      uint32(*sampleRate) * uint32(numChannels) * uint32(bitsPerSample) / 8,
		BlockAlign:    numChannels * bitsPerSample / 8,
		BitsPerSample: bitsPerSample,
		Subchunk2ID:   [4]byte{'d', 'a', 't', 'a'},
		Subchunk2Size: uint32(numSamples) * uint32(numChannels) * uint32(bitsPerSample) / 8,
	}
	header.ChunkSize = 36 + header.Subchunk2Size

	// Create output file
	file, err := os.Create(*output)
	if err != nil {
		log.Fatal("Failed to create file:", err)
	}
	defer file.Close()

	// Write header
	if err := binary.Write(file, binary.LittleEndian, header); err != nil {
		log.Fatal("Failed to write header:", err)
	}

	// Generate and write audio samples
	amplitude := float64(32767) * 0.3 // 30% of max volume to avoid clipping
	omega := 2.0 * math.Pi * *frequency / float64(*sampleRate)

	for i := range numSamples {
		// Generate sine wave sample
		value := int16(amplitude * math.Sin(omega*float64(i)))

		// Write stereo sample (same value for both channels)
		if err := binary.Write(file, binary.LittleEndian, value); err != nil {
			log.Fatal("Failed to write sample:", err)
		}
		if err := binary.Write(file, binary.LittleEndian, value); err != nil {
			log.Fatal("Failed to write sample:", err)
		}
	}

	log.Printf("Successfully generated %s", *output)
	fmt.Printf("\nWAV file details:\n")
	fmt.Printf("- Duration: %.1f seconds\n", *duration)
	fmt.Printf("- Frequency: %.1f Hz\n", *frequency)
	fmt.Printf("- Sample Rate: %d Hz\n", *sampleRate)
	fmt.Printf("- Channels: %d (stereo)\n", numChannels)
	fmt.Printf("- Bit Depth: %d bits\n", bitsPerSample)
	fmt.Printf("- File Size: %d bytes\n", header.ChunkSize+8)
}
