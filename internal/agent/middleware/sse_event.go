package middleware

import "nova/internal/agent"

// SSEEventHandler writes or forwards one outbound SSE event.
type SSEEventHandler func(agent.Event) error

// SSEEventMiddleware chains the next SSE output handler. A middleware forwards an
// event by calling next, mutates output by passing a copied event to next, and
// suppresses output by returning without calling next.
type SSEEventMiddleware interface {
	Next(SSEEventHandler) SSEEventHandler
}

// SSEEventMiddlewareChainOptions controls which optional outbound SSE
// middlewares are installed for a stream.
type SSEEventMiddlewareChainOptions struct {
	HideChapterBodyLiveOutput bool
}
