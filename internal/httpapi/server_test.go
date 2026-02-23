package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
	"github.com/kyleharris/task-board/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestServer_CreateListClaimAndReadyCheck(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	svc := newServiceForHTTPTests(t, repoRoot)
	t.Cleanup(func() { _ = svc.Close() })

	server := NewServer(svc)
	h := server.Handler()

	createBody := `{"title":"HTTP Task","task_type":"design"}`
	createReq := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(createBody))
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var createResp map[string]string
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	taskID := createResp["task_id"]
	require.NotEmpty(t, taskID)

	listReq := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)

	claimBody := `{"actor":{"type":"agent","id":"a1","display_name":"Agent 1"},"auto_renew":true}`
	claimReq := httptest.NewRequest(http.MethodPost, "/tasks/"+taskID+"/claim", bytes.NewBufferString(claimBody))
	claimRec := httptest.NewRecorder()
	h.ServeHTTP(claimRec, claimReq)
	require.Equal(t, http.StatusOK, claimRec.Code)

	ctxBody := `{"actor":{"type":"agent","id":"a1","display_name":"Agent 1"},"type":"context","content":"ctx"}`
	ctxReq := httptest.NewRequest(http.MethodPost, "/tasks/"+taskID+"/artifacts", bytes.NewBufferString(ctxBody))
	ctxRec := httptest.NewRecorder()
	h.ServeHTTP(ctxRec, ctxReq)
	require.Equal(t, http.StatusOK, ctxRec.Code)

	designBody := `{"actor":{"type":"agent","id":"a1","display_name":"Agent 1"},"type":"design","content":"design"}`
	designReq := httptest.NewRequest(http.MethodPost, "/tasks/"+taskID+"/artifacts", bytes.NewBufferString(designBody))
	designRec := httptest.NewRecorder()
	h.ServeHTTP(designRec, designReq)
	require.Equal(t, http.StatusOK, designRec.Code)

	trans2Body := `{"actor":{"type":"agent","id":"a1","display_name":"Agent 1"},"to":"Design"}`
	trans2Req := httptest.NewRequest(http.MethodPost, "/tasks/"+taskID+"/transition", bytes.NewBufferString(trans2Body))
	trans2Rec := httptest.NewRecorder()
	h.ServeHTTP(trans2Rec, trans2Req)
	require.Equal(t, http.StatusOK, trans2Rec.Code)

	rubricArtifactBody := `{"actor":{"type":"agent","id":"a1","display_name":"Agent 1"},"type":"rubric_review","content":"rr"}`
	rubricArtifactReq := httptest.NewRequest(http.MethodPost, "/tasks/"+taskID+"/artifacts", bytes.NewBufferString(rubricArtifactBody))
	rubricArtifactRec := httptest.NewRecorder()
	h.ServeHTTP(rubricArtifactRec, rubricArtifactReq)
	require.Equal(t, http.StatusOK, rubricArtifactRec.Code)

	rubricBody := `{"actor":{"type":"agent","id":"a1","display_name":"Agent 1"},"rubric_version":"v1","required_fields_complete":true,"pass":true}`
	rubricReq := httptest.NewRequest(http.MethodPost, "/tasks/"+taskID+"/rubric", bytes.NewBufferString(rubricBody))
	rubricRec := httptest.NewRecorder()
	h.ServeHTTP(rubricRec, rubricReq)
	require.Equal(t, http.StatusOK, rubricRec.Code)

	readyBody := `{"actor":{"type":"agent","id":"a1","display_name":"Agent 1"}}`
	readyReq := httptest.NewRequest(http.MethodPost, "/tasks/"+taskID+"/ready-check", bytes.NewBufferString(readyBody))
	readyRec := httptest.NewRecorder()
	h.ServeHTTP(readyRec, readyReq)
	require.Equal(t, http.StatusOK, readyRec.Code)
}

func TestServer_ClaimConflict(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	svc := newServiceForHTTPTests(t, repoRoot)
	t.Cleanup(func() { _ = svc.Close() })

	taskID, err := svc.CreateTask(context.Background(), app.CreateTaskInput{Title: "Lease task", TaskType: "default"})
	require.NoError(t, err)
	_, err = svc.ClaimTask(context.Background(), app.ClaimTaskInput{TaskID: taskID, Actor: domain.Actor{Type: domain.ActorTypeAgent, ID: "a1", DisplayName: "Agent 1"}})
	require.NoError(t, err)

	server := NewServer(svc)
	h := server.Handler()

	claimBody := `{"actor":{"type":"human","id":"u1","display_name":"User 1"}}`
	claimReq := httptest.NewRequest(http.MethodPost, "/tasks/"+taskID+"/claim", bytes.NewBufferString(claimBody))
	claimRec := httptest.NewRecorder()
	h.ServeHTTP(claimRec, claimReq)
	require.Equal(t, http.StatusConflict, claimRec.Code)
}

func newServiceForHTTPTests(t *testing.T, repoRoot string) *app.Service {
	t.Helper()
	taskboardDir := filepath.Join(repoRoot, ".taskboard")
	require.NoError(t, os.MkdirAll(filepath.Join(taskboardDir, "tasks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(taskboardDir, "policy.yaml"), []byte(httpPolicyYAML), 0o644))

	db, err := storage.Open(filepath.Join(taskboardDir, "board.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background()))
	require.NoError(t, db.UpsertBoard(context.Background(), "default", repoRoot, time.Now().UTC()))
	require.NoError(t, db.Close())

	svc, err := app.OpenService(repoRoot)
	require.NoError(t, err)
	return svc
}

const httpPolicyYAML = `version: 1
lease_required_states:
  - "Scoping"
  - "Design"
  - "In Progress"
  - "PR"
transitions:
  - from: "Scoping"
    to: "Design"
    actor_types: ["human", "agent"]
  - from: "Design"
    to: "In Progress"
    actor_types: ["human", "agent"]
  - from: "In Progress"
    to: "PR"
    actor_types: ["human", "agent"]
  - from: "PR"
    to: "Complete"
    actor_types: ["human", "agent"]
required_artifacts_by_state:
  "Scoping": ["context"]
  "Design": ["context", "design"]
  "In Progress": ["context", "design", "rubric_review"]
  "PR": ["implementation_notes", "test_report", "docs_update"]
  "Complete": ["implementation_notes", "test_report", "docs_update"]
task_type_leases:
  default:
    default_ttl_minutes: 60
    allow_auto_renew: true
  design:
    default_ttl_minutes: 90
    allow_auto_renew: true
  implementation:
    default_ttl_minutes: 120
    allow_auto_renew: true
`
