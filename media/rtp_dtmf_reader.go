// SPDX-License-Identifier: MPL-2.0
// SPDX-FileCopyrightText: Copyright (c) 2024, Emir Aganovic

package media

import (
	"context"
	"io"
	"log/slog"
)

type RTPDtmfReader struct {
	codec        Codec // Depends on media session. Defaults to 101 per current mapping
	reader       io.Reader
	packetReader *RTPPacketReader

	lastEvent     uint8  // Last DTMF event number
	lastTimestamp uint32 // RTP timestamp of current DTMF event
	endProcessed  bool   // Whether we've already processed the end event
	dtmf          rune
	dtmfSet       bool
}

// RTP DTMF reader is middleware for reading DTMF events
// It reads from io Reader and checks packet Reader
func NewRTPDTMFReader(codec Codec, packetReader *RTPPacketReader, reader io.Reader, minDuration ...uint16) *RTPDtmfReader {
	// minDuration parameter kept for backward compatibility but ignored
	return &RTPDtmfReader{
		codec:        codec,
		packetReader: packetReader,
		reader:       reader,
		lastEvent:    255, // Initialize to invalid event number
	}
}

// Write is RTP io.Writer which adds more sync mechanism
func (w *RTPDtmfReader) Read(b []byte) (int, error) {
	n, err := w.reader.Read(b)
	if err != nil {
		// Signal our reader that no more dtmfs will be read
		// close(w.dtmfCh)
		return n, err
	}

	// Check is this DTMF
	hdr := w.packetReader.PacketHeader
	if hdr.PayloadType != w.codec.PayloadType {
		return n, nil
	}

	// Now decode DTMF
	ev := DTMFEvent{}
	if err := DTMFDecode(b, &ev); err != nil {
		slog.Error("Failed to decode DTMF event", "error", err)
	}
	w.processDTMFEvent(ev)
	return n, nil
}

func (w *RTPDtmfReader) processDTMFEvent(ev DTMFEvent) {
	// Get current RTP timestamp for duplicate detection
	timestamp := w.packetReader.PacketHeader.Timestamp

	if DefaultLogger().Handler().Enabled(context.Background(), slog.LevelDebug) {
		DefaultLogger().Debug("Processing DTMF event", "ev", ev, "timestamp", timestamp)
	}

	// Check if this is a new DTMF event (different digit or different timestamp)
	isNewEvent := w.lastEvent != ev.Event || w.lastTimestamp != timestamp

	if isNewEvent {
		// New DTMF event starting - reset tracking state
		w.lastEvent = ev.Event
		w.lastTimestamp = timestamp
		w.endProcessed = false

		// If it's already an end event, process it immediately
		if ev.EndOfEvent {
			w.dtmf = DTMFToRune(ev.Event)
			w.dtmfSet = true
			w.endProcessed = true
			DefaultLogger().Debug("New DTMF event with immediate end", "digit", w.dtmf)
		}
	} else if ev.EndOfEvent && !w.endProcessed {
		// End of current event - process only once
		w.dtmf = DTMFToRune(ev.Event)
		w.dtmfSet = true
		w.endProcessed = true
		DefaultLogger().Debug("DTMF end event processed", "digit", w.dtmf, "duration", ev.Duration)
	} else if ev.EndOfEvent && w.endProcessed {
		// Duplicate end event (RFC 2833 sends 3) - ignore
		DefaultLogger().Debug("Ignoring duplicate DTMF end event", "event", ev.Event)
	}
	// For continuation packets (not end), we just track them but don't report
}

func (w *RTPDtmfReader) ReadDTMF() (rune, bool) {
	defer func() { w.dtmfSet = false }()
	return w.dtmf, w.dtmfSet
	// dtmf, ok := <-w.dtmfCh
	// return DTMFToRune(dtmf), ok
}
