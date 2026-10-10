# terminal-music

**A Winamp-style music player for the Linux terminal.**
Big LCD clock, spectrum analyzer, 10-band equalizer, playlist, colour themes,
and full mouse support. It's built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and plays audio
through [mpv](https://mpv.io).

```
╭─ TERMINAL-MUSIC ───────────────────────────────────────────╮
│ 1. Demo Band - Morning Signal (2:53)                       │
│ ▶  █▀█ █▀█   █▀█ ▀▀█                                       │
│    █ █ █ █ ▀ █ █  ▀█                                       │
│    ▀▀▀ ▀▀▀ ▀ ▀▀▀ ▀▀▀        ▅ ▔ ▔ ▔ ▔   ▔ ▁                │
│ 192kbps 44.1kHz mono        █ ▇ ▆ █ ▅ ▅ ▅ █   ▁            │
│ ━●─────────────────────────────────────────────────── 2:53 │
│ [|◀] [▶] [▮▮] [■] [▶|] [▲]       VOL ━━━━━━━━━━━●────  70% │
│ SHUF REP:ALL EQ PL                              ◂ winamp ▸ │
╰────────────────────────────────────────────────────────────╯
╭─ EQUALIZER ────────────────────────────────────────────────╮
│ [ON] [RESET]  ◂ Rock ▸                         PRE +0.0 dB │
│    │    │    │    │    │    │    │    │    │    │    │     │
│    │   ▄▄▄   │    │    │    │    │    │    │    │   ▄▄▄    │
│    │   ███  ███  ███  ▄▄▄   │    │   ▄▄▄  ███  ███  ███    │
│   ━━━  ━━━  ━━━  ━━━  ━━━  ━━━  ━━━  ━━━  ━━━  ━━━  ━━━    │
│    │    │    │    │    │   ▀▀▀  ▀▀▀   │    │    │    │     │
│    │    │    │    │    │    │    │    │    │    │    │     │
│    │    │    │    │    │    │    │    │    │    │    │     │
│   PRE  31   62   125  250  500  1k   2k   4k   8k   16k    │
╰────────────────────────────────────────────────────────────╯
╭─ PLAYLIST ─────────────────────────────────────────────────╮
│▶1. Demo Band - Morning Signal                         2:53 │
│ 2. Demo Band - Tape Hiss Lullaby                      3:16 │
│ 3. Demo Band - Neon Arcade                            3:39 │
│ 4. Demo Band - Low Orbit                              4:02 │
│ 5. Demo Band - Dial-Up Dreams                         4:25 │
│ 6. Demo Band - Static Bloom                           4:48 │
│ [+ADD] [-DEL] [CLEAR] [FIND]              6 tracks · 23:03 │
╰────────────────────────────────────────────────────────────╯
```

## Features

- **Main window**: a big LCD-style clock (elapsed or remaining time), a
  scrolling title, bitrate and format, seek bar, transport buttons and
  volume.
- **Visualizer**: a spectrum analyzer with falling peaks, or an
  oscilloscope. It reacts to the sound actually coming out of your speakers.
- **Equalizer**: 10 bands plus preamp, ±12 dB, with presets (Rock, Pop,
  Jazz, Classical, Dance, Bass/Treble boost, Vocal, Loudness).
- **Playlist**: shows tags and durations. Supports shuffle, repeat (all or
  one), search, reordering, gapless playback, M3U/M3U8/PLS playlists and
  internet radio URLs.
- **Keyboard and mouse**: Winamp keys and vim-style keys. Click buttons,
  drag sliders, scroll with the wheel, double-click a track to play it, or
  drop files onto the window to add them.
- **Adapts to the window**: designed for a tall window. On a wide window the
  playlist moves to the side, and panels that don't fit are hidden.
- **10 colour themes**, and you can add your own.
- **Remembers** your playlist, volume, equalizer, theme and options
  between runs.

## Requirements

| Tool | Needed for | Package (Arch / Debian, Ubuntu) |
|---|---|---|
| [mpv](https://mpv.io) | playback (**required**) | `mpv` / `mpv` |
| `ffprobe` | track durations and tags | `ffmpeg` / `ffmpeg` |
| `parec` | the visualizer | `libpulse` / `pulseaudio-utils` |

Without `ffprobe`, tags are still read, but a track's duration only appears
once it has played. Without `parec`, everything except the visualizer works.
`parec` works with both PipeWire (through `pipewire-pulse`) and PulseAudio.
At startup, the player tells you which of these tools are missing.

A terminal with true colour and mouse support is recommended: kitty,
Alacritty, WezTerm, foot, GNOME Terminal, Konsole, and others.

## Installation

You need [Go](https://go.dev/dl/) 1.26 or newer.

```sh
go install github.com/recallog/terminal-music@latest
```

This puts `terminal-music` in `~/go/bin` (or `$GOBIN`). Make sure that folder
is on your `PATH`.

Or build it from a clone:

```sh
git clone https://github.com/recallog/terminal-music.git
cd terminal-music
go build -o terminal-music .
install -Dm755 terminal-music ~/.local/bin/terminal-music   # optional
```

## Usage

```sh
terminal-music ~/Music                     # folders, files, playlists...
terminal-music album/ track.flac mix.m3u   # ...in any combination
terminal-music https://example.com/stream  # internet radio
terminal-music                             # restore the last playlist
```

| Option | Description |
|---|---|
| `-theme NAME` | start with a theme |
| `-list-themes` | list the available themes |
| `-fresh` | start with an empty playlist |
| `-no-viz` | don't record audio for the visualizer |
| `-mpv PATH` | use a specific mpv binary |
| `-mpv-args "..."` | extra mpv options, e.g. `"--replaygain=track"` |

### Keyboard

| Key | Action |
|---|---|
| `space` / `c` | play / pause |
| `x` / `v` | play (restart) / stop |
| `z` / `b` | previous / next track |
| `←` `→` (`shift`: 30s) | seek 5s |
| `+` `-` / `m` | volume / mute |
| `s` / `r` | shuffle / repeat (off, all, one) |
| `↑` `↓` `pgup` `pgdn` `g` `G` | move in the playlist |
| `enter` | play selected track |
| `K` / `J` | move track up / down |
| `d` / `D` | remove track / clear playlist |
| `a` | add a file, folder, playlist or URL (`tab` completes paths) |
| `/` | find a track (`↑` `↓` for other matches, `enter` plays) |
| `.` | jump to the playing track |
| `tab` | switch focus between playlist and equalizer |
| `e` / `p` | show or hide the equalizer / playlist |
| `f` | visualizer: spectrum, scope, off |
| `t` / `T` | next / previous theme |
| `ctrl+r` | elapsed / remaining time |
| `?` | help |
| `q` | quit |

When the equalizer has focus (press `tab`):

| Key | Action |
|---|---|
| `←` `→` / `↑` `↓` | select band / change gain |
| `0` | reset the selected band |
| `[` `]` | previous / next preset |
| `R` | reset all bands |
| `enter` | turn the equalizer on or off |

### Mouse

- Click the buttons and the toggles (**SHUF**, **REP**, **EQ**, **PL**,
  theme arrows).
- Click or drag the seek bar, the volume and the equalizer sliders.
- Use the wheel over the playlist (scroll), sliders (adjust) or the main
  window (volume).
- Double-click a track to play it.
- Click the clock to switch between elapsed and remaining time, and click
  the visualizer to change its mode.
- Drag files from your file manager onto the window to add them.

## Themes

Built-in themes: `winamp`, `terminal` (uses your terminal's own palette),
`dracula`, `nord`, `gruvbox`, `catppuccin`, `tokyo-night`, `synthwave`,
`amber`, `solarized-light`.

To make your own, put a JSON file in `~/.config/terminal-music/themes/`. The
file name becomes the theme name. List only the colours you want to change;
the rest come from the `base` theme (`winamp` if not set).

```json
{
  "base": "nord",
  "display": "#ff9e64",
  "barLow": "#9ece6a",
  "barMid": "#e0af68",
  "barHigh": "#f7768e"
}
```

Colours are `#rrggbb` or ANSI numbers (`"0"` to `"255"`). Set
`"background": ""` to keep your terminal's background.

| Key | Used for |
|---|---|
| `background`, `foreground` | window background and text |
| `muted` | secondary text, inactive toggles |
| `border`, `borderFocus` | panel borders |
| `title` | panel titles |
| `display` | clock, song title, sliders |
| `accent` | active buttons, highlights |
| `playing` | the current track in the playlist |
| `selectionBg`, `selectionFg` | playlist cursor |
| `barLow`, `barMid`, `barHigh`, `peak` | visualizer gradient and peaks |

## Files

| Path | Contents |
|---|---|
| `~/.config/terminal-music/config.json` | settings |
| `~/.config/terminal-music/playlist.m3u` | the playlist, saved when you quit |
| `~/.config/terminal-music/themes/*.json` | your themes |

## How it works

- **Playback**: terminal-music starts mpv in the background (no window, no
  video) and controls it through mpv's
  [JSON IPC](https://mpv.io/manual/master/#json-ipc) socket. mpv is always
  given the next track in advance, which is what makes playback gapless.
  mpv also loads your own `mpv.conf` and scripts, so a script like
  [mpv-mpris](https://github.com/hoyon/mpv-mpris) gives you media keys and
  `playerctl` support.
- **Equalizer**: implemented with mpv's audio filters (ffmpeg's `equalizer`
  filter). It sets mpv's `af` option, so it replaces any audio filters set in
  your `mpv.conf`.
- **Visualizer**: `parec` records the monitor of your default output, which
  is the sound going to your speakers, and a small FFT turns it into bars.
  Since it records everything you hear, other applications' sound shows up
  too.

## Development

```sh
go test ./...   # the mpv and UI tests need mpv; they play with audio off
go vet ./...
```

```
main.go              flags, startup, saving on exit
internal/mpv         mpv process and IPC client
internal/playlist    tracks, play order, M3U/PLS, metadata lookup
internal/viz         audio capture, FFT spectrum, oscilloscope
internal/theme       built-in and user themes
internal/config      settings file
internal/ui          Bubble Tea model: layout, panels, keyboard, mouse
```

## License

[MIT](LICENSE)
