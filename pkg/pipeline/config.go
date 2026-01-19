package pipeline

import "time"

// Config настройки детектора и сплиттера
type Config struct {
	// ThresholdDB порог тишины в децибелах (обычно -40..-50 дБ)
	ThresholdDB float64

	// MinSilenceDuration минимальная длительность тишины между треками
	MinSilenceDuration time.Duration

	// MinTrackDuration минимальная длительность трека (фильтрует мусор)
	MinTrackDuration time.Duration

	// WindowSize размер окна для RMS анализа (в сэмплах)
	WindowSize int

	// FadeIn/FadeOut плавное нарастание/затухание на границах (мс)
	FadeInMs  int
	FadeOutMs int

	// PadStart/PadEnd добавить тишину в начало/конец трека (мс)
	PadStartMs int
	PadEndMs   int
}

// DefaultConfig возвращает настройки по умолчанию для винила
func DefaultConfig() Config {
	return Config{
		ThresholdDB:        -40,
		MinSilenceDuration: 1500 * time.Millisecond,
		MinTrackDuration:   30 * time.Second,
		WindowSize:         1024,
		FadeInMs:           50,
		FadeOutMs:          50,
		PadStartMs:         100,
		PadEndMs:           100,
	}
}

// ConfigForAudiobook настройки для аудиокниг (короткие паузы)
func ConfigForAudiobook() Config {
	return Config{
		ThresholdDB:        -45,
		MinSilenceDuration: 3 * time.Second,
		MinTrackDuration:   60 * time.Second,
		WindowSize:         2048,
		FadeInMs:           20,
		FadeOutMs:          20,
		PadStartMs:         50,
		PadEndMs:           50,
	}
}

// ConfigForLiveRecording настройки для live-записей (длинные паузы между песнями)
func ConfigForLiveRecording() Config {
	return Config{
		ThresholdDB:        -35,
		MinSilenceDuration: 2 * time.Second,
		MinTrackDuration:   45 * time.Second,
		WindowSize:         1024,
		FadeInMs:           100,
		FadeOutMs:          200,
		PadStartMs:         200,
		PadEndMs:           300,
	}
}
