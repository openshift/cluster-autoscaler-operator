package clusterautoscaler

import (
	"fmt"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	"k8s.io/klog/v2"
)

const (
	// The following values are taken from the OpenShift conventions
	// https://github.com/openshift/enhancements/blob/master/CONVENTIONS.md
	leaderElectLeaseDuration = "137s"
	leaderElectRenewDeadline = "107s"
	leaderElectRetryPeriod   = "26s"

	// The max bulk soft taint count is the maximum number of empty nodes that can be soft tainted for
	// deletion at once. A value of zero disables the bulk soft tainting behavior. This option is being
	// added to remediate a bad interaction between the bulk delete logic and the cluster-api provider,
	// for more information see https://issues.redhat.com/browse/OCPBUGS-42132
	maxBulkSoftTaintCount = "0"
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
	CordonNodeBeforeTerminatingArg   AutoscalerArg = "--cordon-node-before-terminating"
	NewPodScaleUpDelayArg            AutoscalerArg = "--new-pod-scale-up-delay"
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
	ScaleUpFromZeroDefaultArch       AutoscalerArg = "--scale-up-from-zero-default-arch"
	ExpanderArg                      AutoscalerArg = "--expander"
	MaxBulkSoftTaintCountArg         AutoscalerArg = "--max-bulk-soft-taint-count"
)

// Constants for the command line expander flags
const (
	leastWasteFlag = "least-waste"
	priorityFlag   = "priority"
	randomFlag     = "random"
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

	// AwsIgnoredLabelZoneID is a label used for the AWS-specific zone identifier, see https://github.com/kubernetes/cloud-provider-aws/issues/300 for a more detailed explanation of its use.
	AwsIgnoredLabelZoneID = "topology.k8s.aws/zone-id"
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

// GCP cloud provider ignore labels for the autoscaler.
const (
	// GceIgnoredLabelGkeZone label is used to specify the zone of the instance.
	GceIgnoredLabelGkeZone = "topology.gke.io/zone"
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

// Nutanix cloud provider ignore labels for the autoscaler.
const (
	// NutanixPrismElementName is a label used by the Nutanix Cloud Controller Manager to identify the Prism service.
	NutanixPrismElementName = "nutanix.com/prism-element-name"

	// NutanixPrismElementUuid is a label used by the Nutanix Cloud Controller Manager to uniquely identify the Prism service.
	NutanixPrismElementUuid = "nutanix.com/prism-element-uuid"

	// NutanixPrismHostName is a label used by the Nutanix Cloud Controller Manager to identify the host.
	NutanixPrismHostName = "nutanix.com/prism-host-name"

	// NutanixPrismHostUuid is a label used by the Nutanix Cloud Controller Manager to uniquely identify the host.
	NutanixPrismHostUuid = "nutanix.com/prism-host-uuid"
)

// AppendBasicIgnoreLabels appends ignore labels for specific cloud provider to the arguments
// so the autoscaler can use these labels without the user having to input them manually.
func appendBasicIgnoreLabels(args []string, cfg *Config) []string {
	switch cfg.platformType {
	case configv1.AWSPlatformType:
		args = append(args, BalancingIgnoreLabelArg.Value(AwsIgnoredLabelEbsCsiZone),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelLifecycle),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelK8sEniconfig),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelEksNodegroup),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelEksctlNodegroupName),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelEksctlInstanceId),
			BalancingIgnoreLabelArg.Value(AwsIgnoredLabelZoneID))
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
	case configv1.NutanixPlatformType:
		args = append(args, BalancingIgnoreLabelArg.Value(NutanixPrismElementName),
			BalancingIgnoreLabelArg.Value(NutanixPrismElementUuid),
			BalancingIgnoreLabelArg.Value(NutanixPrismHostName),
			BalancingIgnoreLabelArg.Value(NutanixPrismHostUuid))
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
		MaxBulkSoftTaintCountArg.Value(maxBulkSoftTaintCount),
	}

	if ca.Spec.MaxPodGracePeriod != nil {
		v := MaxGracefulTerminationSecArg.Value(*s.MaxPodGracePeriod)
		args = append(args, v)
	}

	if ca.Spec.MaxNodeProvisionTime != "" {
		v := MaxNodeProvisionTimeArg.Value(s.MaxNodeProvisionTime)
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

	if ca.Spec.ScaleUp != nil {
		args = append(args, ScaleUpArgs(s.ScaleUp)...)
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

	if len(ca.Spec.Expanders) > 0 {
		expanders := make([]string, 0)
		for _, v := range ca.Spec.Expanders {
			switch v {
			case v1.LeastWasteExpander:
				expanders = append(expanders, leastWasteFlag)
			case v1.PriorityExpander:
				expanders = append(expanders, priorityFlag)
			case v1.RandomExpander:
				expanders = append(expanders, randomFlag)
			default:
				// this shouldn't happen since we have validation on the API types, but just in case
				klog.Errorf("skipping unknown expander: %s", v)
				continue
			}
		}
		args = append(args, ExpanderArg.Value(strings.Join(expanders, ",")))
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

	if sd.CordonNodeBeforeTerminating != nil {
		switch *sd.CordonNodeBeforeTerminating {
		case v1.CordonNodeBeforeTerminatingModeEnabled:
			args = append(args, CordonNodeBeforeTerminatingArg.Value(true))
		case v1.CordonNodeBeforeTerminatingModeDisabled:
			args = append(args, CordonNodeBeforeTerminatingArg.Value(false))
		}
	}

	return args
}

func ScaleUpArgs(su *v1.ScaleUpConfig) []string {
	args := []string{}

	if su.NewPodScaleUpDelay != nil {
		args = append(args, NewPodScaleUpDelayArg.Value(*su.NewPodScaleUpDelay))
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
