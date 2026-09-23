package podmutator

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/wif"
)

const (
	testAudience = "https://api.thalassa.cloud"
	testOrgID    = "org-1"
	testSAID     = "sa-1"
	testProject  = "proj-1"
	testAppName  = "app"
)

func TestOptedIn(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{name: "nil", want: false},
		{name: "no labels", pod: &corev1.Pod{}, want: false},
		{
			name: "wrong value",
			pod:  &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{wif.LabelWIFUse: "yes"}}},
			want: false,
		},
		{
			name: "opted in",
			pod:  &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{wif.LabelWIFUse: wif.LabelWIFUseValue}}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, OptedIn(tt.pod))
		})
	}
}

func TestIdentityFromServiceAccount(t *testing.T) {
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		wif.AnnotationOrganisationID:   testOrgID,
		wif.AnnotationServiceAccountID: testSAID,
		wif.AnnotationProjectID:        testProject,
	}}}
	id := IdentityFromServiceAccount(sa)
	require.True(t, id.Complete())
	require.Equal(t, testOrgID, id.OrganisationID)
	require.Equal(t, testSAID, id.ServiceAccountID)
	require.Equal(t, testProject, id.ProjectID)
	require.False(t, IdentityFromServiceAccount(&corev1.ServiceAccount{}).Complete())
}

func TestMutatePod(t *testing.T) {
	id := Identity{OrganisationID: testOrgID, ServiceAccountID: testSAID, ProjectID: testProject}

	t.Run("incomplete identity no-op", func(t *testing.T) {
		pod := basePod()
		require.False(t, MutatePod(pod, Identity{OrganisationID: testOrgID}, testAudience))
	})

	t.Run("empty audience no-op", func(t *testing.T) {
		pod := basePod()
		require.False(t, MutatePod(pod, id, ""))
	})

	t.Run("injects env volume and mount", func(t *testing.T) {
		pod := basePod()
		require.True(t, MutatePod(pod, id, testAudience))
		require.Len(t, pod.Spec.Volumes, 1)
		require.Equal(t, wif.ProjectedVolumeName, pod.Spec.Volumes[0].Name)
		require.NotNil(t, pod.Spec.Volumes[0].Projected)
		tok := pod.Spec.Volumes[0].Projected.Sources[0].ServiceAccountToken
		require.Equal(t, testAudience, tok.Audience)
		require.Equal(t, wif.ProjectedTokenPath, tok.Path)

		c := pod.Spec.Containers[0]
		require.Equal(t, testOrgID, envValue(c, wif.EnvOrganisationID))
		require.Equal(t, testSAID, envValue(c, wif.EnvServiceAccountID))
		require.Equal(t, testProject, envValue(c, wif.EnvProjectID))
		require.Equal(t, wif.DefaultSubjectTokenFile, envValue(c, wif.EnvSubjectTokenFile))
		require.Len(t, c.VolumeMounts, 1)
		require.Equal(t, wif.ProjectedVolumeName, c.VolumeMounts[0].Name)
	})

	t.Run("idempotent", func(t *testing.T) {
		pod := basePod()
		require.True(t, MutatePod(pod, id, testAudience))
		require.False(t, MutatePod(pod, id, testAudience))
		require.Len(t, pod.Spec.Volumes, 1)
		require.Len(t, pod.Spec.Containers[0].Env, 4)
	})

	t.Run("skips project env when empty", func(t *testing.T) {
		pod := basePod()
		require.True(t, MutatePod(pod, Identity{
			OrganisationID: testOrgID, ServiceAccountID: testSAID,
		}, testAudience))
		require.Empty(t, envValue(pod.Spec.Containers[0], wif.EnvProjectID))
		require.Len(t, pod.Spec.Containers[0].Env, 3)
	})

	t.Run("init containers injected", func(t *testing.T) {
		pod := basePod()
		pod.Spec.InitContainers = []corev1.Container{{Name: "init", Image: "busybox"}}
		require.True(t, MutatePod(pod, id, testAudience))
		require.NotEmpty(t, envValue(pod.Spec.InitContainers[0], wif.EnvOrganisationID))
		require.Len(t, pod.Spec.InitContainers[0].VolumeMounts, 1)
	})

	t.Run("preserves existing env", func(t *testing.T) {
		pod := basePod()
		pod.Spec.Containers[0].Env = []corev1.EnvVar{
			{Name: wif.EnvOrganisationID, Value: "already"},
		}
		require.True(t, MutatePod(pod, id, testAudience))
		require.Equal(t, "already", envValue(pod.Spec.Containers[0], wif.EnvOrganisationID))
		require.Equal(t, testSAID, envValue(pod.Spec.Containers[0], wif.EnvServiceAccountID))
	})

	t.Run("skips volume when equivalent token exists", func(t *testing.T) {
		exp := int64(3600)
		pod := basePod()
		pod.Spec.Volumes = []corev1.Volume{{
			Name: "custom-token",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Path:              wif.ProjectedTokenPath,
							Audience:          testAudience,
							ExpirationSeconds: &exp,
						},
					}},
				},
			},
		}}
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{
			Name: "custom-token", MountPath: wif.ProjectedTokenMountPath, ReadOnly: true,
		}}
		require.True(t, MutatePod(pod, id, testAudience))
		require.Len(t, pod.Spec.Volumes, 1)
		require.Equal(t, "custom-token", pod.Spec.Volumes[0].Name)
		require.Len(t, pod.Spec.Containers[0].VolumeMounts, 1)
	})
}

func basePod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAppName,
			Namespace: "ns",
			Labels:    map[string]string{wif.LabelWIFUse: wif.LabelWIFUseValue},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: testAppName,
			Containers: []corev1.Container{{
				Name:  testAppName,
				Image: "busybox",
			}},
		},
	}
}

func envValue(c corev1.Container, name string) string {
	for _, e := range c.Env {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}
