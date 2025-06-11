package ctxutil

import "context"

func Next[T any](ctx context.Context, channel chan T) (out T, ok bool) {
	select {
	case out = <-channel:
		return out, true
	case <-ctx.Done():
		return out, false
	}
}
