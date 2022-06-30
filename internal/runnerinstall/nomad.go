package runnerinstall

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
	"github.com/hashicorp/waypoint/internal/clierrors"
	nomadutil "github.com/hashicorp/waypoint/internal/installutil/nomad"
	"github.com/hashicorp/waypoint/internal/pkg/flag"
	"k8s.io/apimachinery/pkg/util/wait"
)

type NomadRunnerInstaller struct {
	Config NomadConfig
}

type NomadConfig struct {
	AuthSoftFail       bool              `hcl:"auth_soft_fail,optional"`
	Namespace          string            `hcl:"namespace,optional"`
	ServiceAnnotations map[string]string `hcl:"service_annotations,optional"`

	RunnerImage string `hcl:"runner_image,optional"`
	OdrImage    string `hcl:"odr_image,optional"`

	Region         string   `hcl:"namespace,optional"`
	Datacenters    []string `hcl:"datacenters,optional"`
	PolicyOverride bool     `hcl:"policy_override,optional"`

	RunnerResourcesCPU    string `hcl:"runner_resources_cpu,optional"`
	RunnerResourcesMemory string `hcl:"runner_resources_memory,optional"`

	HostVolume           string            `hcl:"host_volume,optional"`
	CsiVolumeProvider    string            `hcl:"csi_volume_provider,optional"`
	CsiVolumeCapacityMin int64             `hcl:"csi_volume_capacity_min,optional"`
	CsiVolumeCapacityMax int64             `hcl:"csi_volume_capacity_max,optional"`
	CsiFS                string            `hcl:"csi_fs,optional"`
	CsiTopologies        map[string]string `hcl:"nomad_csi_topologies,optional"`
	CsiExternalId        string            `hcl:"nomad_csi_external_id,optional"`
	CsiPluginId          string            `hcl:"nomad_csi_plugin_id,required"`
	CsiSecrets           map[string]string `hcl:"nomad_csi_secrets,optional"`

	NomadHost string `hcl:"nomad_host,optional"`
}

var (
	defaultNomadHost = "http://localhost:4646"
)

func (i *NomadRunnerInstaller) Install(ctx context.Context, opts *InstallOpts) error {
	ui := opts.UI

	sg := ui.StepGroup()
	defer sg.Wait()

	s := sg.Add("Initializing Nomad client...")
	defer func() { s.Abort() }()

	//Build api client from environment
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	s.Done()

	if i.Config.CsiVolumeProvider == "" && i.Config.HostVolume == "" {
		return fmt.Errorf("please include '-nomad-csi-volume-provider' or '-nomad-host-volume'")
	} else if i.Config.CsiVolumeProvider != "" {
		if i.Config.HostVolume != "" {
			return fmt.Errorf("choose either CSI or host volume, not both")
		}

		s = sg.Add("Creating persistent volume")
		err = nomadutil.CreatePersistentVolume(
			ctx,
			client,
			"waypoint-"+opts.Id+"-runner",
			"waypoint-"+opts.Id+"-runner",
			i.Config.CsiPluginId,
			i.Config.CsiVolumeProvider,
			i.Config.CsiFS,
			i.Config.CsiExternalId,
			i.Config.CsiVolumeCapacityMin,
			i.Config.CsiVolumeCapacityMax,
			i.Config.CsiTopologies,
			i.Config.CsiSecrets,
		)
		if err != nil {
			return fmt.Errorf("error creating Nomad persistent volume: %s", clierrors.Humanize(err))
		}
		s.Update("Persistent volume created!")
		s.Status(terminal.StatusOK)
		s.Done()
	}

	// Install the runner
	s = sg.Add("Installing the Waypoint runner")
	_, err = nomadutil.RunJob(ctx, s, client, waypointRunnerNomadJob(i.Config, opts), false)
	if err != nil {
		return err
	}
	s.Update("Waypoint runner installed")
	s.Done()

	return nil
}

