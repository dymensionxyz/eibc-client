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

	"github.com/dymensionxyz/eibc-client/config"
)

type nodeClient struct {
	client          *http.Client
	rollapps        map[string]config.RollappConfig
	lastValidHeight map[string]valid
	get             getFn
}

type valid struct {
	height        int64
	confirmations int
}

type getFn func(ctx context.Context, url string) (*blockValidatedResponse, error)

type blockValidatedResponse struct {
	ChainID string          `json:"ChainID"`
	Result  validationLevel `json:"Result"`
}

type JSONResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		ChainID string `json:"ChainID"`
		Result  string `json:"Result"`
	} `json:"result"`
	Id int `json:"id"`
}

type validationLevel int

const (
	validationLevelNone validationLevel = iota
	validationLevelP2P
	validationLevelSettlement
)

func newNodeClient(rollapps map[string]config.RollappConfig) (*nodeClient, error) {
	for rollappID, cfg := range rollapps {
		if cfg.MinConfirmations == 0 {
			return nil, fmt.Errorf(
				"rollapp ID %s: minimum validated nodes must be greater than 0",
				rollappID,
			)
		}
		if len(cfg.FullNodes) < cfg.MinConfirmations {
			return nil, fmt.Errorf(
				"rollapp ID %s: not enough locations to validate blocks",
				rollappID,
			)
		}
	}
	n := &nodeClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rollapps:        rollapps,
		lastValidHeight: make(map[string]valid),
	}
	n.get = n.getHttp
	return n, nil
}

const (
	blockValidatedPath = "/block_validated"
)

func (c *nodeClient) BlockValidated(
	ctx context.Context,
	rollappID string,
	height int64,
	expectedValidationLevel validationLevel,
) (bool, error) {
	rollappConfig := c.rollapps[rollappID]

	if lv := c.lastValidHeight[rollappID]; height <= lv.height && lv.confirmations >= rollappConfig.MinConfirmations {
		return true, nil
	}

	var validatedNodes int32
	var wg sync.WaitGroup
	errChan := make(chan error, len(rollappConfig.FullNodes))

	for _, location := range rollappConfig.FullNodes {
		wg.Add(1)
		go func(ctx context.Context, rollappID, location string) {
			defer wg.Done()
			valid, err := c.nodeBlockValidated(
				ctx,
				rollappID,
				location,
				height,
				expectedValidationLevel,
			)
			if err != nil {
				errChan <- err
				return
			}
			if valid {
				atomic.AddInt32(&validatedNodes, 1)
			}
		}(ctx, rollappID, location)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return false, <-errChan
	}

	c.lastValidHeight[rollappID] = valid{
		height:        height,
		confirmations: int(validatedNodes),
	}

	return int(validatedNodes) >= rollappConfig.MinConfirmations, nil
}

func (c *nodeClient) nodeBlockValidated(
	ctx context.Context,
	rollappID,
	location string,
	height int64,
	expectedValidationLevel validationLevel,
) (bool, error) {
	url := fmt.Sprintf("%s%s?height=%d", location, blockValidatedPath, height)
	validated, err := c.get(ctx, url)
	if err != nil {
		return false, err
	}

	if validated.ChainID != rollappID {
		return false, fmt.Errorf(
			"invalid chain ID! want: %s, got: %s, height: %d",
			rollappID,
			validated.ChainID,
			height,
		)
	}

	return validated.Result >= expectedValidationLevel, nil
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
		ChainID: jsonResp.Result.ChainID,
		Result:  validationLevel(result),
	}
	return blockValidated, err
}
