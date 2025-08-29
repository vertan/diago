package media

import (
	"io"
	"strings"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
)

// Mock RTP reader for testing
type mockRTPReader struct {
	packets []rtp.Packet
	index   int
}

func (m *mockRTPReader) Read(b []byte) (int, error) {
	if m.index >= len(m.packets) {
		return 0, io.EOF
	}
	pkt := m.packets[m.index]
	m.index++

	data, err := pkt.Marshal()
	if err != nil {
		return 0, err
	}

	n := copy(b, data)
	return n, nil
}

func (m *mockRTPReader) Write(b []byte) (int, error) {
	return len(b), nil
}

// TestDTMFShortDuration tests that short DTMF durations are now accepted
func TestDTMFShortDuration(t *testing.T) {
	codec := Codec{PayloadType: 101}

	// Create a mock packet reader
	packetReader := &RTPPacketReader{
		PacketHeader: rtp.Header{},
	}

	// Create reader with old default minDuration (would have been 480)
	reader := NewRTPDTMFReader(codec, packetReader, nil)

	testCases := []struct {
		name        string
		events      []DTMFEvent
		timestamps  []uint32
		expected    string
		description string
	}{
		{
			name: "very_short_dtmf_20ms",
			events: []DTMFEvent{
				{Event: 1, EndOfEvent: false, Volume: 10, Duration: 80},
				{Event: 1, EndOfEvent: true, Volume: 10, Duration: 160}, // 20ms total
				{Event: 1, EndOfEvent: true, Volume: 10, Duration: 160}, // duplicate
				{Event: 1, EndOfEvent: true, Volume: 10, Duration: 160}, // duplicate
			},
			timestamps:  []uint32{1000, 1000, 1000, 1000},
			expected:    "1",
			description: "Should accept 20ms DTMF (160 timestamp units)",
		},
		{
			name: "short_dtmf_40ms",
			events: []DTMFEvent{
				{Event: 2, EndOfEvent: false, Volume: 10, Duration: 160},
				{Event: 2, EndOfEvent: false, Volume: 10, Duration: 240},
				{Event: 2, EndOfEvent: true, Volume: 10, Duration: 320}, // 40ms total
				{Event: 2, EndOfEvent: true, Volume: 10, Duration: 320}, // duplicate
			},
			timestamps:  []uint32{2000, 2000, 2000, 2000},
			expected:    "2",
			description: "Should accept 40ms DTMF (320 timestamp units)",
		},
		{
			name: "multiple_short_dtmf",
			events: []DTMFEvent{
				// First digit: 3 (30ms)
				{Event: 3, EndOfEvent: false, Volume: 10, Duration: 80},
				{Event: 3, EndOfEvent: true, Volume: 10, Duration: 240},
				// Second digit: 4 (25ms)
				{Event: 4, EndOfEvent: false, Volume: 10, Duration: 80},
				{Event: 4, EndOfEvent: true, Volume: 10, Duration: 200},
				// Third digit: 5 (50ms)
				{Event: 5, EndOfEvent: false, Volume: 10, Duration: 160},
				{Event: 5, EndOfEvent: true, Volume: 10, Duration: 400},
			},
			timestamps:  []uint32{3000, 3000, 4000, 4000, 5000, 5000},
			expected:    "345",
			description: "Should accept multiple short DTMFs in sequence",
		},
		{
			name: "immediate_end_event",
			events: []DTMFEvent{
				// Single end event without prior start events
				{Event: 6, EndOfEvent: true, Volume: 10, Duration: 200},
				{Event: 6, EndOfEvent: true, Volume: 10, Duration: 200}, // duplicate
			},
			timestamps:  []uint32{6000, 6000},
			expected:    "6",
			description: "Should handle immediate end event",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset reader state
			reader.lastEvent = 255
			reader.lastTimestamp = 0
			reader.endProcessed = false
			reader.dtmfSet = false

			detected := strings.Builder{}

			for i, ev := range tc.events {
				// Set timestamp for this event
				packetReader.PacketHeader.Timestamp = tc.timestamps[i]

				reader.processDTMFEvent(ev)
				dtmf, set := reader.ReadDTMF()
				if set {
					detected.WriteRune(dtmf)
				}
			}

			assert.Equal(t, tc.expected, detected.String(), tc.description)
		})
	}
}

// TestDTMFDuplicateEndEvents tests that RFC 2833 triple end-event transmission is handled correctly
func TestDTMFDuplicateEndEvents(t *testing.T) {
	codec := Codec{PayloadType: 101}
	packetReader := &RTPPacketReader{
		PacketHeader: rtp.Header{},
	}
	reader := NewRTPDTMFReader(codec, packetReader, nil)

	// Simulate RFC 2833 compliant transmission with triple end events
	events := []DTMFEvent{
		{Event: 7, EndOfEvent: false, Volume: 10, Duration: 160},
		{Event: 7, EndOfEvent: false, Volume: 10, Duration: 320},
		{Event: 7, EndOfEvent: false, Volume: 10, Duration: 480},
		{Event: 7, EndOfEvent: true, Volume: 10, Duration: 640}, // First end
		{Event: 7, EndOfEvent: true, Volume: 10, Duration: 640}, // Second end (duplicate)
		{Event: 7, EndOfEvent: true, Volume: 10, Duration: 640}, // Third end (duplicate)
	}

	detected := strings.Builder{}
	detectionCount := 0

	for _, ev := range events {
		packetReader.PacketHeader.Timestamp = 7000 // Same timestamp for all packets of this event
		reader.processDTMFEvent(ev)
		dtmf, set := reader.ReadDTMF()
		if set {
			detected.WriteRune(dtmf)
			detectionCount++
		}
	}

	assert.Equal(t, "7", detected.String(), "Should detect digit only once")
	assert.Equal(t, 1, detectionCount, "Should only report DTMF once despite triple end events")
}

