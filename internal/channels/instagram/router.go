package instagram

import (
	"log/slog"
	"net/http"
	"sync"
)

// globalRouter manages the single webhook route for all Instagram instances.
// Only the first Instagram instance mounts the webhook handler.
var globalRouter = &webhookRouter{
	instances: make(map[string]*Channel),
}

// webhookRouter manages the shared webhook endpoint for Instagram.
type webhookRouter struct {
	mu        sync.RWMutex
	instances map[string]*Channel
	handler   http.Handler
}

// register adds an Instagram channel instance to the router.
// The first instance to register becomes the webhook handler.
func (r *webhookRouter) register(ch *Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.instances[ch.instagramID] = ch

	// First instance: set as handler.
	if r.handler == nil {
		r.handler = ch.webhookH
		slog.Debug("instagram: first instance registered as webhook handler",
			"instagram_id", ch.instagramID)
	}
}

// unregister removes an Instagram channel instance. If the removed instance
// currently owned the webhook handler, promote another registered instance.
func (r *webhookRouter) unregister(instagramID string, ch *Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.instances, instagramID)

	// If the removed instance owned the handler, clear it so a replacement can take over.
	if ch != nil && r.handler == ch.webhookH {
		r.handler = nil
	}

	if r.handler == nil {
		for _, next := range r.instances {
			r.handler = next.webhookH
			slog.Debug("instagram: promoted new webhook handler",
				"instagram_id", next.instagramID)
			break
		}
	}
}

// webhookRoute returns the path and handler for the webhook endpoint.
// Returns ("", nil) if this instance is not the handler.
func (r *webhookRouter) webhookRoute() (string, http.Handler) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.handler == nil {
		return "", nil
	}
	return webhookPath, r.handler
}