// waypointRunnerNomadJob takes in a NomadConfig and returns a Nomad Job
// for the Nomad API to run a Waypoint runner.
func waypointRunnerNomadJob(c NomadConfig, opts *InstallOpts) *api.Job {
	// Name AND ID of the Nomad job will be waypoint-runner-ID
	// Name is cosmetic, but ID must be unique
	jobRef := "waypoint-" + opts.Id + "-runner"
	job := api.NewServiceJob(jobRef, jobRef, c.Region, 50)
	job.Namespace = &c.Namespace
	job.Datacenters = c.Datacenters
	job.Meta = c.ServiceAnnotations
	tg := api.NewTaskGroup(runnerName, 1)
	tg.Networks = []*api.NetworkResource{
		{
			Mode: "host",
		},
	}

	// Preserve disk, otherwise upgrades will destroy previous allocation and the disk along with it
	volumeRequest := api.VolumeRequest{ReadOnly: false}
	if c.CsiVolumeProvider != "" {
		volumeRequest.Type = "csi"
		volumeRequest.Source = "waypoint-" + opts.Id + "-runner"
		volumeRequest.AccessMode = "single-node-writer"
		volumeRequest.AttachmentMode = "file-system"
	} else {
		volumeRequest.Type = "host"
		volumeRequest.Source = c.HostVolume
	}

	tg.Volumes = map[string]*api.VolumeRequest{
		runnerName: &volumeRequest,
	}

	job.AddTaskGroup(tg)

	readOnly := false
	volume := runnerName
	destination := "/data"
	volumeMounts := []*api.VolumeMount{
		{
			Volume:      &volume,
			Destination: &destination,
			ReadOnly:    &readOnly,
		},
	}

	task := api.NewTask("runner", "docker")
	task.Config = map[string]interface{}{
		"image": c.RunnerImage,
		"args": []string{
			"runner",
			"agent",
			"-id=" + opts.Id,
			"-state-dir=/data/runner",
			"-cookie=" + opts.Cookie,
			"-vv",
		},
		"auth_soft_fail": c.AuthSoftFail,
	}

	task.VolumeMounts = volumeMounts

	preTask := nomadutil.SetupPretask(volumeMounts)

	tg.AddTask(preTask)

	cpu := nomadutil.DefaultResourcesCPU
	mem := nomadutil.DefaultResourcesMemory

	if c.RunnerResourcesCPU != "" {
		cpu, _ = strconv.Atoi(c.RunnerResourcesCPU)
	}
	if c.RunnerResourcesMemory != "" {
		mem, _ = strconv.Atoi(c.RunnerResourcesMemory)
	}
	task.Resources = &api.Resources{
		CPU:      &cpu,
		MemoryMB: &mem,
	}

	task.Env = map[string]string{}
	for _, line := range opts.AdvertiseClient.Env() {
		idx := strings.Index(line, "=")
		if idx == -1 {
			// Should never happen but let's not crash.
			continue
		}

		key := line[:idx]
		value := line[idx+1:]
		task.Env[key] = value
	}

	// Let the runner know about the Nomad IP
	if c.NomadHost == "" {
		c.NomadHost = defaultNomadHost
	}
	task.Env["NOMAD_ADDR"] = c.NomadHost

	tg.AddTask(task)

	return job
}

