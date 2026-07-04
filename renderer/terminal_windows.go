//go:build windows

package renderer

import (
	"time"
)

func (t *Terminal) resizes() <-chan Size {
	ch := make(chan Size)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		prevW, prevH := t.Size()
		for {
			select {
			case <-ticker.C:
				w, h := t.Size()
				if w != prevW || h != prevH {
					prevW, prevH = w, h
					ch <- Size{W: w, H: h}
				}
			}
		}
	}()
	return ch
}
