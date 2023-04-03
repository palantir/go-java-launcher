package launchlib_test

import (
	"io/fs"
	"testing"

	"github.com/palantir/go-java-launcher/launchlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	lowCPUSharesContent  = []byte(`100`)
	highCPUSharesContent = []byte(`10000`)
	badCPUSharesContent  = []byte(``)
)

func TestScalingRAMPercenter_GetRAMPercentage(t *testing.T) {
	for _, test := range []struct {
		name            string
		filesystem      fs.FS
		expectedPercent float64
		expectedError   error
	}{
		{},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := launchlib.NewScalingRAMPercenter(test.filesystem)
			percent, err := s.RAMPercent()
			if test.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expectedPercent, percent)
		})
	}
}
