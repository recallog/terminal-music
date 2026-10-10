// Package mpv drives a headless mpv process through its JSON IPC socket.
package mpv

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Event is an asynchronous message coming from mpv: a property change, a
// playback event (end-file, file-loaded, ...) or a failed command.
type Event struct {
	Name     string          // mpv event name, or "command-error"
	Property string          // property name for "property-change"
	Data     json.RawMessage // property value for "property-change"
	Reason   string          // "eof", "stop", "error", ... for "end-file"
	Error    string          // file_error for "end-file", error for "command-error"
}

// Options configures the mpv process.
type Options struct {
	Binary    string   // defaults to "mpv"
	Volume    float64  // initial volume
	ExtraArgs []string // appended to the command line
}

// Client is a connection to a running mpv process.
type Client struct {
	cmd  *exec.Cmd
	conn net.Conn
	sock string

	wmu    sync.Mutex
	nextID int64

	// Events receives everything mpv reports. It is closed when mpv exits.
	Events chan Event

	closeOnce sync.Once
}

// Start launches mpv in idle mode and connects to its IPC socket.
func Start(opts Options) (*Client, error) {
	bin := opts.Binary
	if bin == "" {
		bin = "mpv"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("mpv is required for playback but %q was not found in your PATH.\n"+
			"Install it with your package manager (e.g. \"sudo pacman -S mpv\" or \"sudo apt install mpv\"),\n"+
			"or point to it with -mpv /path/to/mpv", bin)
	}
	sock := filepath.Join(os.TempDir(), fmt.Sprintf("terminal-music-%d.sock", os.Getpid()))
	_ = os.Remove(sock)

	args := []string{
		"--idle=yes",
		"--no-video",
		"--no-terminal",
		"--force-window=no",
		"--audio-display=no",
		"--input-default-bindings=no",
		"--input-ipc-server=" + sock,
		fmt.Sprintf("--volume=%g", opts.Volume),
	}
	args = append(args, opts.ExtraArgs...)

	cmd := exec.Command(path, args...)
	// Make sure mpv never outlives us, even if we crash.
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting mpv: %w", err)
	}

	var conn net.Conn
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err = net.Dial("unix", sock)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("connecting to mpv: %w", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	c := &Client{
		cmd:    cmd,
		conn:   conn,
		sock:   sock,
		Events: make(chan Event, 512),
	}
	go c.readLoop()
	return c, nil
}

type message struct {
	Event     string          `json:"event"`
	Name      string          `json:"name"`
	Data      json.RawMessage `json:"data"`
	Reason    string          `json:"reason"`
	FileError string          `json:"file_error"`
	RequestID *int64          `json:"request_id"`
	Error     string          `json:"error"`
}

func (c *Client) readLoop() {
	defer close(c.Events)
	sc := bufio.NewScanner(c.conn)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var m message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			continue
		}
		switch {
		case m.Event != "":
			c.Events <- Event{
				Name:     m.Event,
				Property: m.Name,
				Data:     m.Data,
				Reason:   m.Reason,
				Error:    m.FileError,
			}
		case m.RequestID != nil && m.Error != "" && m.Error != "success":
			c.Events <- Event{Name: "command-error", Error: m.Error}
		}
	}
}

// Command sends a command without waiting for its reply. Failures are
// reported on Events as "command-error".
func (c *Client) Command(args ...any) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.nextID++
	b, err := json.Marshal(map[string]any{"command": args, "request_id": c.nextID})
	if err != nil {
		return err
	}
	_, err = c.conn.Write(append(b, '\n'))
	return err
}

// Set sets an mpv property.
func (c *Client) Set(name string, value any) error {
	return c.Command("set_property", name, value)
}

// Observe asks mpv to report changes of a property on Events.
func (c *Client) Observe(id int, name string) error {
	return c.Command("observe_property", id, name)
}

// Close asks mpv to quit, killing it if it does not comply.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		_ = c.Command("quit")
		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()
		select {
		case err = <-done:
		case <-time.After(2 * time.Second):
			_ = c.cmd.Process.Kill()
			err = <-done
		}
		_ = c.conn.Close()
		_ = os.Remove(c.sock)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			err = nil
		}
	})
	return err
}
