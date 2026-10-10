package playlist

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".oga": true, ".opus": true,
	".m4a": true, ".m4b": true, ".aac": true, ".alac": true, ".wav": true,
	".wv": true, ".ape": true, ".mpc": true, ".wma": true, ".aif": true,
	".aiff": true, ".mka": true, ".dsf": true, ".dff": true, ".tta": true,
	".mod": true, ".xm": true, ".s3m": true, ".it": true, ".spx": true,
	".webm": true, ".mp2": true, ".ac3": true, ".dts": true,
}

// IsAudio reports whether the file name has a known audio extension.
func IsAudio(name string) bool {
	return audioExts[strings.ToLower(filepath.Ext(name))]
}

// IsPlaylist reports whether the file name is a playlist we can read.
func IsPlaylist(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".m3u", ".m3u8", ".pls":
		return true
	}
	return false
}

// Expand turns command line arguments (files, directories, playlists, URLs)
// into tracks. Errors are collected but do not stop the expansion.
func Expand(args []string) ([]*Track, []error) {
	var tracks []*Track
	var errs []error
	for _, arg := range args {
		if strings.Contains(arg, "://") {
			tracks = append(tracks, &Track{Path: arg})
			continue
		}
		abs, err := filepath.Abs(expandHome(arg))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		info, err := os.Stat(abs)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		switch {
		case info.IsDir():
			t, err := scanDir(abs)
			if err != nil {
				errs = append(errs, err)
			}
			tracks = append(tracks, t...)
		case IsPlaylist(abs):
			t, err := ReadPlaylist(abs)
			if err != nil {
				errs = append(errs, err)
			}
			tracks = append(tracks, t...)
		default:
			tracks = append(tracks, &Track{Path: abs})
		}
	}
	return tracks, errs
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}

func scanDir(dir string) ([]*Track, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() && path != dir && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if !d.IsDir() && IsAudio(d.Name()) {
			paths = append(paths, path)
		}
		return nil
	})
	slices.SortFunc(paths, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	tracks := make([]*Track, len(paths))
	for i, p := range paths {
		tracks[i] = &Track{Path: p}
	}
	return tracks, err
}

// ReadPlaylist reads an M3U/M3U8 or PLS file.
func ReadPlaylist(path string) ([]*Track, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	base := filepath.Dir(path)
	resolve := func(entry string) string {
		if strings.Contains(entry, "://") {
			if strings.HasPrefix(entry, "file://") {
				return strings.TrimPrefix(entry, "file://")
			}
			return entry
		}
		entry = expandHome(entry)
		if !filepath.IsAbs(entry) {
			entry = filepath.Join(base, entry)
		}
		return filepath.Clean(entry)
	}

	var tracks []*Track
	sc := bufio.NewScanner(f)
	if strings.EqualFold(filepath.Ext(path), ".pls") {
		files := map[int]*Track{}
		var keys []int
		get := func(n int) *Track {
			if files[n] == nil {
				files[n] = &Track{}
				keys = append(keys, n)
			}
			return files[n]
		}
		for sc.Scan() {
			key, val, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
			if !ok {
				continue
			}
			lk := strings.ToLower(key)
			for _, prefix := range []string{"file", "title", "length"} {
				if !strings.HasPrefix(lk, prefix) {
					continue
				}
				n, err := strconv.Atoi(lk[len(prefix):])
				if err != nil {
					continue
				}
				t := get(n)
				switch prefix {
				case "file":
					t.Path = resolve(val)
				case "title":
					t.Title = val
				case "length":
					if d, err := strconv.ParseFloat(val, 64); err == nil && d > 0 {
						t.Duration = d
					}
				}
			}
		}
		slices.Sort(keys)
		for _, k := range keys {
			if files[k].Path != "" {
				tracks = append(tracks, files[k])
			}
		}
		return tracks, sc.Err()
	}

	var pending *Track
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		switch {
		case line == "":
		case strings.HasPrefix(line, "#EXTINF:"):
			pending = &Track{}
			info := strings.TrimPrefix(line, "#EXTINF:")
			dur, title, _ := strings.Cut(info, ",")
			if d, err := strconv.ParseFloat(strings.Fields(dur + " ")[0], 64); err == nil && d > 0 {
				pending.Duration = d
			}
			if artist, t, ok := strings.Cut(title, " - "); ok {
				pending.Artist, pending.Title = artist, t
			} else {
				pending.Title = title
			}
		case strings.HasPrefix(line, "#"):
		default:
			t := pending
			if t == nil {
				t = &Track{}
			}
			t.Path = resolve(line)
			tracks = append(tracks, t)
			pending = nil
		}
	}
	return tracks, sc.Err()
}

// WriteM3U saves tracks as an extended M3U playlist.
func WriteM3U(path string, tracks []*Track) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, t := range tracks {
		dur := -1
		if t.Duration > 0 {
			dur = int(t.Duration + 0.5)
		}
		title := t.Title
		if t.Artist != "" && title != "" {
			title = t.Artist + " - " + title
		}
		if title != "" || dur > 0 {
			fmt.Fprintf(&b, "#EXTINF:%d,%s\n", dur, title)
		}
		b.WriteString(t.Path + "\n")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