// TestDTMFTimestampBasedDuplication tests timestamp-based duplicate detection
func TestDTMFTimestampBasedDuplication(t *testing.T) {
	codec := Codec{PayloadType: 101}
	packetReader := &RTPPacketReader{
		PacketHeader: rtp.Header{},
	}
	reader := NewRTPDTMFReader(codec, packetReader, nil)

	type eventWithTimestamp struct {
		event     DTMFEvent
		timestamp uint32
	}

	// Test same digit with different timestamps (should be treated as separate events)
	sequence := []eventWithTimestamp{
		// First press of '8'
		{DTMFEvent{Event: 8, EndOfEvent: false, Volume: 10, Duration: 160}, 8000},
		{DTMFEvent{Event: 8, EndOfEvent: true, Volume: 10, Duration: 320}, 8000},
		// Second press of '8' (different timestamp)
		{DTMFEvent{Event: 8, EndOfEvent: false, Volume: 10, Duration: 160}, 9000},
		{DTMFEvent{Event: 8, EndOfEvent: true, Volume: 10, Duration: 320}, 9000},
		// Third press of '8' (yet another timestamp)
		{DTMFEvent{Event: 8, EndOfEvent: false, Volume: 10, Duration: 160}, 10000},
		{DTMFEvent{Event: 8, EndOfEvent: true, Volume: 10, Duration: 320}, 10000},
	}

	detected := strings.Builder{}

	for _, item := range sequence {
		packetReader.PacketHeader.Timestamp = item.timestamp
		reader.processDTMFEvent(item.event)
		dtmf, set := reader.ReadDTMF()
		if set {
			detected.WriteRune(dtmf)
		}
	}

	assert.Equal(t, "888", detected.String(), "Should detect same digit multiple times with different timestamps")
}

// TestDTMFBackwardCompatibility ensures the function signature still accepts minDuration parameter
func TestDTMFBackwardCompatibility(t *testing.T) {
	codec := Codec{PayloadType: 101}
	packetReader := &RTPPacketReader{}

	// Test that the function still accepts minDuration parameter (for backward compatibility)
	reader1 := NewRTPDTMFReader(codec, packetReader, nil)
	assert.NotNil(t, reader1)

	reader2 := NewRTPDTMFReader(codec, packetReader, nil, 480) // with minDuration
	assert.NotNil(t, reader2)

	reader3 := NewRTPDTMFReader(codec, packetReader, nil, 160, 320) // extra params ignored
	assert.NotNil(t, reader3)
}

// TestDTMFReaderOriginal preserves the original test logic with the fix
func TestDTMFReaderOriginal(t *testing.T) {
	codec := Codec{PayloadType: 101}
	packetReader := &RTPPacketReader{
		PacketHeader: rtp.Header{},
	}
	r := NewRTPDTMFReader(codec, packetReader, nil)

	// DTMF 109 - original test sequence
	sequence := []struct {
		event     DTMFEvent
		timestamp uint32
	}{
		// Digit 1
		{DTMFEvent{Event: 1, EndOfEvent: false, Volume: 10, Duration: 160}, 1000},
		{DTMFEvent{Event: 1, EndOfEvent: false, Volume: 10, Duration: 320}, 1000},
		{DTMFEvent{Event: 1, EndOfEvent: false, Volume: 10, Duration: 480}, 1000},
		{DTMFEvent{Event: 1, EndOfEvent: false, Volume: 10, Duration: 640}, 1000},
		{DTMFEvent{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800}, 1000},
		{DTMFEvent{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800}, 1000},
		{DTMFEvent{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800}, 1000},
		// Digit 0
		{DTMFEvent{Event: 0, EndOfEvent: false, Volume: 10, Duration: 160}, 2000},
		{DTMFEvent{Event: 0, EndOfEvent: false, Volume: 10, Duration: 320}, 2000},
		{DTMFEvent{Event: 0, EndOfEvent: false, Volume: 10, Duration: 480}, 2000},
		{DTMFEvent{Event: 0, EndOfEvent: false, Volume: 10, Duration: 640}, 2000},
		{DTMFEvent{Event: 0, EndOfEvent: true, Volume: 10, Duration: 800}, 2000},
		{DTMFEvent{Event: 0, EndOfEvent: true, Volume: 10, Duration: 800}, 2000},
		{DTMFEvent{Event: 0, EndOfEvent: true, Volume: 10, Duration: 800}, 2000},
		// Digit 9
		{DTMFEvent{Event: 9, EndOfEvent: false, Volume: 10, Duration: 160}, 3000},
		{DTMFEvent{Event: 9, EndOfEvent: false, Volume: 10, Duration: 320}, 3000},
		{DTMFEvent{Event: 9, EndOfEvent: false, Volume: 10, Duration: 480}, 3000},
		{DTMFEvent{Event: 9, EndOfEvent: false, Volume: 10, Duration: 640}, 3000},
		{DTMFEvent{Event: 9, EndOfEvent: true, Volume: 10, Duration: 800}, 3000},
		{DTMFEvent{Event: 9, EndOfEvent: true, Volume: 10, Duration: 800}, 3000},
		{DTMFEvent{Event: 9, EndOfEvent: true, Volume: 10, Duration: 800}, 3000},
	}

	detected := strings.Builder{}
	for _, item := range sequence {
		packetReader.PacketHeader.Timestamp = item.timestamp
		r.processDTMFEvent(item.event)
		dtmf, set := r.ReadDTMF()
		if set {
			detected.WriteRune(dtmf)
		}
	}

	assert.Equal(t, "109", detected.String())
}
