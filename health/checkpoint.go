package health

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// CheckCheckpointProvider preflights the checkpoint provider URL.
// It uses /eth/v1/beacon/genesis per C16 (Checkpointz serves this, but 404s on headers/head).
func CheckCheckpointProvider(ctx context.Context, providerURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, providerURL+"/eth/v1/beacon/genesis", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("provider unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("provider returned status %d", resp.StatusCode)
	}

	return nil
}
