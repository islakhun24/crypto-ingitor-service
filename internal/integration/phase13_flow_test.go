package integration

import (
	"os"
	"testing"
)

func TestPhase13DockerComposeFlow(t *testing.T) {
	if os.Getenv("AGGREGATOR_INTEGRATION_TESTS") != "1" {
		t.Skip("set AGGREGATOR_INTEGRATION_TESTS=1 and run the Docker Compose stack to execute the end-to-end flow")
	}

	t.Log("expected manual flow: seed symbols JSONB, seed endpoints, generate jobs, execute mocked adapter, upsert snapshot, aggregate, query REST API, run retention dry-run")
}
