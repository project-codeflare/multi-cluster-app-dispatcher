package queuejob

import (
	"testing"

	"github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
)


func TestGetXQJFullName(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name      string
		qj        *arbv1.AppWrapper
		expected  string
	}{
		{
			name: "valid qj",
			qj: &arbv1.AppWrapper{
				ObjectMeta: metav1.ObjectMeta{
					Name: "qjName",
					Namespace: "qjNamespace",
				},
			},
			expected: "qjName_qjNamespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetXQJFullName(tt.qj)
			g.Expect(result).To(gomega.Equal(tt.expected))
		})
	}
}


func TestHigherSystemPriorityQJ(t *testing.T) { 
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name     string
		qj1      *arbv1.AppWrapper
		qj2      *arbv1.AppWrapper
		expected bool
	}{
		{
			name: "lower priority qj1",
			qj1: &arbv1.AppWrapper{
				Status: arbv1.AppWrapperStatus{
					SystemPriority: 1,
				},
			},
			qj2: &arbv1.AppWrapper{
				Status: arbv1.AppWrapperStatus{
					SystemPriority: 2,
				},
			},
			expected: false,
		},
		{
			name: "higher priority qj1",
			qj1: &arbv1.AppWrapper{
				Status: arbv1.AppWrapperStatus{
					SystemPriority: 2,
				},
			},
			qj2: &arbv1.AppWrapper{
				Status: arbv1.AppWrapperStatus{
					SystemPriority: 1,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HigherSystemPriorityQJ(tt.qj1, tt.qj2)
			g.Expect(result).To(gomega.Equal(tt.expected))
		})
	}
}

func TestGenerateAppWrapperCondition(t *testing.T) { 
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name          				string
		conditionType 				arbv1.AppWrapperConditionType
		status        				corev1.ConditionStatus
		reason        				string
		message       				string
		expected      				arbv1.AppWrapperCondition
	}{
		{
			name:          "generate condition",
			conditionType: arbv1.AppWrapperConditionType("Queuing"),
			status:        corev1.ConditionTrue,
			reason:        "reason",
			message:       "message",
			expected: arbv1.AppWrapperCondition{
				Type:    arbv1.AppWrapperConditionType("Queuing"),
				Status:  corev1.ConditionTrue,
				Reason:  "reason",
				Message: "message",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateAppWrapperCondition(tt.conditionType, tt.status, tt.reason, tt.message)
			
			g.Expect(result.Type).To(gomega.Equal(tt.expected.Type))
			g.Expect(result.Status).To(gomega.Equal(tt.expected.Status))
			g.Expect(result.Reason).To(gomega.Equal(tt.expected.Reason))
			g.Expect(result.Message).To(gomega.Equal(tt.expected.Message))
		})
	}
}


func TestIsLastConditionDuplicate(t *testing.T) { 
	g := gomega.NewGomegaWithT(t)

	aw := &arbv1.AppWrapper{
		Status: arbv1.AppWrapperStatus{
			Conditions: []arbv1.AppWrapperCondition{
				{
					Type: arbv1.AppWrapperConditionType("Queuing"),
					Status: corev1.ConditionTrue,
					Reason: "reason",
					Message: "test",
				},
			},
		},
	}

	tests := []struct {
		name          string
		conditionType arbv1.AppWrapperConditionType
		status        corev1.ConditionStatus
		reason        string
		message       string
		expected      bool
	}{
		{
			name:          "duplicate condition",
			conditionType: arbv1.AppWrapperConditionType("Queuing"),
			status:        corev1.ConditionTrue,
			reason:        "reason",
			message:       "test",
			expected:      true,
		},
		{
			name:          "different condition",
			conditionType: arbv1.AppWrapperConditionType("Running"),
			status:        corev1.ConditionFalse,
			reason:        "noReason",
			message:       "noMessage",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLastConditionDuplicate(aw, tt.conditionType, tt.status, tt.reason, tt.message)
			g.Expect(result).To(gomega.Equal(tt.expected))
		})
	}
}


func TestGetIndexOfMatchedCondition(t *testing.T) { 
	g := gomega.NewGomegaWithT(t)

	aw := &arbv1.AppWrapper{
		Status: arbv1.AppWrapperStatus{
			Conditions: []arbv1.AppWrapperCondition{
				{
					Type: arbv1.AppWrapperConditionType("Queuing"),
					Reason: "reason",
				},
				{
					Type: arbv1.AppWrapperConditionType("Running"),
					Reason: "reason1",
				},
			},
		},
	}

	tests := []struct {
		name          string
		conditionType arbv1.AppWrapperConditionType
		reason        string
		expected      int
	}{
		{
			name:          "match reason and condition",
			conditionType: arbv1.AppWrapperConditionType("Queuing"),
			reason:        "reason",
			expected:      0,
		},
		{
			name:          "match reason and condition",
			conditionType: arbv1.AppWrapperConditionType("Running"),
			reason:        "reason1",
			expected:      1,
		},
		{
			name:          "match reason but not condition",
			conditionType: arbv1.AppWrapperConditionType("Running"),
			reason:        "reason",
			expected:      -1,
		},
		{
			name:          "no match",
			conditionType: arbv1.AppWrapperConditionType("Queuing"),
			reason:        "reason1",
			expected:      -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIndexOfMatchedCondition(aw, tt.conditionType, tt.reason)
			g.Expect(result).To(gomega.Equal(tt.expected))
		})
	}
}
