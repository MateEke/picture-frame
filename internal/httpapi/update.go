package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/MateEke/picture-frame/internal/licenses"
	"github.com/MateEke/picture-frame/internal/updater"
)

// UpdaterStatus is the read-and-act surface over the self-updater, implemented by
// *updater.Updater (and the dev mock). A nil updater reads as "no update available".
type UpdaterStatus interface {
	Status() updater.Status
	Trigger()
	Check()
}

type UpdateStatusResponse struct {
	Current       string `json:"current" doc:"Running version"`
	Platform      string `json:"platform" doc:"Build target, e.g. linux_armv6"`
	Latest        string `json:"latest,omitempty" doc:"Newest available version"`
	NotesURL      string `json:"notes_url,omitempty" doc:"Release notes page for the newest version"`
	Available     bool   `json:"available" doc:"Whether a newer version is available"`
	LastCheck     string `json:"last_check,omitempty" doc:"RFC3339 time of the last check"`
	LastCheckOK   bool   `json:"last_check_ok" doc:"False when the last check couldn't reach the release source"`
	Phase         string `json:"phase" enum:"idle,checking,downloading,verifying,applying" doc:"Current updater activity"`
	LastResult    string `json:"last_result,omitempty" doc:"Outcome of the last apply, e.g. 'rolled back from v1.2.0'"`
	LastResultSeq int    `json:"last_result_seq,omitempty" doc:"Bumps each time a result is recorded; lets the UI detect a repeated outcome"`
}

type getUpdateOutput struct {
	Body UpdateStatusResponse
}

type getLicensesOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

func (s *server) registerUpdateRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-update",
		Method:      http.MethodGet,
		Path:        "/api/system/update",
		Summary:     "Get self-update status",
	}, func(_ context.Context, _ *struct{}) (*getUpdateOutput, error) {
		if s.updater == nil {
			return &getUpdateOutput{Body: UpdateStatusResponse{Phase: updater.PhaseIdle.String()}}, nil
		}
		st := s.updater.Status()
		body := UpdateStatusResponse{
			Current:       st.Current,
			Platform:      st.Platform,
			Latest:        st.Latest,
			NotesURL:      st.NotesURL,
			Available:     st.Available,
			LastCheckOK:   st.LastCheckOK,
			Phase:         st.Phase.String(),
			LastResult:    st.LastResult,
			LastResultSeq: st.LastResultSeq,
		}
		if !st.LastCheck.IsZero() {
			body.LastCheck = st.LastCheck.UTC().Format(time.RFC3339)
		}
		return &getUpdateOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "apply-update",
		Method:        http.MethodPost,
		Path:          "/api/system/update",
		Summary:       "Download, verify, and apply the latest release",
		DefaultStatus: http.StatusAccepted,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		if s.updater == nil {
			return nil, huma.Error409Conflict("the updater is not configured")
		}
		s.updater.Trigger()
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "check-update",
		Method:        http.MethodPost,
		Path:          "/api/system/update/check",
		Summary:       "Check the release source for a newer version now",
		DefaultStatus: http.StatusAccepted,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		if s.updater == nil {
			return nil, huma.Error409Conflict("the updater is not configured")
		}
		s.updater.Check()
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-licenses",
		Method:      http.MethodGet,
		Path:        "/api/system/licenses",
		Summary:     "Third-party dependency notices (plain text)",
	}, func(_ context.Context, _ *struct{}) (*getLicensesOutput, error) {
		return &getLicensesOutput{
			ContentType: "text/plain; charset=utf-8",
			Body:        []byte(licenses.Notices),
		}, nil
	})
}
