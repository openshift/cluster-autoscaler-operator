package clusterautoscaler

import (
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
)

const (
	// The following values are taken from the OpenShift conventions
	// https://github.com/openshift/enhancements/blob/master/CONVENTIONS.md
	leaderElectLeaseDuration = "137s"
	leaderElectRenewDeadline = "107s"
	leaderElectRetryPeriod   = "26s"
)

// AutoscalerArg represents a command line argument to the cluster-autoscaler
// that may be combined with a value or numerical range.
type AutoscalerArg string

// String returns the argument as a plain string.
func (a AutoscalerArg) String() string {
	return string(a)
}

// Value returns the argument with the given value set.
func (a AutoscalerArg) Value(v interface{}) string {
	return fmt.Sprintf("%s=%v", a.String(), v)
}

// Range returns the argument with the given numerical range set.
func (a AutoscalerArg) Range(min, max int) string {
	return fmt.Sprintf("%s=%d:%d", a.String(), min, max)
}

// TypeRange returns the argument with the given type and numerical range set.
func (a AutoscalerArg) TypeRange(t string, min, max int) string {
	return fmt.Sprintf("%s=%s:%d:%d", a.String(), t, min, max)
}

// These constants represent the cluster-autoscaler arguments used by the
// operator when processing ClusterAutoscaler resources.
const (
	LogToStderrArg                   AutoscalerArg = "--logtostderr"
	RecordDuplicatedEventsArg        AutoscalerArg = "--record-duplicated-events"
	NamespaceArg                     AutoscalerArg = "--namespace"
	CloudProviderArg                 AutoscalerArg = "--cloud-provider"
	MaxGracefulTerminationSecArg     AutoscalerArg = "--max-graceful-termination-sec"
	ExpendablePodsPriorityCutoffArg  AutoscalerArg = "--expendable-pods-priority-cutoff"
	ScaleDownEnabledArg              AutoscalerArg = "--scale-down-enabled"
	ScaleDownDelayAfterAddArg        AutoscalerArg = "--scale-down-delay-after-add"
	ScaleDownDelayAfterDeleteArg     AutoscalerArg = "--scale-down-delay-after-delete"
	ScaleDownDelayAfterFailureArg    AutoscalerArg = "--scale-down-delay-after-failure"
	ScaleDownUnneededTimeArg         AutoscalerArg = "--scale-down-unneeded-time"
	ScaleDownUtilizationThresholdArg AutoscalerArg = "--scale-down-utilization-threshold"
	MaxNodesTotalArg                 AutoscalerArg = "--max-nodes-total"
	MaxNodeProvisionTimeArg          AutoscalerArg = "--max-node-provision-time"
	CoresTotalArg                    AutoscalerArg = "--cores-total"
	MemoryTotalArg                   AutoscalerArg = "--memory-total"
	GPUTotalArg                      AutoscalerArg = "--gpu-total"
	VerbosityArg                     AutoscalerArg = "--v"
	BalanceSimilarNodeGroupsArg      AutoscalerArg = "--balance-similar-node-groups"
	BalancingIgnoreLabelArg          AutoscalerArg = "--balancing-ignore-label"
	IgnoreDaemonsetsUtilization      AutoscalerArg = "--ignore-daemonsets-utilization"
	SkipNodesWithLocalStorage        AutoscalerArg = "--skip-nodes-with-local-storage"
	LeaderElectLeaseDurationArg      AutoscalerArg = "--leader-elect-lease-duration"
	LeaderElectRenewDeadlineArg      AutoscalerArg = "--leader-elect-renew-deadline"
	LeaderElectRetryPeriodArg        AutoscalerArg = "--leader-elect-retry-period"
	ExpanderArg                      AutoscalerArg = "--expander"
)

// The following values are for cloud providers which have not yet created specific nodegroupset processors.
// These values should be removed and replaced in the event that one of the cloud providers creates a nodegroupset processor.

// AWS cloud provider ignore labels for the autoscaler.
const (
	// AwsIgnoredLabelEksctlInstanceId  is a label used by eksctl to identify instances.
	AwsIgnoredLabelEksctlInstanceId = "alpha.eksctl.io/instance-id"

	// AwsIgnoredLabelEksctlNodegroupName is a label used by eksctl to identify "node group" names.
	AwsIgnoredLabelEksctlNodegroupName = "alpha.eksctl.io/nodegroup-name"

	// AwsIgnoredLabelEksNodegroup is a label used by eks to identify "node group".
	AwsIgnoredLabelEksNodegroup = "eks.amazonaws.com/nodegroup"

	// AwsIgnoredLabelK8sEniconfig is a label used by the AWS CNI for custom networking.
	AwsIgnoredLabelK8sEniconfig = "k8s.amazonaws.com/eniConfig"

	// AwsIgnoredLabelLifecycle is a label used by the AWS for spot.
	AwsIgnoredLabelLifecycle = "lifecycle"

	// AwsIgnoredLabelEbsCsiZone is a label used by the AWS EBS CSI driver as a target for Persistent Volume Node Affinity.
	AwsIgnoredLabelEbsCsiZone = "topology.ebs.csi.aws.com/zone"
)

