package admin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateContactPageSettings(t *testing.T) {
	require.NoError(t, validateContactPageSettings("", "", ""))
	require.NoError(t, validateContactPageSettings("data:image/png;base64,AAAA", "https://qm.qq.com/q/YqYR2U0gMw", "群号：1102643327"))
	require.NoError(t, validateContactPageSettings("https://cdn.example.com/qr.png", "", strings.Repeat("字", contactPageTextMaxRunes)))

	require.Error(t, validateContactPageSettings("data:image/png;base64,"+strings.Repeat("A", contactPageImageMaxBytes), "", ""))
	require.Error(t, validateContactPageSettings("javascript:alert(1)", "", ""))
	require.Error(t, validateContactPageSettings("", "javascript:alert(1)", ""))
	require.Error(t, validateContactPageSettings("", "qm.qq.com/q/abc", ""))
	require.Error(t, validateContactPageSettings("", "", strings.Repeat("字", contactPageTextMaxRunes+1)))
}
