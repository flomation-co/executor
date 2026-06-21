package twilio_common

import (
	"encoding/binary"
	"math"
)

// mulaw decode table: maps 8-bit mulaw to 16-bit linear PCM.
// Used for energy calculation in VAD.
var mulawDecodeTable [256]int16

func init() {
	for i := 0; i < 256; i++ {
		mulawDecodeTable[i] = decodeMulaw(byte(i))
	}
}

// decodeMulaw converts a single mulaw byte to a 16-bit linear PCM sample.
func decodeMulaw(b byte) int16 {
	b = ^b
	sign := int16(b & 0x80)
	exponent := int(b>>4) & 0x07
	mantissa := int(b & 0x0F)
	sample := int16((mantissa<<3 | 0x84) << exponent)
	sample -= 0x84
	if sign != 0 {
		return -sample
	}
	return sample
}

// MulawEnergy computes the RMS energy of a mulaw audio frame.
// Returns a normalised value between 0.0 and 1.0.
func MulawEnergy(frame []byte) float64 {
	if len(frame) == 0 {
		return 0
	}
	var sumSquares float64
	for _, b := range frame {
		sample := float64(mulawDecodeTable[b])
		sumSquares += sample * sample
	}
	rms := math.Sqrt(sumSquares / float64(len(frame)))
	// Normalise: max 16-bit PCM is 32767
	return rms / 32767.0
}

// StripWAVHeader removes a RIFF/WAV header from audio data if present.
// Twilio Media Streams require raw mulaw samples without file headers.
func StripWAVHeader(data []byte) []byte {
	if len(data) < 44 {
		return data
	}
	// Check for RIFF header
	if data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' {
		// Standard WAV header is 44 bytes, but can vary.
		// Find the "data" chunk marker.
		for i := 12; i < len(data)-8; i++ {
			if data[i] == 'd' && data[i+1] == 'a' && data[i+2] == 't' && data[i+3] == 'a' {
				// Data starts 8 bytes after "data" marker (4 for "data" + 4 for size)
				dataStart := i + 8
				if dataStart < len(data) {
					return data[dataStart:]
				}
			}
		}
		// Fallback: skip standard 44-byte header
		return data[44:]
	}
	return data
}

// WrapMulawInWAV wraps raw mulaw audio samples in a WAV container header.
// This makes the audio recognisable by STT services (ElevenLabs, etc.)
// that expect a proper audio file format rather than raw samples.
// Format: RIFF/WAV, mulaw encoding (format code 7), 8000Hz, mono.
func WrapMulawInWAV(rawMulaw []byte) []byte {
	dataSize := uint32(len(rawMulaw))

	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], 36+dataSize)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)   // fmt chunk size
	binary.LittleEndian.PutUint16(header[20:22], 7)    // audio format: 7 = mulaw
	binary.LittleEndian.PutUint16(header[22:24], 1)    // channels: mono
	binary.LittleEndian.PutUint32(header[24:28], 8000) // sample rate
	binary.LittleEndian.PutUint32(header[28:32], 8000) // byte rate
	binary.LittleEndian.PutUint16(header[32:34], 1)    // block align
	binary.LittleEndian.PutUint16(header[34:36], 8)    // bits per sample
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], dataSize)

	return append(header, rawMulaw...)
}

// ChunkAudio splits audio data into frames of the specified size.
// For mulaw 8kHz, a 20ms frame is 160 bytes.
func ChunkAudio(data []byte, frameSize int) [][]byte {
	if frameSize <= 0 {
		frameSize = 160 // 20ms at 8kHz
	}
	var chunks [][]byte
	for i := 0; i < len(data); i += frameSize {
		end := i + frameSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}
	return chunks
}