// IBM cloud provider ignore labels for the autoscaler.
const (
	// IbmcloudIgnoredLabelWorkerId is a label used by the IBM Cloud Cloud Controler Manager.
	IbmcloudIgnoredLabelWorkerId = "ibm-cloud.kubernetes.io/worker-id"

	// IbmcloudIgnoredLabelVpcBlockCsi is a label used by the IBM Cloud CSI driver as a target for Persisten Volume Node Affinity.
	IbmcloudIgnoredLabelVpcBlockCsi = "vpc-block-csi-driver-labels"

	// IbmcloudIgnoredLabelVpcInstanceId on IBM Cloud when a VPC is in use.
	IbmcloudIgnoredLabelVpcInstanceId = "ibm-cloud.kubernetes.io/vpc-instance-id"
)

// GCP cloud provider ignore labels for the autoscaler.
const (
	// GceIgnoredLabelGkeZone label is used to specify the zone of the instance.
	GceIgnoredLabelGkeZone = "topology.gke.io/zone"
)

// Azure cloud provider ignore labels for the autoscaler.
const (
	// AzureDiskTopologyKey is the topology key of Azure Disk CSI driver.
	AzureDiskTopologyKey = "topology.disk.csi.azure.com/zone"

	// AzureNodepoolLegacyLabel is a label specifying which Azure node pool a particular node belongs to.
	AzureNodepoolLegacyLabel = "agentpool"

	// AzureNodepoolLabel is an AKS label specifying which nodepool a particular node belongs to.
	AzureNodepoolLabel = "kubernetes.azure.com/agentpool"
)

// Alibaba cloud provider ignore labels for the autoscaler.
const (
	// AlicloudIgnoredLabelCsiZone is a label used by the Alibaba Cloud CSI driver as a target for Persistent Volume Node Affinity.
	AlicloudIgnoredLabelCsiZone = "topology.diskplugin.csi.alibabacloud.com/zone"
)

// AppendBasicIgnoreLabels appends ignore labels for specific cloud provider to the arguments
// so the autoscaler can use these labels without the user having to input them manually.
func appendBasicIgnoreLabels(args []string, cfg *Config) []string {
	switch cfg.platformType {
	case configv1.AlibabaCloudPlatformType:
		args = append(args, BalancingIgnoreLabelArg.Value(AlicloudIgnoredLabelCsiZone))
	case configv1.AWSPlatformType:
		args = append(args, BalancingIgnoreLabelArg.Value(AwsIgnoredLabelEbsCsiZone),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelLifecycle),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelK8sEniconfig),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelEksNodegroup),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelEksctlNodegroupName),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelEksctlInstanceId))
	case configv1.AzurePlatformType:
		args = append(args, BalancingIgnoreLabelArg.Value(AzureDiskTopologyKey),
			BalancingIgnoreLabelArg.Value(AzureNodepoolLegacyLabel),
			BalancingIgnoreLabelArg.Value(AzureNodepoolLabel),
		)
	case configv1.GCPPlatformType:
		args = append(args, BalancingIgnoreLabelArg.Value(GceIgnoredLabelGkeZone))
	case configv1.IBMCloudPlatformType:
		args = append(args, BalancingIgnoreLabelArg.Value(IbmcloudIgnoredLabelWorkerId),
			BalancingIgnoreLabelArg.Value(IbmcloudIgnoredLabelVpcBlockCsi),
			BalancingIgnoreLabelArg.Value(IbmcloudIgnoredLabelVpcInstanceId))
	}

	return args
}