func (i *NomadRunnerInstaller) InstallFlags(set *flag.Set) {
	set.StringSliceVar(&flag.StringSliceVar{
		Name:    "nomad-dc",
		Target:  &i.Config.Datacenters,
		Default: []string{"dc1"},
		Usage:   "Datacenters to install to for Nomad.",
	})

	set.StringVar(&flag.StringVar{
		Name:    "nomad-runner-image",
		Target:  &i.Config.RunnerImage,
		Usage:   "Docker image for the Waypoint runner.",
		Default: defaultRunnerImage,
	})

	set.StringVar(&flag.StringVar{
		Name:   "nomad-host-volume",
		Target: &i.Config.HostVolume,
		Usage:  "Nomad host volume name.",
	})

	set.StringVar(&flag.StringVar{
		Name:   "nomad-csi-volume-plugin-id",
		Target: &i.Config.CsiPluginId,
		Usage:  "The ID of the CSI plugin that manages the volume, required for volume type 'csi'.",
	})

	set.StringVar(&flag.StringVar{
		Name:   "nomad-csi-volume-provider",
		Target: &i.Config.CsiVolumeProvider,
		Usage:  "Nomad CSI volume provider, required for volume type 'csi'.",
	})

	set.Int64Var(&flag.Int64Var{
		Name:    "nomad-csi-volume-capacity-min",
		Target:  &i.Config.CsiVolumeCapacityMin,
		Usage:   "Nomad CSI volume capacity minimum, in bytes.",
		Default: nomadutil.DefaultCSIVolumeCapacityMin,
	})

	set.Int64Var(&flag.Int64Var{
		Name:    "nomad-csi-volume-capacity-max",
		Target:  &i.Config.CsiVolumeCapacityMax,
		Usage:   "Nomad CSI volume capacity maximum, in bytes.",
		Default: nomadutil.DefaultCSIVolumeCapacityMax,
	})

	set.StringVar(&flag.StringVar{
		Name:    "nomad-csi-fs",
		Target:  &i.Config.CsiFS,
		Usage:   "Nomad CSI volume mount option file system.",
		Default: nomadutil.DefaultCSIVolumeMountFS,
	})

	set.StringMapVar(&flag.StringMapVar{
		Name:   "nomad-csi-topologies",
		Target: &i.Config.CsiTopologies,
		Usage:  "Locations from which the Nomad Volume will be accessible.",
	})

	set.StringVar(&flag.StringVar{
		Name:   "nomad-csi-external-id",
		Target: &i.Config.CsiExternalId,
		Usage:  "The ID of the physical volume from the Nomad storage provider.",
	})

	set.StringMapVar(&flag.StringMapVar{
		Name:   "nomad-csi-secrets",
		Target: &i.Config.CsiSecrets,
		Usage:  "Credentials for publishing volume for Waypoint runner.",
	})
}

func (i *NomadRunnerInstaller) Uninstall(ctx context.Context, opts *InstallOpts) error {
	ui := opts.UI

	sg := ui.StepGroup()
	defer sg.Wait()

	s := sg.Add("Initializing Nomad client...")
	defer func() { s.Abort() }()

	// Build api client
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return err
	}
	s.Done()

	s = sg.Add("Locate existing Waypoint runner...")
	var waypointRunnerJobName string
	possibleRunnerJobNames := []string{
		"waypoint-" + opts.Id + "-runner",
		runnerName,
	}
	for _, runnerJobName := range possibleRunnerJobNames {
		jobs, _, err := client.Jobs().PrefixList(runnerJobName)
		if err != nil {
			s.Update("Unable to find nomad job %s for Waypoint runner", waypointRunnerJobName)
			return err
		}
		if len(jobs) > 0 {
			waypointRunnerJobName = runnerJobName
			break
		}
	}

	if waypointRunnerJobName == "" {
		s.Update("Could not find Waypoint runner in Nomad")
		return fmt.Errorf("Could not find Waypoint runner in Nomad")
	}

	s.Update("Waypoint runner found.")
	s.Done()

	s = sg.Add("Uninstalling the Waypoint runner...")
	_, _, err = client.Jobs().Deregister(waypointRunnerJobName, false, &api.WriteOptions{})
	if err != nil {
		s.Update("Unable to deregister Waypoint runner job.")
		return err
	}

	s.Update("Waiting for jobs to be stopped...")
	err = wait.PollImmediate(2*time.Second, 10*time.Minute, func() (bool, error) {
		jobs, _, err := client.Jobs().PrefixList(waypointRunnerJobName)
		if err != nil {
			return false, err
		}
		for _, job := range jobs {
			if job.Status != "dead" {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	// Delete CSI volume for runner (if it exists)
	vols, _, err := client.CSIVolumes().List(&api.QueryOptions{Prefix: waypointRunnerJobName})
	if err != nil {
		return err
	}
	for _, vol := range vols {
		if vol.ID == waypointRunnerJobName {
			s.Update("Destroying persistent CSI volume")
			err = client.CSIVolumes().Deregister(vol.ID, true, &api.WriteOptions{})
			if err != nil {
				return err
			}
			s.Update("Successfully destroyed persistent volumes")
			break
		}
	}

	_, _, err = client.Jobs().Deregister(waypointRunnerJobName, true, &api.WriteOptions{})
	if err != nil {
		s.Update("Unable to deregister Waypoint runner job.")
		return err
	}
	s.Update("Waypoint runner job and allocations purged")
	s.Done()

	return nil
}

func (i *NomadRunnerInstaller) UninstallFlags(set *flag.Set) {
}
