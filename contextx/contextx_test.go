package contextx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), "k1", "v1")
	ctx1 := UpdateValue(ctx, "k1", "v2")
	require.Equal(t, "v2", ctx1.Value("k1"))
	require.Equal(t, "v1", ctx.Value("k1")) // will not be changed

	ctx2 := UpdateValue(ctx1, "k1", "v3")
	require.Same(t, ctx1, ctx2) // reusing the same context
	require.Equal(t, "v3", ctx1.Value("k1"))
	require.Equal(t, "v3", ctx2.Value("k1"))
	require.Equal(t, "v1", ctx.Value("k1")) // will not be changed

	ctx3 := context.WithValue(ctx2, "k2", "v1") // with one more std context
	require.Equal(t, "v3", ctx3.Value("k1"))    // still able to get the val
}
