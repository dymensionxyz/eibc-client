package eibc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type nodeClient struct {
	client                  *http.Client
	locations               []string
	expectedValidationLevel validationLevel
	minimumValidatedNodes   int
	get                     getFn
}

type getFn func(ctx context.Context, url string) (*blockValidatedResponse, error)

type blockValidatedResponse struct {
	Result validationLevel `json:"Result"`
}

type JSONResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Result string `json:"Result"`
	} `json:"result"`
	Id int `json:"id"`
}

type validationLevel int

const (
	validationLevelNone validationLevel = iota
	validationLevelP2P
	validationLevelSettlement
)

func newNodeClient(
	locations []string,
	expectedValidationLevel validationLevel,
	minimumValidatedNodes int,
) (*nodeClient, error) {
	if minimumValidatedNodes == 0 {
		return nil, fmt.Errorf("minimum validated nodes must be greater than 0")
	}
	if len(locations) < minimumValidatedNodes {
		return nil, fmt.Errorf("not enough locations to validate blocks")
	}
	n := &nodeClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		locations:               locations,
		expectedValidationLevel: expectedValidationLevel,
		minimumValidatedNodes:   minimumValidatedNodes,
	}
	n.get = n.getHttp
	return n, nil
}

const (
	blockValidatedPath = "/block_validated"
)

func (c *nodeClient) BlockValidated(ctx context.Context, height int64) (bool, error) {
	var validatedNodes int32
	var wg sync.WaitGroup
	errChan := make(chan error, len(c.locations))

	for _, location := range c.locations {
		wg.Add(1)
		go func(ctx context.Context, location string) {
			defer wg.Done()
			valid, err := c.nodeBlockValidated(ctx, location, height)
			if err != nil {
				errChan <- err
				return
			}
			if valid {
				atomic.AddInt32(&validatedNodes, 1)
			}
		}(ctx, location)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return false, <-errChan
	}

	return int(validatedNodes) >= c.minimumValidatedNodes, nil
}

func (c *nodeClient) nodeBlockValidated(ctx context.Context, location string, height int64) (bool, error) {
	url := fmt.Sprintf("%s%s?height=%d", location, blockValidatedPath, height)
	validated, err := c.get(ctx, url)
	if err != nil {
		return false, err
	}
	return validated.Result >= c.expectedValidationLevel, nil
}

func (c *nodeClient) getHttp(ctx context.Context, url string) (*blockValidatedResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", url, err)
	}
	defer resp.Body.Close()

	jsonResp := new(JSONResponse)
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result, err := strconv.ParseInt(jsonResp.Result.Result, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}
	blockValidated := &blockValidatedResponse{
		Result: validationLevel(result),
	}
	return blockValidated, err
}
