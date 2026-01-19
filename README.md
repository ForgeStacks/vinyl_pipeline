# vinyl-pipeline

Split vinyl recordings into separate tracks by detecting silence.

Pure Go, no FFmpeg, single binary.

## Features

- 🎵 **Automatic track detection** — finds silence between tracks
- 📁 **WAV & FLAC input** — reads both formats
- 🎚️ **Configurable thresholds** — adjust sensitivity for your recordings  
- ⚡ **Fast** — processes faster than real-time
- 🔧 **Presets** — vinyl, audiobook, live recordings
- 💾 **Zero dependencies** — single static binary

## Installation

```bash
go install github.com/formeo/vinyl-pipeline/cmd/vinsplit@latest
```

Or build from source:

```bash
git clone https://github.com/formeo/vinyl-pipeline
cd vinyl-pipeline
go build -o vinsplit ./cmd/vinsplit
```

## Usage

```bash
# Basic usage - split vinyl recording
vinsplit vinyl_side_a.wav

# Adjust sensitivity
vinsplit -db -38 -silence 2s album.flac

# Use preset for audiobooks
vinsplit -preset audiobook recording.wav

# Preview without writing files
vinsplit -dry-run -v vinyl.wav

# Custom output directory and prefix
vinsplit -out ./my_album -prefix "song" vinyl.wav
```

## Options

| Option | Default | Description |
|--------|---------|-------------|
| `-db` | -40 | Silence threshold in dB (-60 to -20) |
| `-silence` | 1.5s | Minimum silence duration between tracks |
| `-min-track` | 30s | Minimum track duration |
| `-out` | ./tracks_* | Output directory |
| `-prefix` | track | Output filename prefix |
| `-format` | wav | Output format (wav) |
| `-preset` | - | Preset: vinyl, audiobook, live |
| `-dry-run` | false | Analyze only, don't write files |
| `-v` | false | Verbose output |

## Presets

| Preset | Threshold | Min Silence | Min Track | Use case |
|--------|-----------|-------------|-----------|----------|
| `vinyl` | -40 dB | 1.5s | 30s | Standard vinyl records |
| `audiobook` | -45 dB | 3s | 60s | Audiobook chapters |
| `live` | -35 dB | 2s | 45s | Live concert recordings |

## Example output

```
$ vinsplit -v vinyl_side_a.flac

Loading: vinyl_side_a.flac
  Format: 44100 Hz, 2 ch, 22:34
  Samples: 59734560
  Config: threshold=-40dB, silence=1.5s, min_track=30s

Analyzing...

Detected 6 track(s):
--------------------------------------------------
  Track 01: 0:00 - 3:42 (3:42)
  Track 02: 3:44 - 7:15 (3:31)
  Track 03: 7:17 - 10:58 (3:41)
  Track 04: 11:00 - 14:22 (3:22)
  Track 05: 14:24 - 18:45 (4:21)
  Track 06: 18:47 - 22:34 (3:47)
--------------------------------------------------
Total: 22:24

Silence regions found: 5
  1. 3:42 (2.1s)
  2. 7:15 (1.8s)
  3. 10:58 (2.3s)
  4. 14:22 (1.6s)
  5. 18:45 (2.0s)

Saving to ./tracks_vinyl_side_a/
  ✓ track_01.wav (39.2 MB)
  ✓ track_02.wav (37.1 MB)
  ✓ track_03.wav (38.8 MB)
  ✓ track_04.wav (35.5 MB)
  ✓ track_05.wav (46.0 MB)
  ✓ track_06.wav (40.0 MB)

Done in 1.847s
```

## Algorithm

1. **Sliding window RMS** — compute energy for each window
2. **Threshold comparison** — RMS < threshold = silence
3. **Merge regions** — silence > min_duration = track boundary
4. **Filter tracks** — discard tracks < min_track_duration
5. **Extract with fade** — apply fade in/out at boundaries

```
Audio signal:
▁▃▅▇█▇▅▃▁▁▁▁▁▁▁▃▅▇███▇▅▃▁▁▁▁▁▁▁▁▃▅▇█▇▅▃▁
          ↑              ↑
       silence        silence
    (track boundary)
```

## Part of audiotools.dev

This tool is part of the [audiotools.dev](https://audiotools.dev) ecosystem:

- [go-audio-converter](https://github.com/formeo/go-audio-converter) — Convert between audio formats
- [audiobook-cleaner](https://github.com/formeo/audiobook-cleaner) — AI-powered noise removal
- [music_recognition](https://github.com/formeo/music_recognition) — Bulk music identification

## License

MIT
