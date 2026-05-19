package catalog

import (
	"encoding/base64"
	"fmt"
	"strconv"

	codecsh264 "github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	codecsh265 "github.com/bluenviron/mediacommon/v2/pkg/codecs/h265"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediamtx/internal/stream"
)

// Catalog matches draft-ietf-moq-msf-00.
type Catalog struct {
	Version int            `json:"version"`
	Tracks  []CatalogTrack `json:"tracks"`
}

// CatalogTrack matches draft-ietf-moq-msf-00 track fields.
type CatalogTrack struct {
	Name       string  `json:"name"`
	Packaging  string  `json:"packaging"`
	IsLive     bool    `json:"isLive"`
	Namespace  string  `json:"namespace,omitempty"`
	Codec      string  `json:"codec,omitempty"`
	Bitrate    int     `json:"bitrate,omitempty"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	Framerate  float64 `json:"framerate,omitempty"`
	Samplerate int     `json:"samplerate,omitempty"`
	Channels   int     `json:"channels,omitempty"`
	ClockRate  int     `json:"clockrate,omitempty"`
	InitData   string  `json:"initData,omitempty"` // base64 AVCDecoderConfigurationRecord / similar
}

// FromStream populates the catalog from a live stream description.
func (c *Catalog) FromStream(strm *stream.Stream) {
	var tracks []CatalogTrack
	i := 0
	for _, media := range strm.Desc.Medias {
		for _, forma := range media.Formats {
			tracks = append(tracks, trackFromFormat(i, forma))
			i++
		}
	}
	c.Version = 1
	c.Tracks = tracks
}

func trackFromFormat(idx int, forma format.Format) CatalogTrack {
	ct := CatalogTrack{
		Name:      strconv.Itoa(idx),
		Packaging: "loc",
		IsLive:    true,
		ClockRate: forma.ClockRate(),
	}

	switch f := forma.(type) {
	case *format.H264:
		sps, pps := f.SafeParams()
		if len(sps) >= 4 {
			ct.Codec = fmt.Sprintf("avc1.%02X%02X%02X", sps[1], sps[2], sps[3])
		} else {
			ct.Codec = "avc1.640028"
		}
		if sps != nil {
			var s codecsh264.SPS
			if err := s.Unmarshal(sps); err == nil {
				ct.Width = s.Width()
				ct.Height = s.Height()
			}
		}
		if sps != nil && pps != nil {
			ct.InitData = base64.StdEncoding.EncodeToString(buildAVCConfig(sps, pps))
		}

	case *format.H265:
		_, sps, _ := f.SafeParams()
		if sps != nil {
			var s codecsh265.SPS
			if err := s.Unmarshal(sps); err == nil {
				ct.Width = s.Width()
				ct.Height = s.Height()
			}
		}
		ct.Codec = "hvc1.1.6.L93.B0"

	case *format.AV1:
		ct.Codec = "av01.0.04M.08"

	case *format.VP9:
		ct.Codec = "vp09.00.10.08"

	case *format.VP8:
		ct.Codec = "vp8"

	case *format.Opus:
		ct.Codec = "opus"
		ct.Samplerate = 48000
		ct.Channels = f.ChannelCount

	case *format.MPEG4Audio:
		ct.Codec = "mp4a.40.2"
		if f.Config != nil {
			ct.Samplerate = f.Config.SampleRate
			ct.Channels = f.Config.ChannelCount
			if configBytes, err := f.Config.Marshal(); err == nil {
				ct.InitData = base64.StdEncoding.EncodeToString(configBytes)
			}
		}

	case *format.AC3:
		ct.Codec = "ac-3"
		ct.Samplerate = f.SampleRate
		ct.Channels = f.ChannelCount

	case *format.G711:
		if f.MULaw {
			ct.Codec = "mulaw"
		} else {
			ct.Codec = "alaw"
		}
		ct.Samplerate = f.SampleRate

	case *format.LPCM:
		ct.Codec = "pcm"
		ct.Samplerate = f.SampleRate
		ct.Channels = f.ChannelCount
	}

	return ct
}

// buildAVCConfig builds an AVCDecoderConfigurationRecord from SPS and PPS NAL units.
// TODO: use mp4.AVCDecoderConfiguration ?
func buildAVCConfig(sps, pps []byte) []byte {
	b := make([]byte, 0, 11+len(sps)+len(pps))
	b = append(b, 0x01)
	b = append(b, sps[1], sps[2], sps[3])
	b = append(b, 0xFF) // lengthSizeMinusOne = 3 → 4-byte lengths
	b = append(b, 0xE1) // numSPS = 1
	b = append(b, byte(len(sps)>>8), byte(len(sps)))
	b = append(b, sps...)
	b = append(b, 0x01) // numPPS = 1
	b = append(b, byte(len(pps)>>8), byte(len(pps)))
	b = append(b, pps...)
	return b
}
