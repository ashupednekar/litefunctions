package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Stream emits recent entries followed by new entries as server-sent events.
// It returns nil when the browser disconnects or the parent context ends.
func (c *Client) Stream(ctx context.Context, w http.ResponseWriter, flusher http.Flusher) error {
	if _, err := fmt.Fprint(w, "retry: 3000\n\n"); err != nil {
		return err
	}
	flusher.Flush()

	start := time.Now().Add(-c.lookback)
	direction := "backward"
	for {
		end := time.Now()
		entries, err := c.queryRange(ctx, start, end, direction)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) && ctx.Err() != nil {
				return nil
			}
			if writeErr := writeEvent(w, "stream_error", map[string]string{"error": "log stream temporarily unavailable"}); writeErr != nil {
				return writeErr
			}
			flusher.Flush()
			return err
		}

		for _, entry := range entries {
			if err := writeEvent(w, "log", entry); err != nil {
				return err
			}
		}
		if len(entries) == 0 {
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return err
			}
		}
		flusher.Flush()

		// The first request returns the newest entries. Subsequent requests move
		// forward and resume just after the newest timestamp already emitted.
		direction = "forward"
		start = end
		if len(entries) > 0 {
			start = entries[len(entries)-1].Timestamp.Add(time.Nanosecond)
		}

		poll := time.NewTimer(c.pollInterval)
		select {
		case <-ctx.Done():
			if !poll.Stop() {
				select {
				case <-poll.C:
				default:
				}
			}
			return nil
		case <-poll.C:
		}
	}
}

func writeEvent(w http.ResponseWriter, event string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	return err
}
