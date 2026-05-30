package twilio_common

import (
	"time"
)

// VAD (Voice Activity Detection) detects speech boundaries in mulaw audio.
// Uses simple amplitude-based detection with configurable thresholds.
type VAD struct {
	// SilenceThreshold is the normalised energy level below which audio
	// is considered silence. Range 0.0-1.0. Default: 0.01.
	SilenceThreshold float64

	// SilenceDuration is how long silence must persist before the VAD
	// considers speech to have ended. Default: 1500ms.
	SilenceDuration time.Duration

	// MinSpeechDuration is the minimum duration of speech required before
	// accepting it as valid input. Prevents false triggers from brief
	// noise bursts. Default: 300ms.
	MinSpeechDuration time.Duration

	speechStartedAt *time.Time
	lastSpeechAt    *time.Time
}

// NewVAD creates a VAD with sensible defaults for telephony audio.
func NewVAD() *VAD {
	return &VAD{
		SilenceThreshold:  0.01,
		SilenceDuration:   1500 * time.Millisecond,
		MinSpeechDuration: 300 * time.Millisecond,
	}
}

// NewVADWithConfig creates a VAD with custom settings.
func NewVADWithConfig(silenceThreshold float64, silenceDurationMs, minSpeechDurationMs int) *VAD {
	v := NewVAD()
	if silenceThreshold > 0 {
		v.SilenceThreshold = silenceThreshold
	}
	if silenceDurationMs > 0 {
		v.SilenceDuration = time.Duration(silenceDurationMs) * time.Millisecond
	}
	if minSpeechDurationMs > 0 {
		v.MinSpeechDuration = time.Duration(minSpeechDurationMs) * time.Millisecond
	}
	return v
}

// VADResult indicates what the VAD detected after processing a frame.
type VADResult int

const (
	// VADSilence indicates no speech detected.
	VADSilence VADResult = iota
	// VADSpeech indicates active speech.
	VADSpeech
	// VADEndOfSpeech indicates speech has ended (silence after speech).
	VADEndOfSpeech
)

// ProcessFrame analyses a mulaw audio frame and returns the VAD state.
// Call this for each incoming audio chunk from the WebSocket.
func (v *VAD) ProcessFrame(frame []byte) VADResult {
	energy := MulawEnergy(frame)
	now := time.Now()

	if energy >= v.SilenceThreshold {
		// Speech detected
		if v.speechStartedAt == nil {
			v.speechStartedAt = &now
		}
		v.lastSpeechAt = &now
		return VADSpeech
	}

	// Silence
	if v.lastSpeechAt == nil {
		return VADSilence
	}

	// Check if silence has lasted long enough after speech
	silenceSoFar := now.Sub(*v.lastSpeechAt)
	if silenceSoFar >= v.SilenceDuration {
		// Check minimum speech duration
		if v.speechStartedAt != nil {
			speechDuration := v.lastSpeechAt.Sub(*v.speechStartedAt)
			if speechDuration < v.MinSpeechDuration {
				// Speech was too short — treat as noise, reset
				v.Reset()
				return VADSilence
			}
		}
		return VADEndOfSpeech
	}

	// In the silence gap after speech, but not long enough yet
	return VADSpeech
}

// Reset clears the VAD state for a new utterance.
func (v *VAD) Reset() {
	v.speechStartedAt = nil
	v.lastSpeechAt = nil
}