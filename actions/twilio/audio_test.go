package twilio_common

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestMulawEnergy_Silence(t *testing.T) {
	RegisterTestingT(t)

	// Mulaw silence is 0xFF (or 0x7F)
	silence := make([]byte, 160)
	for i := range silence {
		silence[i] = 0xFF
	}
	energy := MulawEnergy(silence)
	Expect(energy).To(BeNumerically("<", 0.01))
}

func TestMulawEnergy_Loud(t *testing.T) {
	RegisterTestingT(t)

	// Mulaw 0x00 decodes to a large positive value
	loud := make([]byte, 160)
	for i := range loud {
		loud[i] = 0x00
	}
	energy := MulawEnergy(loud)
	Expect(energy).To(BeNumerically(">", 0.5))
}

func TestMulawEnergy_Empty(t *testing.T) {
	RegisterTestingT(t)

	Expect(MulawEnergy(nil)).To(Equal(0.0))
	Expect(MulawEnergy([]byte{})).To(Equal(0.0))
}

func TestStripWAVHeader_WithRIFF(t *testing.T) {
	RegisterTestingT(t)

	// Build a minimal WAV header (44 bytes) + some audio data
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	copy(header[8:12], "WAVE")
	copy(header[36:40], "data")
	// data chunk size at bytes 40-43
	header[40] = 4 // 4 bytes of audio

	audio := []byte{0x01, 0x02, 0x03, 0x04}
	data := append(header, audio...)

	stripped := StripWAVHeader(data)
	Expect(stripped).To(Equal(audio))
}

func TestStripWAVHeader_NoHeader(t *testing.T) {
	RegisterTestingT(t)

	audio := []byte{0x01, 0x02, 0x03}
	Expect(StripWAVHeader(audio)).To(Equal(audio))
}

func TestStripWAVHeader_TooShort(t *testing.T) {
	RegisterTestingT(t)

	short := []byte{0x01, 0x02}
	Expect(StripWAVHeader(short)).To(Equal(short))
}

func TestChunkAudio(t *testing.T) {
	RegisterTestingT(t)

	data := make([]byte, 400)
	chunks := ChunkAudio(data, 160)
	Expect(chunks).To(HaveLen(3))
	Expect(chunks[0]).To(HaveLen(160))
	Expect(chunks[1]).To(HaveLen(160))
	Expect(chunks[2]).To(HaveLen(80)) // remainder
}

func TestChunkAudio_ExactMultiple(t *testing.T) {
	RegisterTestingT(t)

	data := make([]byte, 320)
	chunks := ChunkAudio(data, 160)
	Expect(chunks).To(HaveLen(2))
	Expect(chunks[0]).To(HaveLen(160))
	Expect(chunks[1]).To(HaveLen(160))
}

func TestWrapMulawInWAV(t *testing.T) {
	RegisterTestingT(t)

	rawMulaw := make([]byte, 8000) // 1 second at 8kHz
	for i := range rawMulaw {
		rawMulaw[i] = 0x7F
	}

	wav := WrapMulawInWAV(rawMulaw)

	// Should have 44-byte header + raw data
	Expect(len(wav)).To(Equal(44 + len(rawMulaw)))

	// Check RIFF header
	Expect(string(wav[0:4])).To(Equal("RIFF"))
	Expect(string(wav[8:12])).To(Equal("WAVE"))
	Expect(string(wav[12:16])).To(Equal("fmt "))
	Expect(string(wav[36:40])).To(Equal("data"))

	// Check format code = 7 (mulaw)
	Expect(wav[20]).To(Equal(byte(7)))
	Expect(wav[21]).To(Equal(byte(0)))

	// Check sample rate = 8000
	Expect(wav[24]).To(Equal(byte(0x40))) // 8000 = 0x1F40
	Expect(wav[25]).To(Equal(byte(0x1F)))

	// Audio data should be intact after header
	Expect(wav[44:]).To(Equal(rawMulaw))
}

func TestWrapMulawInWAV_RoundTrip(t *testing.T) {
	RegisterTestingT(t)

	rawMulaw := []byte{0x01, 0x02, 0x03, 0x04}
	wav := WrapMulawInWAV(rawMulaw)

	// StripWAVHeader should recover the original data
	stripped := StripWAVHeader(wav)
	Expect(stripped).To(Equal(rawMulaw))
}

func TestChunkAudio_DefaultFrameSize(t *testing.T) {
	RegisterTestingT(t)

	data := make([]byte, 160)
	chunks := ChunkAudio(data, 0) // should default to 160
	Expect(chunks).To(HaveLen(1))
}
