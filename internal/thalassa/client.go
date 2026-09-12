package thalassa

import (
	"fmt"
	"strings"

	"github.com/thalassa-cloud/client-go/pkg/client"
	"github.com/thalassa-cloud/client-go/thalassa"
	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/config"
)

const DefaultSubjectTokenFile = "/var/run/secrets/thalassa/token"

// NewClient builds a Thalassa API client using OIDC token exchange (WIF).
func NewClient(cfg config.Config) (thalassa.Client, error) {
	baseURL := config.NormalizeURL(cfg.ThalassaURL)
	tokenURL := baseURL + "/oidc/token"
	subjectFile := strings.TrimSpace(cfg.SubjectTokenFile)
	if subjectFile == "" {
		subjectFile = DefaultSubjectTokenFile
	}

	opts := []client.Option{
		client.WithBaseURL(baseURL),
		client.WithOrganisation(cfg.OrganisationID),
		client.WithUserAgent("thalassa-workload-identity-controller/0.1.0"),
		client.WithAuthOIDCTokenExchange(client.OIDCTokenExchangeConfig{
			TokenURL:         tokenURL,
			SubjectTokenFile: subjectFile,
			OrganisationID:   cfg.OrganisationID,
			ServiceAccountID: cfg.ControllerServiceAccount,
		}),
	}
	if project := strings.TrimSpace(cfg.ProjectIdentity); project != "" {
		opts = append(opts, client.WithProject(project))
	}

	tc, err := thalassa.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create thalassa client: %w", err)
	}
	return tc, nil
}
