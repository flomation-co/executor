package twilio_common

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestVAD_SilenceOnly(t *testing.T) {
	RegisterTestingT(t)

	vad := NewVAD()

	silence := make([]byte, 160)
	for i := range silence {
		silence[i] = 0xFF
	}

	result := vad.ProcessFrame(silence)
	Expect(result).To(Equal(VADSilence))
}

func TestVAD_SpeechDetected(t *testing.T) {
	RegisterTestingT(t)

	vad := NewVAD()

	// Loud audio
	loud := make([]byte, 160)
	for i := range loud {
		loud[i] = 0x10
	}

	result := vad.ProcessFrame(loud)
	Expect(result).To(Equal(VADSpeech))
}

func TestVAD_EndOfSpeech(t *testing.T) {
	RegisterTestingT(t)

	// Direct field access to set MinSpeechDuration below default
	vad := NewVAD()
	vad.SilenceDuration = 50 * time.Millisecond
	vad.MinSpeechDuration = 1 * time.Millisecond

	// Send speech frames with gap to exceed min speech duration
	loud := make([]byte, 160)
	for i := range loud {
		loud[i] = 0x10
	}
	vad.ProcessFrame(loud)
	time.Sleep(5 * time.Millisecond)
	vad.ProcessFrame(loud)

	// Wait well beyond silence duration
	time.Sleep(100 * time.Millisecond)

	// Send silence
	silence := make([]byte, 160)
	for i := range silence {
		silence[i] = 0xFF
	}

	result := vad.ProcessFrame(silence)
	Expect(result).To(Equal(VADEndOfSpeech))
}

func TestVAD_Reset(t *testing.T) {
	RegisterTestingT(t)

	vad := NewVAD()

	loud := make([]byte, 160)
	for i := range loud {
		loud[i] = 0x10
	}
	vad.ProcessFrame(loud)

	vad.Reset()

	silence := make([]byte, 160)
	for i := range silence {
		silence[i] = 0xFF
	}
	result := vad.ProcessFrame(silence)
	Expect(result).To(Equal(VADSilence))
}

func TestVAD_MinSpeechDuration(t *testing.T) {
	RegisterTestingT(t)

	// Require 500ms of speech, silence after 50ms
	vad := NewVADWithConfig(0.01, 50, 500)

	// Very brief speech
	loud := make([]byte, 160)
	for i := range loud {
		loud[i] = 0x10
	}
	vad.ProcessFrame(loud)

	time.Sleep(60 * time.Millisecond)

	// Silence — should reset because speech was too short
	silence := make([]byte, 160)
	for i := range silence {
		silence[i] = 0xFF
	}
	result := vad.ProcessFrame(silence)
	Expect(result).To(Equal(VADSilence)) // Reset due to min speech duration
}

func TestNewVADWithConfig_Defaults(t *testing.T) {
	RegisterTestingT(t)

	vad := NewVADWithConfig(0, 0, 0)
	Expect(vad.SilenceThreshold).To(Equal(0.01))
	Expect(vad.SilenceDuration).To(Equal(1500 * time.Millisecond))
	Expect(vad.MinSpeechDuration).To(Equal(300 * time.Millisecond))
}
