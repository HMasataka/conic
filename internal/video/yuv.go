package video

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// YUVFormat represents the YUV pixel format
type YUVFormat int

const (
	YUV420 YUVFormat = iota
)

// YUVHeader represents the header for our custom YUV file format
type YUVHeader struct {
	Magic      [4]byte // "YUV\x00"
	Version    uint32
	Width      uint32
	Height     uint32
	Format     uint32
	FrameRate  uint32
	FrameCount uint32
}

// YUVWriter writes YUV frames to a file
type YUVWriter struct {
	file       *os.File
	header     YUVHeader
	frameSize  int
	frameCount uint32
}

// NewYUVWriter creates a new YUV file writer
func NewYUVWriter(filename string, width, height, frameRate uint32) (*YUVWriter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	frameSize := int(width*height*3/2) // YUV420 format

	writer := &YUVWriter{
		file: file,
		header: YUVHeader{
			Magic:      [4]byte{'Y', 'U', 'V', 0},
			Version:    1,
			Width:      width,
			Height:     height,
			Format:     uint32(YUV420),
			FrameRate:  frameRate,
			FrameCount: 0,
		},
		frameSize: frameSize,
	}

	// Write placeholder header
	if err := writer.writeHeader(); err != nil {
		file.Close()
		return nil, err
	}

	return writer, nil
}

// WriteFrame writes a YUV frame to the file
func (w *YUVWriter) WriteFrame(frame []byte) error {
	if len(frame) != w.frameSize {
		return fmt.Errorf("invalid frame size: expected %d, got %d", w.frameSize, len(frame))
	}

	if _, err := w.file.Write(frame); err != nil {
		return fmt.Errorf("failed to write frame: %w", err)
	}

	w.frameCount++
	return nil
}

// Close closes the YUV file and updates the header
func (w *YUVWriter) Close() error {
	// Update frame count in header
	w.header.FrameCount = w.frameCount

	// Seek to beginning and rewrite header
	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}

	if err := w.writeHeader(); err != nil {
		return err
	}

	return w.file.Close()
}

func (w *YUVWriter) writeHeader() error {
	return binary.Write(w.file, binary.LittleEndian, w.header)
}

// YUVReader reads YUV frames from a file
type YUVReader struct {
	file         *os.File
	header       YUVHeader
	frameSize    int
	currentFrame uint32
}

// NewYUVReader creates a new YUV file reader
func NewYUVReader(filename string) (*YUVReader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	reader := &YUVReader{
		file: file,
	}

	// Read header
	if err := binary.Read(file, binary.LittleEndian, &reader.header); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Validate magic
	if string(reader.header.Magic[:3]) != "YUV" {
		file.Close()
		return nil, fmt.Errorf("invalid YUV file format")
	}

	reader.frameSize = int(reader.header.Width * reader.header.Height * 3 / 2)

	return reader, nil
}

// Width returns the video width
func (r *YUVReader) Width() uint32 {
	return r.header.Width
}

// Height returns the video height
func (r *YUVReader) Height() uint32 {
	return r.header.Height
}

// FrameRate returns the video frame rate
func (r *YUVReader) FrameRate() uint32 {
	return r.header.FrameRate
}

// FrameCount returns the total number of frames
func (r *YUVReader) FrameCount() uint32 {
	return r.header.FrameCount
}

// ReadFrame reads the next YUV frame
func (r *YUVReader) ReadFrame() ([]byte, error) {
	if r.currentFrame >= r.header.FrameCount {
		return nil, io.EOF
	}

	frame := make([]byte, r.frameSize)
	if _, err := io.ReadFull(r.file, frame); err != nil {
		return nil, fmt.Errorf("failed to read frame: %w", err)
	}

	r.currentFrame++
	return frame, nil
}

// Seek seeks to a specific frame
func (r *YUVReader) Seek(frameNumber uint32) error {
	if frameNumber >= r.header.FrameCount {
		return fmt.Errorf("frame number %d out of range (0-%d)", frameNumber, r.header.FrameCount-1)
	}

	headerSize := int64(binary.Size(r.header))
	offset := headerSize + int64(frameNumber)*int64(r.frameSize)

	if _, err := r.file.Seek(offset, 0); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	r.currentFrame = frameNumber
	return nil
}

// Reset resets the reader to the beginning
func (r *YUVReader) Reset() error {
	return r.Seek(0)
}

// Close closes the YUV reader
func (r *YUVReader) Close() error {
	return r.file.Close()
}