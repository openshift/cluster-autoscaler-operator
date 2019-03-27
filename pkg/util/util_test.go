package util

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFilterString(t *testing.T) {
	filterStringTests := []struct {
		label    string
		needle   string
		haystack []string
		output   []string
		count    int
	}{
		{
			label:    "single instance",
			needle:   "foo",
			haystack: []string{"foo", "bar", "baz"},
			output:   []string{"bar", "baz"},
			count:    1,
		},
		{
			label:    "multiple instances",
			needle:   "foo",
			haystack: []string{"foo", "bar", "foo"},
			output:   []string{"bar"},
			count:    2,
		},
		{
			label:    "zero instances",
			needle:   "buzz",
			haystack: []string{"foo", "bar", "foo"},
			output:   []string{"foo", "bar", "foo"},
			count:    0,
		},
	}

	for _, tt := range filterStringTests {
		tt := tt // capture range variable
		t.Run(tt.label, func(t *testing.T) {
			t.Parallel()
			got, count := FilterString(tt.haystack, tt.needle)

			if !reflect.DeepEqual(got, tt.output) {
				t.Errorf("got %q, want %q", got, tt.output)
			}

			if count != tt.count {
				t.Errorf("got count %d, want count %d", count, tt.count)
			}
		})
	}
}

func TestReleaseVersionMatches(t *testing.T) {
	releaseVersion := "v100"

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
		},
	}

	testCases := []struct {
		label        string
		expectedBool bool
		annotations  map[string]string
	}{
		{
			label:        "no annotation",
			expectedBool: false,
			annotations:  nil,
		},
		{
			label:        "wrong version",
			expectedBool: false,
			annotations: map[string]string{
				ReleaseVersionAnnotation: "BAD",
			},
		},
		{
			label:        "correct version",
			expectedBool: true,
			annotations: map[string]string{
				ReleaseVersionAnnotation: releaseVersion,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			deployment.SetAnnotations(tc.annotations)

			ok := ReleaseVersionMatches(deployment, releaseVersion)
			if ok != tc.expectedBool {
				t.Errorf("got %t, want %t", ok, tc.expectedBool)
			}
		})
	}
}

func TestDeploymentUpdated(t *testing.T) {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  "test-namespace",
			Generation: 100,
		},
	}

	testCases := []struct {
		label        string
		expectedBool bool
		status       appsv1.DeploymentStatus
	}{
		{
			label:        "old generation",
			expectedBool: false,
			status: appsv1.DeploymentStatus{
				AvailableReplicas:  10,
				Replicas:           10,
				UpdatedReplicas:    10,
				ObservedGeneration: 10,
			},
		},
		{
			label:        "replicas not updated",
			expectedBool: false,
			status: appsv1.DeploymentStatus{
				Replicas:           10,
				UpdatedReplicas:    5,
				ObservedGeneration: 100,
			},
		},
		{
			label:        "no available replicas",
			expectedBool: false,
			status: appsv1.DeploymentStatus{
				AvailableReplicas:  0,
				Replicas:           10,
				UpdatedReplicas:    10,
				ObservedGeneration: 100,
			},
		},
		{
			label:        "available and updated",
			expectedBool: true,
			status: appsv1.DeploymentStatus{
				AvailableReplicas:  10,
				Replicas:           10,
				UpdatedReplicas:    10,
				ObservedGeneration: 100,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			deployment.Status = tc.status

			ok := DeploymentUpdated(deployment)
			if ok != tc.expectedBool {
				t.Errorf("got %t, want %t", ok, tc.expectedBool)
			}
		})
	}
}
