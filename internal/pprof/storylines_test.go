package pprof

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFirstAppFrame(t *testing.T) {
	stack := []string{
		"runtime.goexit",
		"net/http.(*Server).Serve",
		"gitlab.com/ductone/c1/pkg/api.DoThing",
		"github.com/conductorone/other/pkg.Run",
	}
	prefixes := []string{"gitlab.com/ductone/c1"}
	frame := firstAppFrame(stack, prefixes)
	require.Equal(t, "gitlab.com/ductone/c1/pkg/api.DoThing", frame)
}