// AutoscalerArgs returns a slice of strings representing command line arguments
// to the cluster-autoscaler corresponding to the values in the given
// ClusterAutoscaler resource.
func AutoscalerArgs(ca *v1.ClusterAutoscaler, cfg *Config) []string {
	s := &ca.Spec

	args := []string{
		LogToStderrArg.String(),
		RecordDuplicatedEventsArg.String(),
		CloudProviderArg.Value(cfg.CloudProvider),
		NamespaceArg.Value(cfg.Namespace),
		LeaderElectLeaseDurationArg.Value(leaderElectLeaseDuration),
		LeaderElectRenewDeadlineArg.Value(leaderElectRenewDeadline),
		LeaderElectRetryPeriodArg.Value(leaderElectRetryPeriod),
	}

	if ca.Spec.MaxPodGracePeriod != nil {
		v := MaxGracefulTerminationSecArg.Value(*s.MaxPodGracePeriod)
		args = append(args, v)
	}

	if ca.Spec.MaxNodeProvisionTime != "" {
		v := MaxNodeProvisionTimeArg.Value(s.MaxNodeProvisionTime)
		args = append(args, v)
	}

	if ca.Spec.Expander != "" {
		v := ExpanderArg.Value(s.Expander)
		args = append(args, v)
	}

	if ca.Spec.PodPriorityThreshold != nil {
		v := ExpendablePodsPriorityCutoffArg.Value(*s.PodPriorityThreshold)
		args = append(args, v)
	}

	if ca.Spec.ResourceLimits != nil {
		args = append(args, ResourceArgs(s.ResourceLimits)...)
	}

	if ca.Spec.ScaleDown != nil {
		args = append(args, ScaleDownArgs(s.ScaleDown)...)
	}

	if ca.Spec.BalanceSimilarNodeGroups != nil {
		args = append(args, BalanceSimilarNodeGroupsArg.Value(*ca.Spec.BalanceSimilarNodeGroups))

		// Append basic ignore labels for a specific cloud provider.
		args = appendBasicIgnoreLabels(args, cfg)
	}

	if ca.Spec.IgnoreDaemonsetsUtilization != nil {
		args = append(args, IgnoreDaemonsetsUtilization.Value(*ca.Spec.IgnoreDaemonsetsUtilization))
	}

	if ca.Spec.SkipNodesWithLocalStorage != nil {
		args = append(args, SkipNodesWithLocalStorage.Value(*ca.Spec.SkipNodesWithLocalStorage))
	}

	for _, ignoredLabel := range ca.Spec.BalancingIgnoredLabels {
		args = append(args, BalancingIgnoreLabelArg.Value(ignoredLabel))
	}

	// Prefer log level set from ClousterAutoscaler resource
	if ca.Spec.LogVerbosity != nil {
		args = append(args, VerbosityArg.Value(*ca.Spec.LogVerbosity))
	} else {
		// From environment variable or default
		args = append(args, VerbosityArg.Value(cfg.Verbosity))
	}

	return args
}

// ScaleDownArgs returns a slice of strings representing command line arguments
// to the cluster-autoscaler corresponding to the values in the given
// ScaleDownConfig object.
func ScaleDownArgs(sd *v1.ScaleDownConfig) []string {
	if !sd.Enabled {
		return []string{ScaleDownEnabledArg.Value(false)}
	}

	args := []string{
		ScaleDownEnabledArg.Value(true),
	}

	if sd.DelayAfterAdd != nil {
		args = append(args, ScaleDownDelayAfterAddArg.Value(*sd.DelayAfterAdd))
	}

	if sd.DelayAfterDelete != nil {
		args = append(args, ScaleDownDelayAfterDeleteArg.Value(*sd.DelayAfterDelete))
	}

	if sd.DelayAfterFailure != nil {
		args = append(args, ScaleDownDelayAfterFailureArg.Value(*sd.DelayAfterFailure))
	}

	if sd.UnneededTime != nil {
		args = append(args, ScaleDownUnneededTimeArg.Value(*sd.UnneededTime))
	}

	if sd.UtilizationThreshold != nil {
		args = append(args, ScaleDownUtilizationThresholdArg.Value(*sd.UtilizationThreshold))
	}

	return args
}

// ResourceArgs returns a slice of strings representing command line arguments
// to the cluster-autoscaler corresponding to the values in the given
// ResourceLimits object.
func ResourceArgs(rl *v1.ResourceLimits) []string {
	args := []string{}

	if rl.MaxNodesTotal != nil {
		args = append(args, MaxNodesTotalArg.Value(*rl.MaxNodesTotal))
	}

	if rl.Cores != nil {
		min, max := int(rl.Cores.Min), int(rl.Cores.Max)
		args = append(args, CoresTotalArg.Range(min, max))
	}

	if rl.Memory != nil {
		min, max := int(rl.Memory.Min), int(rl.Memory.Max)
		args = append(args, MemoryTotalArg.Range(min, max))
	}

	for _, g := range rl.GPUS {
		min, max := int(g.Min), int(g.Max)
		args = append(args, GPUTotalArg.TypeRange(g.Type, min, max))
	}

	return args
}
