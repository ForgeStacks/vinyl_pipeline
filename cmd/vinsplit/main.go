package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/formeo/vinyl-pipeline/internal/audio"
	"github.com/formeo/vinyl-pipeline/pkg/pipeline"
)

var version = "0.1.0"

func main() {
	// Флаги
	threshold := flag.Float64("db", -40, "silence threshold in dB (-60 to -20)")
	minSilence := flag.Duration("silence", 1500*time.Millisecond, "minimum silence duration between tracks")
	minTrack := flag.Duration("min-track", 30*time.Second, "minimum track duration")
	outDir := flag.String("out", "", "output directory (default: ./tracks_<filename>)")
	prefix := flag.String("prefix", "track", "output filename prefix")
	format := flag.String("format", "wav", "output format: wav (flac coming soon)")
	preset := flag.String("preset", "", "preset: vinyl, audiobook, live")
	dryRun := flag.Bool("dry-run", false, "analyze only, don't write files")
	showVersion := flag.Bool("version", false, "show version")
	verbose := flag.Bool("v", false, "verbose output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `vinyl-pipeline v%s - Split vinyl recordings into separate tracks

Usage:
  vinsplit [options] <input.wav|input.flac>

Options:
`, version)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  vinsplit vinyl_side_a.wav
  vinsplit -db -38 -silence 2s album.flac
  vinsplit -preset audiobook recording.wav
  vinsplit -dry-run -v vinyl.wav

Presets:
  vinyl     - standard vinyl: -40dB, 1.5s silence, 30s min track
  audiobook - audiobook chapters: -45dB, 3s silence, 60s min track  
  live      - live recordings: -35dB, 2s silence, 45s min track
`)
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("vinyl-pipeline v%s\n", version)
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	inputPath := flag.Arg(0)

	// Проверяем входной файл
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", inputPath)
		os.Exit(1)
	}

	// Загружаем конфигурацию
	cfg := loadConfig(*preset, *threshold, *minSilence, *minTrack)

	// Выходная директория
	outputDir := *outDir
	if outputDir == "" {
		baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		outputDir = fmt.Sprintf("./tracks_%s", baseName)
	}

	// Запускаем обработку
	if err := run(inputPath, outputDir, *prefix, *format, cfg, *dryRun, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(preset string, threshold float64, minSilence, minTrack time.Duration) pipeline.Config {
	var cfg pipeline.Config

	switch preset {
	case "audiobook":
		cfg = pipeline.ConfigForAudiobook()
	case "live":
		cfg = pipeline.ConfigForLiveRecording()
	case "vinyl", "":
		cfg = pipeline.DefaultConfig()
	default:
		fmt.Fprintf(os.Stderr, "Warning: unknown preset '%s', using default\n", preset)
		cfg = pipeline.DefaultConfig()
	}

	// Переопределяем значения из флагов (если отличаются от дефолтных)
	if threshold != -40 {
		cfg.ThresholdDB = threshold
	}
	if minSilence != 1500*time.Millisecond {
		cfg.MinSilenceDuration = minSilence
	}
	if minTrack != 30*time.Second {
		cfg.MinTrackDuration = minTrack
	}

	return cfg
}

func run(inputPath, outputDir, prefix, format string, cfg pipeline.Config, dryRun, verbose bool) error {
	fmt.Printf("Loading: %s\n", inputPath)
	startTime := time.Now()

	// Читаем аудио
	audioData, err := audio.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read audio: %w", err)
	}

	fmt.Printf("  Format: %d Hz, %d ch, %s\n",
		audioData.SampleRate, audioData.Channels, audioData.DurationString())

	if verbose {
		fmt.Printf("  Samples: %d\n", len(audioData.Samples))
		fmt.Printf("  Config: threshold=%.0fdB, silence=%v, min_track=%v\n",
			cfg.ThresholdDB, cfg.MinSilenceDuration, cfg.MinTrackDuration)
	}

	// Анализируем
	fmt.Println("\nAnalyzing...")
	spl := pipeline.New(audioData.SampleRate, audioData.Channels, cfg)

	tracks, err := spl.Split(audioData.Samples)
	if err != nil {
		return fmt.Errorf("split: %w", err)
	}

	if len(tracks) == 0 {
		fmt.Println("No tracks detected. Try adjusting -db or -silence parameters.")
		return nil
	}

	// Выводим результаты
	fmt.Printf("\nDetected %d track(s):\n", len(tracks))
	fmt.Println(strings.Repeat("-", 50))

	totalDuration := time.Duration(0)
	for _, t := range tracks {
		startTs := pipeline.FormatTimestamp(spl.SamplesToDuration(t.StartSample))
		endTs := pipeline.FormatTimestamp(spl.SamplesToDuration(t.EndSample))
		fmt.Printf("  Track %02d: %s - %s (%s)\n",
			t.Index, startTs, endTs, pipeline.FormatDuration(t.Duration))
		totalDuration += t.Duration
	}

	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("Total: %s\n", pipeline.FormatDuration(totalDuration))

	// Показываем участки тишины в verbose режиме
	if verbose {
		silences := spl.GetSilenceRegions(audioData.Samples)
		fmt.Printf("\nSilence regions found: %d\n", len(silences))
		for i, s := range silences {
			startTs := pipeline.FormatTimestamp(spl.SamplesToDuration(s.StartSample))
			fmt.Printf("  %d. %s (%.1fs)\n", i+1, startTs, s.Duration.Seconds())
		}
	}

	// Если dry-run — выходим
	if dryRun {
		fmt.Println("\nDry run - no files written.")
		return nil
	}

	// Создаём выходную директорию
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Записываем треки
	fmt.Printf("\nSaving to %s/\n", outputDir)

	for _, t := range tracks {
		// Извлекаем данные трека
		trackSamples := spl.Extract(audioData.Samples, t)

		trackData := &audio.AudioData{
			Samples:    trackSamples,
			SampleRate: audioData.SampleRate,
			Channels:   audioData.Channels,
			BitDepth:   16,
		}

		// Имя файла
		filename := fmt.Sprintf("%s_%02d.%s", prefix, t.Index, format)
		outPath := filepath.Join(outputDir, filename)

		// Записываем
		if err := audio.WriteFile(outPath, trackData); err != nil {
			return fmt.Errorf("write track %d: %w", t.Index, err)
		}

		// Размер файла
		info, _ := os.Stat(outPath)
		sizeMB := float64(info.Size()) / 1024 / 1024

		fmt.Printf("  ✓ %s (%.1f MB)\n", filename, sizeMB)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\nDone in %v\n", elapsed.Round(time.Millisecond))

	return nil
}


