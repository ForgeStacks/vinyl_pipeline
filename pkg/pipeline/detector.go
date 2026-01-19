package pipeline

import (
	"math"
	"time"
)

// SilenceRegion представляет участок тишины
type SilenceRegion struct {
	StartSample int
	EndSample   int
	Duration    time.Duration
}

// Detector анализирует аудио и находит участки тишины
type Detector struct {
	cfg        Config
	sampleRate int
	channels   int
}

// NewDetector создаёт детектор с заданной конфигурацией
func NewDetector(sampleRate, channels int, cfg Config) *Detector {
	return &Detector{
		cfg:        cfg,
		sampleRate: sampleRate,
		channels:   channels,
	}
}

// DetectSilence находит все участки тишины в аудио
// samples - интерливленные сэмплы (L R L R для стерео)
func (d *Detector) DetectSilence(samples []int16) []SilenceRegion {
	threshold := d.dbToLinear(d.cfg.ThresholdDB)
	minSilenceSamples := d.durationToSamples(d.cfg.MinSilenceDuration)
	windowSize := d.cfg.WindowSize * d.channels

	var regions []SilenceRegion
	silenceStart := -1

	// Проходим по аудио скользящим окном
	for i := 0; i < len(samples)-windowSize; i += windowSize {
		window := samples[i : i+windowSize]
		rms := d.calculateRMS(window)

		if rms < threshold {
			// Начало или продолжение тишины
			if silenceStart == -1 {
				silenceStart = i / d.channels
			}
		} else {
			// Конец тишины
			if silenceStart != -1 {
				silenceEnd := i / d.channels
				length := silenceEnd - silenceStart

				if length >= minSilenceSamples {
					regions = append(regions, SilenceRegion{
						StartSample: silenceStart,
						EndSample:   silenceEnd,
						Duration:    d.samplesToDuration(length),
					})
				}
				silenceStart = -1
			}
		}
	}

	// Проверяем тишину в конце файла
	if silenceStart != -1 {
		silenceEnd := len(samples) / d.channels
		length := silenceEnd - silenceStart
		if length >= minSilenceSamples {
			regions = append(regions, SilenceRegion{
				StartSample: silenceStart,
				EndSample:   silenceEnd,
				Duration:    d.samplesToDuration(length),
			})
		}
	}

	return regions
}

// AnalyzeLevel возвращает профиль громкости для визуализации
// Возвращает массив RMS значений в dB, по одному на каждый блок
func (d *Detector) AnalyzeLevel(samples []int16, blockSize int) []float64 {
	windowSize := blockSize * d.channels
	numBlocks := len(samples) / windowSize
	levels := make([]float64, numBlocks)

	for i := 0; i < numBlocks; i++ {
		start := i * windowSize
		end := start + windowSize
		if end > len(samples) {
			end = len(samples)
		}
		rms := d.calculateRMS(samples[start:end])
		levels[i] = d.linearToDb(rms)
	}

	return levels
}

// calculateRMS вычисляет RMS (Root Mean Square) энергию окна
func (d *Detector) calculateRMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}

	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(len(samples)))
}

// dbToLinear преобразует децибелы в линейное значение
// -40 дБ → ~328 (1% от 32768)
func (d *Detector) dbToLinear(db float64) float64 {
	return math.Pow(10, db/20) * 32768
}

// linearToDb преобразует линейное значение в децибелы
func (d *Detector) linearToDb(linear float64) float64 {
	if linear <= 0 {
		return -96 // минимум
	}
	return 20 * math.Log10(linear/32768)
}

// durationToSamples преобразует длительность в количество сэмплов
func (d *Detector) durationToSamples(dur time.Duration) int {
	return int(dur.Seconds() * float64(d.sampleRate))
}

// samplesToDuration преобразует количество сэмплов в длительность
func (d *Detector) samplesToDuration(samples int) time.Duration {
	seconds := float64(samples) / float64(d.sampleRate)
	return time.Duration(seconds * float64(time.Second))
}
