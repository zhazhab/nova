package middleware

import "nova/internal/agent"

// SSEEventMiddlewareChain applies outbound SSE middleware in registration order.
type SSEEventMiddlewareChain struct {
	middlewares []SSEEventMiddleware
}

// NewSSEEventMiddlewareChain creates the default outbound SSE middleware chain.
func NewSSEEventMiddlewareChain() *SSEEventMiddlewareChain {
	return newSSEEventMiddlewareChainWithMiddlewares(
		newWriteFileChapterBodySSEMiddleware(),
	)
}

func newSSEEventMiddlewareChainWithMiddlewares(middlewares ...SSEEventMiddleware) *SSEEventMiddlewareChain {
	return &SSEEventMiddlewareChain{middlewares: append([]SSEEventMiddleware(nil), middlewares...)}
}

// Next returns a handler that runs middleware before the final SSE writer.
func (c *SSEEventMiddlewareChain) Next(final SSEEventHandler) SSEEventHandler {
	if final == nil {
		final = func(agent.Event) error { return nil }
	}
	if c == nil {
		return final
	}
	wrapped := final
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		middleware := c.middlewares[i]
		if middleware == nil {
			continue
		}
		wrapped = middleware.Next(wrapped)
	}
	return wrapped
}
