// Package relay implements the Solo WebSocket relay server.
package relay

import (
	"log/slog"

	"github.com/gorilla/websocket"

	relaymetrics "github.com/WuErPing/solo/relay/internal/metrics"
)

type bufferedFrame struct {
	msgType int
	data    []byte
}

type FrameBuffer struct {
	frames  []bufferedFrame
	maxSize int
}

func NewFrameBuffer(maxSize int) *FrameBuffer {
	return &FrameBuffer{
		frames:  make([]bufferedFrame, 0),
		maxSize: maxSize,
	}
}

func (b *FrameBuffer) Push(msgType int, frame []byte) {
	b.frames = append(b.frames, bufferedFrame{msgType: msgType, data: frame})
	relaymetrics.FramesBuffered.Inc()
	if len(b.frames) > b.maxSize {
		b.frames = b.frames[len(b.frames)-b.maxSize:]
		relaymetrics.BufferOverflows.Inc()
	}
}

func (b *FrameBuffer) Flush(conn *websocket.Conn, logger *slog.Logger) int {
	if len(b.frames) == 0 {
		return 0
	}
	flushed := 0
	for _, frame := range b.frames {
		if err := conn.WriteMessage(frame.msgType, frame.data); err != nil {
			b.frames = b.frames[flushed:]
			logger.Debug("flush interrupted, re-buffering remaining frames", "remaining", len(b.frames))
			return flushed
		}
		flushed++
	}
	b.frames = b.frames[:0]
	return flushed
}

func (b *FrameBuffer) Len() int {
	return len(b.frames)
}

func (b *FrameBuffer) Clear() {
	b.frames = b.frames[:0]
}
