package media

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTimecodeToMs(t *testing.T) {
	cases := map[string]int64{
		"00:00:00,000": 0,
		"00:00:00,005": 5,    // regression: was corrupted to 500
		"00:00:01,050": 1050, // regression: was corrupted to 1500
		"00:00:03,090": 3090, // regression: was corrupted to 3900
		"00:00:00,500": 500,
		"00:00:01,100": 1100,
		"01:02:03,456": 3723456,
		"00:00:02.250": 2250, // VTT-style dot separator
	}
	for in, want := range cases {
		got, err := parseTimecodeToMs(in)
		require.NoError(t, err, in)
		require.Equal(t, want, got, in)
	}
}

func TestParseSRTTimestamps(t *testing.T) {
	srt := "1\n00:00:01,050 --> 00:00:03,090\nesto es la cocina\n"
	segs, err := parseSRT(srt, 5)
	require.NoError(t, err)
	require.Len(t, segs, 1)
	require.Equal(t, int64(1050), segs[0].StartMs)
	require.Equal(t, int64(3090), segs[0].EndMs)
	require.Equal(t, "esto es la cocina", segs[0].Text)
}
