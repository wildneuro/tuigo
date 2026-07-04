//go:build !windows

package renderer

import (
	"os"
	"os/signal"
	"syscall"
)

func (t *Terminal) resizes() <-chan Size {
	ch := make(chan Size)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	go func() {
		defer close(ch)
		for range sig {
			w, h := t.Size()
			ch <- Size{W: w, H: h}
		}
	}()
	return ch
}
