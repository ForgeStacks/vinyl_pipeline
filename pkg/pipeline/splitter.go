package pipeline

import (
	"fmt"
	"time"
)

// Track представляет один извлечённый трек
type Track struct {
	Index       int
	StartSample int
	EndSample   int
	Duration    time.Duration
	Samples     []int16 // данные трека (после извлечения)
}

// pipeline разбивает аудио на треки
type pipeline struct {
	cfg        Config
	detector   *Detector
	sampleRate int
	channels   int
}

// New создаёт сплиттер с заданной конфигурацией
func New(sampleRate, channels int, cfg Config) *pipeline {
	return &pipeline{
		cfg:        cfg,
		detector:   NewDetector(sampleRate, channels, cfg),
		sampleRate: sampleRate,
		channels:   channels,
	}
}

// Split анализирует аудио и возвращает список треков
// Сами данные треков не копируются до вызова Extract
func (s *pipeline) Split(samples []int16) ([]Track, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("empty audio data")
	}

	// Находим участки тишины
	silenceRegions := s.detector.DetectSilence(samples)

	// Минимальная длина трека в сэмплах
	minTrackSamples := s.durationToSamples(s.cfg.MinTrackDuration)

	var tracks []Track
	trackStart := 0
	trackIndex := 1
	totalSamples := len(samples) / s.channels

	for _, silence := range silenceRegions {
		// Точка разреза — середина тишины
		cutPoint := (silence.StartSample + silence.EndSample) / 2
		trackLength := cutPoint - trackStart

		if trackLength >= minTrackSamples {
			tracks = append(tracks, Track{
				Index:       trackIndex,
				StartSample: trackStart,
				EndSample:   cutPoint,
				Duration:    s.samplesToDuration(trackLength),
			})
			trackIndex++
		}

		trackStart = cutPoint
	}

	// Последний трек (от последней тишины до конца)
	lastTrackLength := totalSamples - trackStart
	if lastTrackLength >= minTrackSamples {
		tracks = append(tracks, Track{
			Index:       trackIndex,
			StartSample: trackStart,
			EndSample:   totalSamples,
			Duration:    s.samplesToDuration(lastTrackLength),
		})
	}

	// Если тишины не найдено — весь файл один трек
	if len(tracks) == 0 && totalSamples >= minTrackSamples {
		tracks = append(tracks, Track{
			Index:       1,
			StartSample: 0,
			EndSample:   totalSamples,
			Duration:    s.samplesToDuration(totalSamples),
		})
	}

	return tracks, nil
}

// Extract извлекает данные трека с применением fade и padding
func (s *pipeline) Extract(samples []int16, track Track) []int16 {
	// Конвертируем параметры в сэмплы
	padStart := s.msToSamples(s.cfg.PadStartMs)
	padEnd := s.msToSamples(s.cfg.PadEndMs)
	fadeIn := s.msToSamples(s.cfg.FadeInMs)
	fadeOut := s.msToSamples(s.cfg.FadeOutMs)

	// Расширяем границы с учётом padding
	start := track.StartSample - padStart
	end := track.EndSample + padEnd

	totalSamples := len(samples) / s.channels
	if start < 0 {
		start = 0
	}
	if end > totalSamples {
		end = totalSamples
	}

	// Извлекаем данные (интерливленные)
	startIdx := start * s.channels
	endIdx := end * s.channels
	extracted := make([]int16, endIdx-startIdx)
	copy(extracted, samples[startIdx:endIdx])

	// Применяем fade in
	s.applyFadeIn(extracted, fadeIn)

	// Применяем fade out
	s.applyFadeOut(extracted, fadeOut)

	return extracted
}

// applyFadeIn применяет плавное нарастание к началу
func (s *pipeline) applyFadeIn(samples []int16, fadeSamples int) {
	fadeLen := fadeSamples * s.channels
	if fadeLen > len(samples) {
		fadeLen = len(samples)
	}

	for i := 0; i < fadeLen; i++ {
		// Линейный fade: 0 → 1
		factor := float64(i) / float64(fadeLen)
		samples[i] = int16(float64(samples[i]) * factor)
	}
}

// applyFadeOut применяет плавное затухание к концу
func (s *pipeline) applyFadeOut(samples []int16, fadeSamples int) {
	fadeLen := fadeSamples * s.channels
	if fadeLen > len(samples) {
		fadeLen = len(samples)
	}

	startIdx := len(samples) - fadeLen
	for i := 0; i < fadeLen; i++ {
		// Линейный fade: 1 → 0
		factor := 1.0 - float64(i)/float64(fadeLen)
		samples[startIdx+i] = int16(float64(samples[startIdx+i]) * factor)
	}
}

// GetSilenceRegions возвращает найденные участки тишины (для отладки/визуализации)
func (s *pipeline) GetSilenceRegions(samples []int16) []SilenceRegion {
	return s.detector.DetectSilence(samples)
}

// durationToSamples преобразует длительность в количество сэмплов
func (s *pipeline) durationToSamples(dur time.Duration) int {
	return int(dur.Seconds() * float64(s.sampleRate))
}

// SamplesToDuration преобразует количество сэмплов в длительность (публичный)
func (s *pipeline) SamplesToDuration(samples int) time.Duration {
	seconds := float64(samples) / float64(s.sampleRate)
	return time.Duration(seconds * float64(time.Second))
}

// samplesToDuration преобразует количество сэмплов в длительность
func (s *pipeline) samplesToDuration(samples int) time.Duration {
	seconds := float64(samples) / float64(s.sampleRate)
	return time.Duration(seconds * float64(time.Second))
}

// msToSamples преобразует миллисекунды в сэмплы
func (s *pipeline) msToSamples(ms int) int {
	return s.sampleRate * ms / 1000
}

// FormatDuration форматирует длительность как MM:SS
func FormatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// FormatTimestamp форматирует позицию как HH:MM:SS
func FormatTimestamp(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}
