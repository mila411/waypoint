package server

import (
	"context"
	"testing"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	anypb "google.golang.org/protobuf/types/known/anypb"

	pb "github.com/hashicorp/waypoint/pkg/server/gen"
)

func TestGwNullAnyUnaryInterceptor(t *testing.T) {
	f := gwNullAnyUnaryInterceptor()

	t.Run("with gw metadata", func(t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		ctx = metadata.NewIncomingContext(ctx, metadata.New(map[string]string{
			gwruntime.MetadataPrefix + "yo": "yo",
		}))

		called := false
		resp, err := f(ctx, nil, &grpc.UnaryServerInfo{},
			func(ctx context.Context, req interface{}) (interface{}, error) {
				called = true
				return &pb.Build{
					Artifact: &pb.Artifact{
						Artifact: &anypb.Any{},
					},
				}, nil
			},
		)
		require.True(called)
		require.NoError(err)
		require.Equal(resp, &pb.Build{
			Artifact: &pb.Artifact{
				Artifact: nil,
			},
		})
	})
}
