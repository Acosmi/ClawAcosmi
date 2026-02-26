package api

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"log"

	"Argus-compound/go-sensory/internal/capture"

	"golang.org/x/net/websocket"
)

// handleFrameStream pushes frames to connected WebSocket clients (JSON + base64).
func (s *Server) handleFrameStream(ws *websocket.Conn) {
	log.Printf("Client connected: %s", ws.RemoteAddr())

	// Register with hub to get a dedicated frame channel
	frameChan := s.hub.Register(ws.RemoteAddr().String())

	defer func() {
		s.hub.Unregister(frameChan)
		ws.Close()
		log.Printf("Client disconnected: %s", ws.RemoteAddr())
	}()

	for frame := range frameChan {
		// Convert frame to JPEG thumbnail
		jpegB64, err := frameToJPEGBase64(frame, 50) // quality 50 for thumbnails
		if err != nil {
			continue
		}

		msg := FrameMessage{
			Type:      "frame",
			FrameNo:   frame.FrameNo,
			Width:     frame.Width,
			Height:    frame.Height,
			Timestamp: frame.Timestamp.UnixMilli(),
			ImageB64:  jpegB64,
		}

		data, _ := json.Marshal(msg)
		if _, err := ws.Write(data); err != nil {
			return // client disconnected
		}
	}
}

// handleBinaryFrameStream sends frames as compact binary messages.
// Protocol: [4B width LE][4B height LE][8B frame_no LE][4B jpeg_size LE][N bytes JPEG]
// Total header: 20 bytes. Eliminates ~33% base64 overhead.
func (s *Server) handleBinaryFrameStream(ws *websocket.Conn) {
	log.Printf("Binary client connected: %s", ws.RemoteAddr())

	frameChan := s.hub.Register(ws.RemoteAddr().String())

	defer func() {
		s.hub.Unregister(frameChan)
		ws.Close()
		log.Printf("Binary client disconnected: %s", ws.RemoteAddr())
	}()

	// Ensure WebSocket sends binary frames (opcode 0x02), not text.
	ws.PayloadType = websocket.BinaryFrame

	for frame := range frameChan {
		jpegData, err := frameToJPEG(frame, 50)
		if err != nil {
			continue
		}

		// Build binary message: 20-byte header + JPEG payload
		var buf bytes.Buffer
		buf.Grow(20 + len(jpegData))
		binary.Write(&buf, binary.LittleEndian, uint32(frame.Width))
		binary.Write(&buf, binary.LittleEndian, uint32(frame.Height))
		binary.Write(&buf, binary.LittleEndian, frame.FrameNo)
		binary.Write(&buf, binary.LittleEndian, uint32(len(jpegData)))
		buf.Write(jpegData)

		if _, err := ws.Write(buf.Bytes()); err != nil {
			return // client disconnected
		}
	}
}

// frameToJPEGBase64 converts a frame to a base64-encoded JPEG string.
func frameToJPEGBase64(frame *capture.Frame, quality int) (string, error) {
	jpegData, err := frameToJPEG(frame, quality)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(jpegData), nil
}
