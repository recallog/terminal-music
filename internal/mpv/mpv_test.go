package mpv

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func tone(t *testing.T, dir, name string, freq int) string {
	t.Helper()
	p := filepath.Join(dir, name)
	out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi",
		"-i", "sine=frequency="+itoa(freq)+":duration=1", p).CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg: %v %s", err, out)
	}
	return p
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

// TestQueueAdvance checks the sequence the UI relies on for gapless play:
// load a file, clear the rest of mpv's playlist, append the next file, and
// see mpv move to it by itself.
func TestQueueAdvance(t *testing.T) {
	for _, bin := range []string{"mpv", "ffmpeg"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skip(bin + " not installed")
		}
	}
	dir := t.TempDir()
	a := tone(t, dir, "a.wav", 440)
	b := tone(t, dir, "b.wav", 880)

	c, err := Start(Options{Volume: 0, ExtraArgs: []string{"--ao=null"}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := c.Observe(1, "path"); err != nil {
		t.Fatal(err)
	}
	_ = c.Command("loadfile", a, "replace")
	_ = c.Command("playlist-clear")
	_ = c.Command("loadfile", b, "append")

	var paths []string
	timeout := time.After(10 * time.Second)
	for len(paths) < 2 {
		select {
		case ev, ok := <-c.Events:
			if !ok {
				t.Fatal("mpv exited")
			}
			if ev.Name == "command-error" {
				t.Fatalf("command error: %s", ev.Error)
			}
			if ev.Name == "property-change" && ev.Property == "path" {
				var s string
				if json.Unmarshal(ev.Data, &s) == nil && s != "" {
					paths = append(paths, s)
				}
			}
		case <-timeout:
			t.Fatalf("timeout, paths seen: %v", paths)
		}
	}
	if paths[0] != a || paths[1] != b {
		t.Fatalf("got %v, want [%s %s]", paths, a, b)
	}
}
