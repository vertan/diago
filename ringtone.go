// SPDX-License-Identifier: MPL-2.0
// SPDX-FileCopyrightText: Copyright (c) 2024, Emir Aganovic

package diago

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"github.com/vertan/diago/audio"
	"github.com/vertan/diago/media"
)

var (
	ringtones sync.Map
)

func loadRingTonePCM(codec media.Codec) ([]byte, error) {
	uuid := fmt.Sprintf("%s-%d", codec.Name, codec.SampleRate)
	ringval, exists := ringtones.Load(uuid)
	if exists {
		return ringval.([]byte), nil
	}
	pcmBytes := generateRingTonePCM(int(codec.SampleRate))
	ringtones.Store(uuid, pcmBytes)
	return pcmBytes, nil
}

func generateRingTonePCM(sampleRate int) []byte {
	var (
		durationSec = 2
		volume      = 0.3
		freq1       = 350.0
		freq2       = 440.0
	)

	numSamples := sampleRate * durationSec
	buf := &bytes.Buffer{}

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		// Combine the two sine waves and normalize
		sample := volume * (math.Sin(2*math.Pi*freq1*t) + math.Sin(2*math.Pi*freq2*t)) / 2.0
		// Convert to 16-bit signed PCM
		intSample := int16(sample * math.MaxInt16)
		binary.Write(buf, binary.LittleEndian, intSample)
	}

	pcmBytes := buf.Bytes()

	return pcmBytes
}

// AudioRingtone is playback for ringtone
//
// Experimental
type AudioRingtone struct {
	writer       *audio.PCMEncoderWriter
	ringtone     []byte
	sampleSize   int
	mediaSession *media.MediaSession
}

func (a *AudioRingtone) PlayBackground() (func() error, error) {
	if err := a.mediaSession.StartRTP(1); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}
	wg.Add(1)
	var playErr error
	go func() {
		defer wg.Done()
		playErr = a.play(ctx)
	}()

	return func() error {
		cancel()

		if err := a.mediaSession.StopRTP(2, 0); err != nil {
			return err
		}
		wg.Wait()

		// enable RTP again
		if err := a.mediaSession.StartRTP(2); err != nil {
			return err
		}

		if e, ok := playErr.(net.Error); ok && e.Timeout() {
			return nil
		}

		return playErr
	}, nil
}

func (a *AudioRingtone) Play(ctx context.Context) error {
	return a.play(ctx)
}

func (a *AudioRingtone) play(timerCtx context.Context) error {
	t := time.NewTimer(0)
	for {
		_, err := media.WriteAll(a.writer, a.ringtone, a.sampleSize)
		if err != nil {
			return err
		}

		t.Reset(4 * time.Second)
		select {
		case <-t.C:
		case <-timerCtx.Done():
			return timerCtx.Err()
		}
	}
}
