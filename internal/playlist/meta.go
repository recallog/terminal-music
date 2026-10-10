package playlist

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dhowden/tag"
)

// Meta is the metadata found for a file.
type Meta struct {
	Path     string
	Title    string
	Artist   string
	Album    string
	Duration float64
}

var ffprobePath, _ = exec.LookPath("ffprobe")

// HasFFprobe reports whether ffprobe is available to read durations.
func HasFFprobe() bool { return ffprobePath != "" }

// Probe reads tags and duration of a local file. It uses ffprobe when
// available (tags and duration), and falls back to reading tags only.
func Probe(path string) Meta {
	m := Meta{Path: path}
	if ffprobePath != "" {
		if probeFFprobe(&m) {
			return m
		}
	}
	probeTags(&m)
	return m
}

func probeFFprobe(m *Meta) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffprobePath,
		"-v", "quiet", "-print_format", "json",
		"-show_format", "-show_streams", "-select_streams", "a:0",
		m.Path).Output()
	if err != nil {
		return false
	}
	var res struct {
		Format struct {
			Duration string            `json:"duration"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			Tags map[string]string `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return false
	}
	m.Duration, _ = strconv.ParseFloat(res.Format.Duration, 64)
	tags := map[string]string{}
	// Ogg/Opus files carry their tags on the stream, others on the format.
	for _, s := range res.Streams {
		for k, v := range s.Tags {
			tags[strings.ToLower(k)] = v
		}
	}
	for k, v := range res.Format.Tags {
		tags[strings.ToLower(k)] = v
	}
	m.Title = strings.TrimSpace(tags["title"])
	m.Artist = strings.TrimSpace(tags["artist"])
	if m.Artist == "" {
		m.Artist = strings.TrimSpace(tags["album_artist"])
	}
	m.Album = strings.TrimSpace(tags["album"])
	return true
}

func probeTags(m *Meta) {
	f, err := os.Open(m.Path)
	if err != nil {
		return
	}
	defer f.Close()
	md, err := tag.ReadFrom(f)
	if err != nil {
		return
	}
	m.Title = strings.TrimSpace(md.Title())
	m.Artist = strings.TrimSpace(md.Artist())
	m.Album = strings.TrimSpace(md.Album())
}

// Apply copies the metadata into the track.
func (m Meta) Apply(t *Track) {
	t.Probed = true
	if m.Title != "" {
		t.Title = m.Title
		t.Artist = m.Artist
	}
	if m.Album != "" {
		t.Album = m.Album
	}
	if m.Duration > 0 {
		t.Duration = m.Duration
	}
}
