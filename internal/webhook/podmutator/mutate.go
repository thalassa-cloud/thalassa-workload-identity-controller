package podmutator

import (
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/wif"
)

// Identity holds Thalassa IDs read from a ServiceAccount.
type Identity struct {
	OrganisationID   string
	ServiceAccountID string
	ProjectID        string
}

// Complete reports whether required exchange IDs are present.
func (id Identity) Complete() bool {
	return strings.TrimSpace(id.OrganisationID) != "" &&
		strings.TrimSpace(id.ServiceAccountID) != ""
}

// IdentityFromServiceAccount reads WIF annotations from a ServiceAccount.
func IdentityFromServiceAccount(sa *corev1.ServiceAccount) Identity {
	if sa == nil || sa.Annotations == nil {
		return Identity{}
	}
	return Identity{
		OrganisationID:   strings.TrimSpace(sa.Annotations[wif.AnnotationOrganisationID]),
		ServiceAccountID: strings.TrimSpace(sa.Annotations[wif.AnnotationServiceAccountID]),
		ProjectID:        strings.TrimSpace(sa.Annotations[wif.AnnotationProjectID]),
	}
}

// OptedIn returns true when the pod requests webhook injection.
func OptedIn(pod *corev1.Pod) bool {
	if pod == nil || pod.Labels == nil {
		return false
	}
	return pod.Labels[wif.LabelWIFUse] == wif.LabelWIFUseValue
}

// MutatePod injects env vars and a projected token volume when needed.
// Returns whether the pod was modified.
func MutatePod(pod *corev1.Pod, id Identity, audience string) bool {
	if pod == nil || !id.Complete() {
		return false
	}
	audience = strings.TrimSpace(audience)
	if audience == "" {
		return false
	}

	changed := ensureProjectedTokenVolume(pod, audience)
	for i := range pod.Spec.InitContainers {
		if injectContainer(&pod.Spec.InitContainers[i], id) {
			changed = true
		}
	}
	for i := range pod.Spec.Containers {
		if injectContainer(&pod.Spec.Containers[i], id) {
			changed = true
		}
	}
	return changed
}

func injectContainer(c *corev1.Container, id Identity) bool {
	changed := ensureEnv(c, wif.EnvOrganisationID, id.OrganisationID)
	if ensureEnv(c, wif.EnvServiceAccountID, id.ServiceAccountID) {
		changed = true
	}
	if id.ProjectID != "" {
		if ensureEnv(c, wif.EnvProjectID, id.ProjectID) {
			changed = true
		}
	}
	if ensureEnv(c, wif.EnvSubjectTokenFile, wif.DefaultSubjectTokenFile) {
		changed = true
	}
	if ensureVolumeMount(c) {
		changed = true
	}
	return changed
}

func ensureEnv(c *corev1.Container, name, value string) bool {
	for i := range c.Env {
		if c.Env[i].Name == name {
			return false
		}
	}
	c.Env = append(c.Env, corev1.EnvVar{Name: name, Value: value})
	return true
}

func ensureVolumeMount(c *corev1.Container) bool {
	for _, m := range c.VolumeMounts {
		if m.Name == wif.ProjectedVolumeName || m.MountPath == wif.ProjectedTokenMountPath {
			return false
		}
	}
	c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
		Name:      wif.ProjectedVolumeName,
		MountPath: wif.ProjectedTokenMountPath,
		ReadOnly:  true,
	})
	return true
}

func ensureProjectedTokenVolume(pod *corev1.Pod, audience string) bool {
	for _, v := range pod.Spec.Volumes {
		if v.Name == wif.ProjectedVolumeName {
			return false
		}
		if volumeHasThalassaToken(v, audience) {
			return false
		}
	}
	exp := int64(3600)
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: wif.ProjectedVolumeName,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: []corev1.VolumeProjection{{
					ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
						Path:              wif.ProjectedTokenPath,
						Audience:          audience,
						ExpirationSeconds: &exp,
					},
				}},
			},
		},
	})
	return true
}

func volumeHasThalassaToken(v corev1.Volume, audience string) bool {
	if v.Projected == nil {
		return false
	}
	for _, src := range v.Projected.Sources {
		if src.ServiceAccountToken == nil {
			continue
		}
		tok := src.ServiceAccountToken
		if tok.Path == wif.ProjectedTokenPath && tok.Audience == audience {
			return true
		}
	}
	return false
}
