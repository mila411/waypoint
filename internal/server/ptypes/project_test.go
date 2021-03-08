package ptypes

import (
	"testing"

	"github.com/stretchr/testify/require"

	pb "github.com/hashicorp/waypoint/internal/server/gen"
)

func TestValidateProject(t *testing.T) {
	cases := []struct {
		Name   string
		Modify func(*pb.Project)
		Error  string
	}{
		{
			"valid",
			nil,
			"",
		},

		{
			"name is not set",
			func(v *pb.Project) {
				v.Name = ""
			},
			"name: cannot be blank",
		},

		{
			"polling set but disabled",
			func(v *pb.Project) {
				v.DataSourcePoll = &pb.Project_Poll{Enabled: false}
			},
			"",
		},

		{
			"polling interval is invalid",
			func(v *pb.Project) {
				v.DataSourcePoll = &pb.Project_Poll{
					Enabled:  true,
					Interval: "very long",
				}
			},
			"invalid duration",
		},

		{
			"polling interval is valid",
			func(v *pb.Project) {
				v.DataSourcePoll = &pb.Project_Poll{
					Enabled:  true,
					Interval: "5m",
				}
			},
			"",
		},

		{
			"data source git with no URL",
			func(v *pb.Project) {
				v.DataSource = &pb.Job_DataSource{
					Source: &pb.Job_DataSource_Git{
						Git: &pb.Job_Git{
							Url: "",
						},
					},
				}
			},
			"url: cannot be blank",
		},
	}

	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			require := require.New(t)

			value := TestProject(t, nil)
			if f := tt.Modify; f != nil {
				f(value)
			}

			err := ValidateProject(value)
			if tt.Error == "" {
				require.NoError(err)
				return
			}

			require.Error(err)
			require.Contains(err.Error(), tt.Error)
		})
	}
}
