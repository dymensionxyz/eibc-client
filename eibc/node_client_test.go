package eibc

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dymensionxyz/eibc-client/config"
)

func Test_nodeClient_nodeBlockValidated(t *testing.T) {
	t.Skip()

	type fields struct {
		client                  *http.Client
		rollapps                map[string]config.RollappConfig
		expectedValidationLevel validationLevel
	}
	type args struct {
		height    int64
		rollappID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success: p2p at height 1",
			fields: fields{
				client: &http.Client{},
				rollapps: map[string]config.RollappConfig{
					"rollapp1": {
						MinConfirmations: 1,
						FullNodes:        []string{"http://localhost:26657"},
					},
				},
				expectedValidationLevel: validationLevelP2P,
			},
			args: args{
				height:    1,
				rollappID: "rollapp1",
			},
			want:    true,
			wantErr: assert.NoError,
		}, {
			name: "failure: p2p at height 100",
			fields: fields{
				client: &http.Client{},
				rollapps: map[string]config.RollappConfig{
					"rollapp1": {
						MinConfirmations: 1,
						FullNodes:        []string{"http://localhost:26657"},
					},
				},
				expectedValidationLevel: validationLevelP2P,
			},
			args: args{
				height:    100,
				rollappID: "rollapp1",
			},
			want:    false,
			wantErr: assert.NoError,
		}, {
			name: "failure: settlement at height 1",
			fields: fields{
				client: &http.Client{},
				rollapps: map[string]config.RollappConfig{
					"rollapp1": {
						MinConfirmations: 1,
						FullNodes:        []string{"http://localhost:26657"},
					},
				},
				expectedValidationLevel: validationLevelSettlement,
			},
			args: args{
				height:    1,
				rollappID: "rollapp1",
			},
			want:    false,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &nodeClient{
				client:   tt.fields.client,
				rollapps: tt.fields.rollapps,
			}
			c.get = c.getHttp

			ctx := context.Background()
			got, err := c.nodeBlockValidated(ctx, tt.args.rollappID, tt.fields.rollapps[tt.args.rollappID].FullNodes[0], tt.args.height, tt.fields.expectedValidationLevel)
			if !tt.wantErr(t, err, fmt.Sprintf("nodeBlockValidated(%s, %s, %v)", tt.args.rollappID, tt.fields.rollapps[tt.args.rollappID].FullNodes[0], tt.args.height)) {
				return
			}
			assert.Equalf(t, tt.want, got, "nodeBlockValidated(%s, %s, %v)", tt.args.rollappID, tt.fields.rollapps[tt.args.rollappID].FullNodes[0], tt.args.height)
		})
	}
}
