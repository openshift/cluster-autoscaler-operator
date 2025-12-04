package clusterautoscaler

import (
	"errors"
	"fmt"

	"k8s.io/utils/ptr"

	autoscalingv1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// createOrUpdateAutoscalerServiceMonitor will create or update a serviceMonitor
// for the given ClusterAutoscaler custom resource instance.
func (r *Reconciler) createOrUpdateAutoscalerNetworkPolicies(ca *autoscalingv1.ClusterAutoscaler) (result []controllerutil.OperationResult, err error) {
	// If the policies objects don't exist yet on the API server, they will be used to create the objects. But if they do exist, they'll be
	// overwritten with the API server's version of the object by controllerutil.CreateOrUpdate() (which is called by createOrUpdateObjectForCA()).
	policies := r.AutoscalerNetworkPolicies(ca)
	// This version is going to stay untouched
	desired := r.AutoscalerNetworkPolicies(ca)
	for i, policy := range policies {
		r, e := r.createOrUpdateObjectForCA(ca, &policy, func() error {
			// This mutate function only gets called if the object already exists on the API server
			// Replace the spec of the object returned by the API server with the version we want.
			// If the Specs don't match, CreateOrUpdate() will do the update
			policy.Spec = desired[i].Spec
			return nil
		})
		result = append(result, r)
		err = errors.Join(err, e)
	}
	return
}

// makePort is a helper funciton to create a NetworkPolicyPort
func makePort(proto *corev1.Protocol,
	port intstr.IntOrString,
	//nolint:unparam
	endPort int32) networkingv1.NetworkPolicyPort {
	r := networkingv1.NetworkPolicyPort{
		Protocol: proto,
		Port:     nil,
	}
	if port != intstr.FromInt32(0) && port != intstr.FromString("") && port != intstr.FromString("0") {
		r.Port = &port
	}
	if endPort != 0 {
		r.EndPort = ptr.To(endPort)
	}
	return r
}

// AutoscalerNetworkPolicies returns the expected networkpolicies belonging
// to the given ClusterAutoscaler.
func (r *Reconciler) AutoscalerNetworkPolicies(ca *autoscalingv1.ClusterAutoscaler) []networkingv1.NetworkPolicy {
	protocolTCP := corev1.ProtocolTCP
	protocolUDP := corev1.ProtocolUDP
	var policies []networkingv1.NetworkPolicy
	namespacedName := r.AutoscalerName(ca)
	// Default deny all.  Additional policies will add all allowed traffic
	policies = append(policies, networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkingv1.SchemeGroupVersion.String(),
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-default-deny", namespacedName.Name),
			Namespace: namespacedName.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cluster-autoscaler": ca.Name,
					"k8s-app":            "cluster-autoscaler",
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
		},
	})
	// Cluster Autoscaler should be able to reach the cluster DNS
	policies = append(policies, networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkingv1.SchemeGroupVersion.String(),
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-allow-egress-to-dns", namespacedName.Name),
			Namespace: namespacedName.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cluster-autoscaler": ca.Name,
					"k8s-app":            "cluster-autoscaler",
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "openshift-dns",
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"dns.operator.openshift.io/daemonset-dns": "default",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						makePort(&protocolTCP, intstr.FromInt32(5353), 0),
						makePort(&protocolUDP, intstr.FromInt32(5353), 0),
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
		},
	})
	// Cluster Autoscaler should be able to reach the API server
	policies = append(policies, networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkingv1.SchemeGroupVersion.String(),
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-allow-egress-to-api-server", namespacedName.Name),
			Namespace: namespacedName.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cluster-autoscaler": ca.Name,
					"k8s-app":            "cluster-autoscaler",
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						makePort(&protocolTCP, intstr.FromInt32(6443), 0),
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
		},
	})
	// Cluster Autoscaler's webhooks port should be reachable
	policies = append(policies, networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkingv1.SchemeGroupVersion.String(),
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-allow-ingress-to-webhooks", namespacedName.Name),
			Namespace: namespacedName.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cluster-autoscaler": ca.Name,
					"k8s-app":            "cluster-autoscaler",
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						makePort(&protocolTCP, intstr.FromInt(r.config.WebhooksPort), 0),
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	})
	// Cluster Autoscaler's metrics port should be reachable
	policies = append(policies, networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkingv1.SchemeGroupVersion.String(),
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-allow-ingress-to-metrics", namespacedName.Name),
			Namespace: namespacedName.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cluster-autoscaler": ca.Name,
					"k8s-app":            "cluster-autoscaler",
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						makePort(&protocolTCP, intstr.FromInt32(int32(8085)), 0),
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	})
	return policies
}
