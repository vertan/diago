package media

import (
	"strings"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
)

func TestDTMFReader(t *testing.T) {
	// Create a mock packet reader to provide timestamps
	packetReader := &RTPPacketReader{
		PacketHeader: rtp.Header{},
	}
	r := RTPDtmfReader{
		packetReader: packetReader,
		lastEvent:    255, // Initialize to invalid event
	}

	// DTMF 109
	timestamps := []uint32{
		// Digit 1
		1000, 1000, 1000, 1000, 1000, 1000, 1000,
		// Digit 0
		2000, 2000, 2000, 2000, 2000, 2000, 2000,
		// Digit 9
		3000, 3000, 3000, 3000, 3000, 3000, 3000,
	}
	timestampIndex := 0

	sequence := []DTMFEvent{
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 160},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 320},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 480},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 640},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 0, EndOfEvent: false, Volume: 10, Duration: 160},
		{Event: 0, EndOfEvent: false, Volume: 10, Duration: 320},
		{Event: 0, EndOfEvent: false, Volume: 10, Duration: 480},
		{Event: 0, EndOfEvent: false, Volume: 10, Duration: 640},
		{Event: 0, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 0, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 0, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 9, EndOfEvent: false, Volume: 10, Duration: 160},
		{Event: 9, EndOfEvent: false, Volume: 10, Duration: 320},
		{Event: 9, EndOfEvent: false, Volume: 10, Duration: 480},
		{Event: 9, EndOfEvent: false, Volume: 10, Duration: 640},
		{Event: 9, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 9, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 9, EndOfEvent: true, Volume: 10, Duration: 800},
	}

	detected := strings.Builder{}
	for _, ev := range sequence {
		// Set the appropriate timestamp for this event
		packetReader.PacketHeader.Timestamp = timestamps[timestampIndex]
		timestampIndex++

		r.processDTMFEvent(ev)
		dtmf, set := r.ReadDTMF()
		if set {
			detected.WriteRune(dtmf)
		}
	}

	assert.Equal(t, "109", detected.String())
}

func TestDTMFReaderRepeated(t *testing.T) {
	// Create a mock packet reader to provide timestamps
	packetReader := &RTPPacketReader{
		PacketHeader: rtp.Header{},
	}
	r := RTPDtmfReader{
		packetReader: packetReader,
		lastEvent:    255, // Initialize to invalid event
	}

	// DTMF 111 - three separate presses of digit 1
	// Each press needs a different timestamp to be detected as separate
	timestamps := []uint32{
		// First '1'
		1000, 1000, 1000, 1000, 1000, 1000, 1000,
		// Second '1'
		2000, 2000, 2000, 2000, 2000, 2000, 2000,
		// Third '1'
		3000, 3000, 3000, 3000, 3000, 3000, 3000,
	}
	timestampIndex := 0

	sequence := []DTMFEvent{
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 160},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 320},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 480},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 640},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 160},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 320},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 480},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 640},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 160},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 320},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 480},
		{Event: 1, EndOfEvent: false, Volume: 10, Duration: 640},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
		{Event: 1, EndOfEvent: true, Volume: 10, Duration: 800},
	}

	detected := strings.Builder{}
	for _, ev := range sequence {
		// Set the appropriate timestamp for this event
		packetReader.PacketHeader.Timestamp = timestamps[timestampIndex]
		timestampIndex++

		r.processDTMFEvent(ev)
		dtmf, set := r.ReadDTMF()
		if set {
			detected.WriteRune(dtmf)
		}
	}

	assert.Equal(t, "111", detected.String())
}
