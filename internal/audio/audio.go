package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/mewkiz/flac"
)

// AudioData содержит декодированные аудиоданные
type AudioData struct {
	Samples    []int16
	SampleRate int
	Channels   int
	BitDepth   int
}

// ReadFile читает аудиофайл и возвращает PCM данные
func ReadFile(path string) (*AudioData, error) {
	ext := strings.ToLower(filepath.Ext(path))

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	switch ext {
	case ".wav", ".wave":
		return decodeWAV(file)
	case ".flac":
		return decodeFLAC(file)
	default:
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}
}

// WriteFile записывает аудиоданные в файл
func WriteFile(path string, data *AudioData) error {
	ext := strings.ToLower(filepath.Ext(path))

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	switch ext {
	case ".wav", ".wave":
		return encodeWAV(file, data)
	case ".flac":
		return fmt.Errorf("FLAC encoding requires go-audio-converter; use WAV for now")
	default:
		return fmt.Errorf("unsupported output format: %s", ext)
	}
}

// decodeWAV декодирует WAV файл
func decodeWAV(r io.Reader) (*AudioData, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	decoder := wav.NewDecoder(bytes.NewReader(data))
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}

	if err := decoder.FwdToPCM(); err != nil {
		return nil, fmt.Errorf("forward to PCM: %w", err)
	}

	sampleRate := int(decoder.SampleRate)
	channels := int(decoder.NumChans)
	bitDepth := int(decoder.BitDepth)

	buf := &audio.IntBuffer{
		Data:   make([]int, 0),
		Format: &audio.Format{SampleRate: sampleRate, NumChannels: channels},
	}

	tmpBuf := &audio.IntBuffer{
		Data:   make([]int, 8192),
		Format: buf.Format,
	}

	for {
		n, err := decoder.PCMBuffer(tmpBuf)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read PCM: %w", err)
		}
		if n == 0 {
			break
		}
		buf.Data = append(buf.Data, tmpBuf.Data[:n]...)
	}

	// Нормализуем к 16-bit
	samples := make([]int16, len(buf.Data))
	var maxVal float64 = 32768
	if bitDepth == 24 {
		maxVal = 8388608
	} else if bitDepth == 32 {
		maxVal = 2147483648
	}

	for i, s := range buf.Data {
		normalized := float64(s) / maxVal * 32767
		if normalized > 32767 {
			normalized = 32767
		} else if normalized < -32768 {
			normalized = -32768
		}
		samples[i] = int16(normalized)
	}

	return &AudioData{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   16,
	}, nil
}

// decodeFLAC декодирует FLAC файл
func decodeFLAC(r io.Reader) (*AudioData, error) {
	stream, err := flac.New(r)
	if err != nil {
		return nil, fmt.Errorf("open FLAC: %w", err)
	}
	defer stream.Close()

	sampleRate := int(stream.Info.SampleRate)
	channels := int(stream.Info.NChannels)
	bitsPerSample := int(stream.Info.BitsPerSample)

	var samples []int16

	for {
		frame, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse frame: %w", err)
		}

		nSamples := int(frame.Subframes[0].NSamples)

		for i := 0; i < nSamples; i++ {
			for ch := 0; ch < channels; ch++ {
				sample := frame.Subframes[ch].Samples[i]

				var normalized int16
				switch bitsPerSample {
				case 8:
					normalized = int16(sample << 8)
				case 16:
					normalized = int16(sample)
				case 24:
					normalized = int16(sample >> 8)
				case 32:
					normalized = int16(sample >> 16)
				default:
					normalized = int16(sample)
				}
				samples = append(samples, normalized)
			}
		}
	}

	return &AudioData{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   16,
	}, nil
}

// encodeWAV кодирует в WAV формат
func encodeWAV(w io.Writer, data *AudioData) error {
	dataSize := len(data.Samples) * 2
	fileSize := 36 + dataSize
	byteRate := data.SampleRate * data.Channels * 2
	blockAlign := data.Channels * 2

	// RIFF header
	w.Write([]byte("RIFF"))
	binary.Write(w, binary.LittleEndian, uint32(fileSize))
	w.Write([]byte("WAVE"))

	// fmt chunk
	w.Write([]byte("fmt "))
	binary.Write(w, binary.LittleEndian, uint32(16))        // chunk size
	binary.Write(w, binary.LittleEndian, uint16(1))         // PCM
	binary.Write(w, binary.LittleEndian, uint16(data.Channels))
	binary.Write(w, binary.LittleEndian, uint32(data.SampleRate))
	binary.Write(w, binary.LittleEndian, uint32(byteRate))
	binary.Write(w, binary.LittleEndian, uint16(blockAlign))
	binary.Write(w, binary.LittleEndian, uint16(16))        // bits per sample

	// data chunk
	w.Write([]byte("data"))
	binary.Write(w, binary.LittleEndian, uint32(dataSize))

	for _, s := range data.Samples {
		binary.Write(w, binary.LittleEndian, s)
	}

	return nil
}

// Duration возвращает длительность аудио
func (a *AudioData) Duration() float64 {
	totalSamples := len(a.Samples) / a.Channels
	return float64(totalSamples) / float64(a.SampleRate)
}

// DurationString возвращает длительность в формате MM:SS
func (a *AudioData) DurationString() string {
	dur := a.Duration()
	minutes := int(dur) / 60
	seconds := int(dur) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
