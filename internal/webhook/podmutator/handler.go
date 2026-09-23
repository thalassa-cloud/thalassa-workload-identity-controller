package podmutator

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Handler mutates opted-in pods using ServiceAccount WIF annotations.
type Handler struct {
	Client   client.Client
	Decoder  admission.Decoder
	Audience string
}

// Handle implements admission.Handler.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("pod", req.Namespace+"/"+req.Name)

	pod := &corev1.Pod{}
	if err := h.Decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	// Namespace is often empty on the decoded object for CREATE.
	if pod.Namespace == "" {
		pod.Namespace = req.Namespace
	}

	if !OptedIn(pod) {
		return admission.Allowed("wif.use label not set")
	}

	saName := pod.Spec.ServiceAccountName
	if saName == "" {
		saName = "default"
	}
	var sa corev1.ServiceAccount
	if err := h.Client.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: saName}, &sa); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("serviceaccount not found; skipping mutation", "serviceAccount", saName)
			return admission.Allowed("serviceaccount not found")
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	id := IdentityFromServiceAccount(&sa)
	if !id.Complete() {
		logger.Info("serviceaccount missing WIF identity annotations; skipping mutation",
			"serviceAccount", saName)
		return admission.Allowed("wif identity annotations incomplete")
	}

	if !MutatePod(pod, id, h.Audience) {
		return admission.Allowed("already injected or no changes")
	}

	marshaled, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}
