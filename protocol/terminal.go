package protocol

import "encoding/json"

// TerminalStreamOpcode represents the opcode in a binary terminal frame.
// Frame format: [opcode:1 byte][slot:1 byte][payload:N bytes]
type TerminalStreamOpcode byte

const (
	TerminalOutput   TerminalStreamOpcode = 0x01
	TerminalInput    TerminalStreamOpcode = 0x02
	TerminalResize   TerminalStreamOpcode = 0x03
	TerminalSnapshot TerminalStreamOpcode = 0x04
)

// TerminalStreamFrame represents a decoded binary terminal frame.
type TerminalStreamFrame struct {
	Opcode  TerminalStreamOpcode
	Slot    byte
	Payload []byte
}

// EncodeTerminalFrame encodes a terminal stream frame into bytes.
func EncodeTerminalFrame(frame TerminalStreamFrame) []byte {
	buf := make([]byte, 2+len(frame.Payload))
	buf[0] = byte(frame.Opcode)
	buf[1] = frame.Slot
	copy(buf[2:], frame.Payload)
	return buf
}

// DecodeTerminalFrame decodes bytes into a terminal stream frame.
// Returns nil if the data is too short to contain a valid frame.
func DecodeTerminalFrame(data []byte) *TerminalStreamFrame {
	if len(data) < 2 {
		return nil
	}
	opcode := TerminalStreamOpcode(data[0])
	switch opcode {
	case TerminalOutput, TerminalInput, TerminalResize, TerminalSnapshot:
		// valid
	default:
		return nil
	}
	slot := data[1]
	payload := data[2:]
	return &TerminalStreamFrame{
		Opcode:  opcode,
		Slot:    slot,
		Payload: payload,
	}
}

// TerminalResizePayload represents the JSON payload for a Resize opcode.
type TerminalResizePayload struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

// DecodeTerminalResize decodes a resize payload from bytes.
func DecodeTerminalResize(payload []byte) (*TerminalResizePayload, error) {
	var r TerminalResizePayload
	if err := json.Unmarshal(payload, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// genzod
// TerminalCell represents a single cell in the terminal grid.
type TerminalCell struct {
	Char          string `json:"char"`
	FG            *int   `json:"fg,omitempty"`
	BG            *int   `json:"bg,omitempty"`
	FGMode        *int   `json:"fgMode,omitempty"`
	BGMode        *int   `json:"bgMode,omitempty"`
	Bold          *bool  `json:"bold,omitempty"`
	Italic        *bool  `json:"italic,omitempty"`
	Underline     *bool  `json:"underline,omitempty"`
	Dim           *bool  `json:"dim,omitempty"`
	Inverse       *bool  `json:"inverse,omitempty"`
	Strikethrough *bool  `json:"strikethrough,omitempty"`
}

// genzod
// TerminalCursor represents the cursor state in a terminal snapshot.
type TerminalCursor struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Hidden *bool  `json:"hidden,omitempty"`
	Style  string `json:"style,omitempty"`
	Blink  *bool  `json:"blink,omitempty"`
}

// genzod
// TerminalState represents the full state of a terminal (used in Snapshot opcode).
type TerminalState struct {
	Rows       int              `json:"rows"`
	Cols       int              `json:"cols"`
	Grid       [][]TerminalCell `json:"grid"`
	Scrollback [][]TerminalCell `json:"scrollback"`
	Cursor     TerminalCursor   `json:"cursor"`
	Title      string           `json:"title,omitempty"`
}
