package lists

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReverse(t *testing.T) {
	require.Equal(t, ReverseString(""), "")
	require.Equal(t, ReverseString("X"), "X")
	require.Equal(t, ReverseString("b\u0301"), "b\u0301")
	require.Equal(t, ReverseString("😎⚽"), "⚽😎")
	require.Equal(t, ReverseString("Les Mise\u0301rables"), "selbare\u0301siM seL")
	require.Equal(t, ReverseString("ab\u0301cde"), "edcb\u0301a")
	require.Equal(t, ReverseString("This `\xc5` is an invalid UTF8 character"), "retcarahc 8FTU dilavni na si `�` sihT")
	require.Equal(t, ReverseString("The quick bròwn 狐 jumped over the lazy 犬"), "犬 yzal eht revo depmuj 狐 nwòrb kciuq ehT")
	require.Equal(t, ReverseString("google.com"), "moc.elgoog")
}
