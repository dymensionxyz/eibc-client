package eibc

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_nodeClient_nodeBlockValidated(t *testing.T) {
	type fields struct {
		client                  *http.Client
		locations               []string
		expectedValidationLevel validationLevel
	}
	type args struct {
		height int64
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
				client:                  &http.Client{},
				locations:               []string{"http://localhost:26657"},
				expectedValidationLevel: validationLevelP2P,
			},
			args: args{
				height: 1,
			},
			want:    true,
			wantErr: assert.NoError,
		}, {
			name: "failure: p2p at height 100",
			fields: fields{
				client:                  &http.Client{},
				locations:               []string{"http://localhost:26657"},
				expectedValidationLevel: validationLevelP2P,
			},
			args: args{
				height: 100,
			},
			want:    false,
			wantErr: assert.NoError,
		}, {
			name: "failure: settlement at height 1",
			fields: fields{
				client:                  &http.Client{},
				locations:               []string{"http://localhost:26657"},
				expectedValidationLevel: validationLevelSettlement,
			},
			args: args{
				height: 1,
			},
			want:    false,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &nodeClient{
				client:                  tt.fields.client,
				locations:               tt.fields.locations,
				expectedValidationLevel: tt.fields.expectedValidationLevel,
			}
			c.get = c.getHttp

			ctx := context.Background()
			got, err := c.nodeBlockValidated(ctx, tt.fields.locations[0], tt.args.height)
			if !tt.wantErr(t, err, fmt.Sprintf("nodeBlockValidated(%v, %v)", tt.fields.locations[0], tt.args.height)) {
				return
			}
			assert.Equalf(t, tt.want, got, "nodeBlockValidated(%v, %v)", tt.fields.locations[0], tt.args.height)
		})
	}
}
