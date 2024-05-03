package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type denomFetcher struct {
	sync.Mutex
	pathMap map[string]string
	client  cosmosclient.Client
}

func newDenomFetcher(client cosmosclient.Client) *denomFetcher {
	return &denomFetcher{
		pathMap: make(map[string]string),
		client:  client,
	}

}

func (d *denomFetcher) getDenomFromPath(ctx context.Context, path, destChannel string) (string, error) {
	zeroChannelPrefix := "transfer/channel-0/"

	// Remove "transfer/channel-0/" prefix if it exists
	path = strings.TrimPrefix(path, zeroChannelPrefix)

	if path == defaultHubDenom {
		return defaultHubDenom, nil
	}

	// for denoms other than adym we should have a full path
	// in order to be albe to derive the ibc denom hash
	if !strings.Contains(path, "channel-") {
		path = fmt.Sprintf("transfer/%s/%s", destChannel, path)
	}

	d.Lock()
	defer d.Unlock()

	denom, ok := d.pathMap[path]
	if ok {
		return denom, nil
	}

	queryClient := types.NewQueryClient(d.client.Context())

	req := &types.QueryDenomHashRequest{
		Trace: path,
	}

	res, err := queryClient.DenomHash(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to query denom hash: %w", err)
	}

	ibcDenom := fmt.Sprintf("ibc/%s", res.Hash)

	d.pathMap[path] = ibcDenom

	return ibcDenom, nil
}
