package pipeline

import (
	"testing"
	"time"
)

func TestDetectSilence_WithClearGaps(t *testing.T) {
	sampleRate := 44100
	channels := 1
	cfg := DefaultConfig()
	cfg.MinSilenceDuration = 100 * time.Millisecond // короткая тишина для теста
	cfg.WindowSize = 256

	detector := NewDetector(sampleRate, channels, cfg)

	// Создаём тестовый сигнал: громко → тихо → громко
	// 1 секунда громкого, 0.5 секунды тишины, 1 секунда громкого
	samples := make([]int16, sampleRate*3) // 3 секунды

	// Первая секунда — громкий сигнал (синусоида ~50% громкости)
	for i := 0; i < sampleRate; i++ {
		samples[i] = 16000
	}

	// 0.5 секунды — тишина
	for i := sampleRate; i < sampleRate+sampleRate/2; i++ {
		samples[i] = 10 // почти ноль
	}

	// Последние 1.5 секунды — снова громко
	for i := sampleRate + sampleRate/2; i < len(samples); i++ {
		samples[i] = 16000
	}

	silences := detector.DetectSilence(samples)

	if len(silences) != 1 {
		t.Errorf("Expected 1 silence region, got %d", len(silences))
		return
	}

	// Проверяем что тишина обнаружена примерно в середине
	expectedStart := sampleRate
	expectedEnd := sampleRate + sampleRate/2

	if silences[0].StartSample < expectedStart-sampleRate/10 ||
		silences[0].StartSample > expectedStart+sampleRate/10 {
		t.Errorf("Silence start %d not near expected %d", silences[0].StartSample, expectedStart)
	}

	if silences[0].EndSample < expectedEnd-sampleRate/10 ||
		silences[0].EndSample > expectedEnd+sampleRate/10 {
		t.Errorf("Silence end %d not near expected %d", silences[0].EndSample, expectedEnd)
	}
}

func TestDetectSilence_NoSilence(t *testing.T) {
	sampleRate := 44100
	channels := 1
	cfg := DefaultConfig()

	detector := NewDetector(sampleRate, channels, cfg)

	// Постоянный громкий сигнал
	samples := make([]int16, sampleRate*5)
	for i := range samples {
		samples[i] = 20000
	}

	silences := detector.DetectSilence(samples)

	if len(silences) != 0 {
		t.Errorf("Expected 0 silence regions, got %d", len(silences))
	}
}

func TestDetectSilence_AllSilence(t *testing.T) {
	sampleRate := 44100
	channels := 1
	cfg := DefaultConfig()
	cfg.MinSilenceDuration = 500 * time.Millisecond

	detector := NewDetector(sampleRate, channels, cfg)

	// Полная тишина
	samples := make([]int16, sampleRate*3) // 3 секунды тишины

	silences := detector.DetectSilence(samples)

	if len(silences) != 1 {
		t.Errorf("Expected 1 silence region covering whole file, got %d", len(silences))
	}
}

func TestSplit_MultipleTracks(t *testing.T) {
	sampleRate := 44100
	channels := 1
	cfg := DefaultConfig()
	cfg.MinSilenceDuration = 100 * time.Millisecond
	cfg.MinTrackDuration = 500 * time.Millisecond

	spl := New(sampleRate, channels, cfg)

	// 3 трека по 1 секунде с паузами по 0.2 секунды
	totalSamples := sampleRate*3 + sampleRate/5*2
	samples := make([]int16, totalSamples)

	pos := 0

	// Трек 1
	for i := 0; i < sampleRate; i++ {
		samples[pos] = 15000
		pos++
	}

	// Пауза 1
	for i := 0; i < sampleRate/5; i++ {
		samples[pos] = 5
		pos++
	}

	// Трек 2
	for i := 0; i < sampleRate; i++ {
		samples[pos] = 15000
		pos++
	}

	// Пауза 2
	for i := 0; i < sampleRate/5; i++ {
		samples[pos] = 5
		pos++
	}

	// Трек 3
	for i := 0; i < sampleRate; i++ {
		samples[pos] = 15000
		pos++
	}

	tracks, err := spl.Split(samples)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(tracks) != 3 {
		t.Errorf("Expected 3 tracks, got %d", len(tracks))
	}

	// Проверяем индексы
	for i, track := range tracks {
		if track.Index != i+1 {
			t.Errorf("Track %d has wrong index %d", i, track.Index)
		}
	}
}

func TestCalculateRMS(t *testing.T) {
	detector := NewDetector(44100, 1, DefaultConfig())

	// Тишина
	silence := make([]int16, 100)
	rms := detector.calculateRMS(silence)
	if rms != 0 {
		t.Errorf("RMS of silence should be 0, got %f", rms)
	}

	// Постоянный сигнал
	constant := make([]int16, 100)
	for i := range constant {
		constant[i] = 1000
	}
	rms = detector.calculateRMS(constant)
	if rms != 1000 {
		t.Errorf("RMS of constant 1000 should be 1000, got %f", rms)
	}
}

func TestDbConversion(t *testing.T) {
	detector := NewDetector(44100, 1, DefaultConfig())

	// -6 dB ≈ 50% амплитуды
	linear := detector.dbToLinear(-6)
	expected := 32768.0 * 0.501 // примерно
	if linear < expected*0.9 || linear > expected*1.1 {
		t.Errorf("-6dB should be ~%f, got %f", expected, linear)
	}

	// 0 dB = 100%
	linear = detector.dbToLinear(0)
	if linear != 32768 {
		t.Errorf("0dB should be 32768, got %f", linear)
	}

	// Обратное преобразование
	db := detector.linearToDb(32768)
	if db != 0 {
		t.Errorf("linearToDb(32768) should be 0, got %f", db)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		dur      time.Duration
		expected string
	}{
		{30 * time.Second, "0:30"},
		{90 * time.Second, "1:30"},
		{3*time.Minute + 45*time.Second, "3:45"},
		{10*time.Minute + 5*time.Second, "10:05"},
	}

	for _, tt := range tests {
		result := FormatDuration(tt.dur)
		if result != tt.expected {
			t.Errorf("FormatDuration(%v) = %s, want %s", tt.dur, result, tt.expected)
		}
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		dur      time.Duration
		expected string
	}{
		{30 * time.Second, "0:30"},
		{90 * time.Second, "1:30"},
		{time.Hour + 5*time.Minute + 30*time.Second, "1:05:30"},
	}

	for _, tt := range tests {
		result := FormatTimestamp(tt.dur)
		if result != tt.expected {
			t.Errorf("FormatTimestamp(%v) = %s, want %s", tt.dur, result, tt.expected)
		}
	}
}
