// Package viz captures the audio being played and turns it into spectrum
// bars and oscilloscope samples.
package viz

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os/exec"
	"sync"
	"syscall"
)

// SampleRate is the capture sample rate in Hz.
const SampleRate = 44100

const ringSize = 8192

// Capture records the default output's monitor (what the speakers play)
// through parec, which PulseAudio and PipeWire both provide.
type Capture struct {
	cmd *exec.Cmd

	mu   sync.Mutex
	ring [ringSize]float32
	pos  int
	err  error
}

// StartCapture launches the recorder.
func StartCapture() (*Capture, error) {
	path, err := exec.LookPath("parec")
	if err != nil {
		return nil, errors.New("parec not found (install pulseaudio-utils or pipewire-pulse)")
	}
	cmd := exec.Command(path,
		"--raw", "--format=float32le", "--channels=1", "--rate=44100",
		"--latency-msec=20", "--device=@DEFAULT_MONITOR@",
		"--client-name=terminal-music", "--stream-name=visualizer")
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &Capture{cmd: cmd}
	go c.read(out)
	return c, nil
}

func (c *Capture) read(r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := io.ReadFull(r, buf)
		c.mu.Lock()
		for i := 0; i+4 <= n; i += 4 {
			c.ring[c.pos] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i:]))
			c.pos = (c.pos + 1) % ringSize
		}
		if err != nil {
			c.err = errors.New("audio capture stopped")
		}
		c.mu.Unlock()
		if err != nil {
			_ = c.cmd.Wait()
			return
		}
	}
}

// Latest copies the most recent len(dst) samples into dst.
func (c *Capture) Latest(dst []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := min(len(dst), ringSize)
	start := (c.pos - n + ringSize) % ringSize
	for i := 0; i < n; i++ {
		dst[i] = c.ring[(start+i)%ringSize]
	}
}

// Err returns the capture failure, if any.
func (c *Capture) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close stops the recorder.
func (c *Capture) Close() {
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}
