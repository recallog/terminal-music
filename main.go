// Command terminal-music is a Winamp-like music player for the terminal,
// using mpv for playback.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/recallog/terminal-music/internal/config"
	"github.com/recallog/terminal-music/internal/mpv"
	"github.com/recallog/terminal-music/internal/playlist"
	"github.com/recallog/terminal-music/internal/theme"
	"github.com/recallog/terminal-music/internal/ui"
	"github.com/recallog/terminal-music/internal/viz"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "terminal-music:", err)
		os.Exit(1)
	}
}

func run() error {
	themeName := flag.String("theme", "", "colour theme (see -list-themes)")
	listThemes := flag.Bool("list-themes", false, "list the available themes and exit")
	noViz := flag.Bool("no-viz", false, "disable the audio capture used by the visualizer")
	mpvBin := flag.String("mpv", "mpv", "path of the mpv binary")
	mpvArgs := flag.String("mpv-args", "", "extra mpv options, space separated (e.g. \"--replaygain=track\")")
	fresh := flag.Bool("fresh", false, "do not restore the previous playlist")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: terminal-music [options] [files, folders, playlists, URLs...]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	var warnings []string
	cfg, err := config.Load()
	if err != nil {
		warnings = append(warnings, "config: "+err.Error())
	}

	builtins := theme.Builtins()
	user, errs := theme.LoadDir(config.ThemesDir(), builtins)
	for _, e := range errs {
		warnings = append(warnings, "theme: "+e.Error())
	}
	themes := append(builtins, user...)
	if *listThemes {
		for _, t := range themes {
			fmt.Println(t.Name)
		}
		return nil
	}
	if *themeName != "" {
		found := false
		for _, t := range themes {
			if strings.EqualFold(t.Name, *themeName) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("unknown theme %q (try -list-themes)", *themeName)
		}
		cfg.Theme = *themeName
	}

	if !playlist.HasFFprobe() {
		warnings = append(warnings, "ffprobe not found (install ffmpeg): durations appear only once a track plays")
	}

	pl := playlist.New()
	args := flag.Args()
	if len(args) > 0 {
		tracks, errs := playlist.Expand(args)
		for _, e := range errs {
			warnings = append(warnings, e.Error())
		}
		pl.Add(tracks...)
	} else if !*fresh {
		if tracks, err := playlist.ReadPlaylist(config.PlaylistPath()); err == nil {
			pl.Add(tracks...)
		}
	}

	player, err := mpv.Start(mpv.Options{
		Binary:    *mpvBin,
		Volume:    cfg.Volume,
		ExtraArgs: strings.Fields(*mpvArgs),
	})
	if err != nil {
		return err
	}
	defer player.Close()

	var capture *viz.Capture
	if !*noViz {
		capture, err = viz.StartCapture()
		if err != nil {
			warnings = append(warnings, "visualizer: "+err.Error())
		} else {
			defer capture.Close()
		}
	}

	model := ui.New(ui.Options{
		Config:   cfg,
		Themes:   themes,
		MPV:      player,
		Playlist: pl,
		Analyzer: viz.NewAnalyzer(capture),
		Autoplay: len(args) > 0,
		Warnings: warnings,
	})
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return err
	}

	if err := config.Save(model.Config()); err != nil {
		fmt.Fprintln(os.Stderr, "terminal-music: saving config:", err)
	}
	if err := playlist.WriteM3U(config.PlaylistPath(), pl.Tracks); err != nil {
		fmt.Fprintln(os.Stderr, "terminal-music: saving playlist:", err)
	}
	return nil
}
