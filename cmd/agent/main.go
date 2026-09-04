// Command oshimai-agent is a load-generation worker for multi-region testing: it registers with
// an Oshimai control plane, long-polls for work, executes assignments locally using the same
// pkg/loadengine the server itself uses, and reports results back. Run one of these in each
// region you want real geographic load/latency data from.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/oshimai/twin/pkg/loadengine"
	"github.com/oshimai/twin/pkg/vusession"
)

type assignment struct {
	RunID    string                  `json:"run_id"`
	Scenario *vusession.Scenario     `json:"scenario"`
	Load     loadengine.EngineConfig `json:"load_config"`
}

func main() {
	server := flag.String("server", "http://localhost:8080", "Oshimai control plane base URL")
	region := flag.String("region", "unspecified", "Region label reported to the control plane, e.g. jakarta, singapore, us-east")
	id := flag.String("id", "", "Agent ID (default: hostname-region)")
	flag.Parse()

	agentID := *id
	if agentID == "" {
		host, _ := os.Hostname()
		agentID = fmt.Sprintf("%s-%s", strings.ToLower(host), *region)
	}

	client := &http.Client{Timeout: 40 * time.Second}

	if err := register(client, *server, agentID, *region); err != nil {
		log.Fatalf("[oshimai-agent] registration failed: %v", err)
	}
	log.Printf("[oshimai-agent] registered as %q (region=%s), polling %s for work...", agentID, *region, *server)

	for {
		a, ok, err := poll(client, *server, agentID)
		if err != nil {
			log.Printf("[oshimai-agent] poll error: %v (retrying in 5s)", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if !ok {
			continue // Long-poll timed out with no work — poll again immediately.
		}

		log.Printf("[oshimai-agent] received assignment for run %s: %d VUs, %v", a.RunID, a.Load.VUs, a.Load.Duration)
		summary, runErr := execute(a)
		if runErr != nil {
			log.Printf("[oshimai-agent] execution failed: %v", runErr)
			submitResult(client, *server, agentID, a.RunID, nil, runErr.Error())
			continue
		}
		log.Printf("[oshimai-agent] run %s complete: %d requests, %.1f rps", a.RunID, summary.TotalRequests, summary.ActualRPS)
		submitResult(client, *server, agentID, a.RunID, summary, "")
	}
}

func register(client *http.Client, server, id, region string) error {
	body, _ := json.Marshal(map[string]string{"id": id, "region": region})
	resp, err := client.Post(server+"/api/v1/agents/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func poll(client *http.Client, server, id string) (assignment, bool, error) {
	resp, err := client.Get(server + "/api/v1/agents/" + id + "/poll")
	if err != nil {
		return assignment{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return assignment{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return assignment{}, false, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var a assignment
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return assignment{}, false, err
	}
	return a, true, nil
}

func execute(a assignment) (*loadengine.ExecutionSummary, error) {
	engine, err := loadengine.NewLoadEngine(a.Load)
	if err != nil {
		return nil, fmt.Errorf("engine init failed: %w", err)
	}
	return engine.Run(context.Background(), a.Scenario)
}

func submitResult(client *http.Client, server, agentID, runID string, summary *loadengine.ExecutionSummary, errMsg string) {
	body, _ := json.Marshal(map[string]any{
		"run_id": runID, "agent_id": agentID, "summary": summary, "error": errMsg,
	})
	resp, err := client.Post(server+"/api/v1/agents/"+agentID+"/results", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[oshimai-agent] failed to submit result: %v", err)
		return
	}
	resp.Body.Close()
}
