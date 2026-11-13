package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/data"
)

type apiFileConfig struct{ file string }

func (c apiFileConfig) SQLiteFile() string { return c.file }

func TestAssessmentEndpointsUseDataServiceWithoutFlowService(t *testing.T) {
	ctx := context.Background()
	datasvc := data.NewSQLLite(ctx, apiFileConfig{filepath.Join(t.TempDir(), "api.db")})
	if datasvc == nil {
		t.Fatal("unable to open database")
	}
	defer datasvc.Finalize()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	controller := &Controller{DataSvc: datasvc}
	controller.Routes(router.Group(ContextRoot))
	post := func(path string, value any) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, ContextRoot+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}

	if got := post("/launches", map[string]string{"assetId": "asset-hq-dc"}).Code; got != http.StatusAccepted {
		t.Fatalf("initiate launch status = %d", got)
	}
	launches, err := datasvc.RetrieveLaunchesByAssetID(ctx, "asset-hq-dc")
	if err != nil || len(launches) == 0 {
		t.Fatalf("launches = %v, err = %v", launches, err)
	}
	launch := launches[len(launches)-1]
	launchQuestions, err := datasvc.RetrieveQuestionsByCategoryType(ctx, model.LaunchQuestionCategory)
	if err != nil {
		t.Fatal(err)
	}
	launchAnswers, err := firstAnswers(datasvc, ctx, model.LaunchQuestionCategory, launchQuestions)
	if err != nil {
		t.Fatal(err)
	}
	if got := post("/launches/"+launch.ID+"/finalize-answers", map[string]any{"answers": launchAnswers}).Code; got != http.StatusOK {
		t.Fatalf("finalize launch status = %d", got)
	}
	launch, err = datasvc.RetrieveLaunchByID(ctx, launch.ID)
	if err != nil || launch.State != model.WaitingForTierApprovalLaunchState {
		t.Fatalf("launch after finalize = %+v, err = %v", launch, err)
	}
	if got := post("/launches/"+launch.ID+"/validate-tier", map[string]string{}).Code; got != http.StatusOK {
		t.Fatalf("validate tier status = %d", got)
	}

	shields, err := datasvc.RetrieveShieldsByAssetID(ctx, "asset-hq-dc")
	if err != nil || len(shields) == 0 {
		t.Fatalf("shields = %v, err = %v", shields, err)
	}
	shield := shields[len(shields)-1]
	shieldQuestions, err := datasvc.RetrieveQuestionsByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		t.Fatal(err)
	}
	shieldAnswers, err := firstAnswers(datasvc, ctx, model.ShieldQuestionCategory, shieldQuestions)
	if err != nil {
		t.Fatal(err)
	}
	if got := post("/shields/"+shield.ID+"/finalize-answers", map[string]any{"answers": shieldAnswers}).Code; got != http.StatusOK {
		t.Fatalf("finalize shield answers status = %d", got)
	}
	evidences := []model.EvidenceSubmission{}
	for _, question := range shieldQuestions {
		if question.EvidenceRequired {
			evidences = append(evidences, model.EvidenceSubmission{
				QuestionID: question.ID, Description: "Recorded for test", URL: "https://example.test/evidence",
			})
		}
	}
	if got := post("/shields/"+shield.ID+"/finalize-evidences", map[string]any{"evidences": evidences}).Code; got != http.StatusOK {
		t.Fatalf("finalize shield evidences status = %d", got)
	}
	shield, err = datasvc.RetrieveShieldByID(ctx, shield.ID)
	if err != nil || shield.State != model.CompletedShieldState {
		t.Fatalf("shield after finalize = %+v, err = %v", shield, err)
	}
	asset, err := datasvc.RetrieveAssetByID(ctx, "asset-hq-dc")
	if err != nil || asset.State != model.ShieldedAssetState {
		t.Fatalf("asset after Shield completion = %+v, err = %v", asset, err)
	}
}

func firstAnswers(datasvc data.IService, ctx context.Context, category model.QuestionCategoryType, questions []model.Question) ([]model.AnswerSubmission, error) {
	predefined, err := datasvc.RetrievePredefinedAnswersByCategoryType(ctx, category)
	if err != nil {
		return nil, err
	}
	byQuestion := make(map[string]string, len(predefined))
	for _, answer := range predefined {
		if _, exists := byQuestion[answer.QuestionID]; !exists {
			byQuestion[answer.QuestionID] = answer.ID
		}
	}
	submissions := make([]model.AnswerSubmission, 0, len(questions))
	for _, question := range questions {
		answerID, exists := byQuestion[question.ID]
		if !exists {
			continue
		}
		submissions = append(submissions, model.AnswerSubmission{
			QuestionID: question.ID, PredefinedAnswerIDs: []string{answerID},
		})
	}
	return submissions, nil
}
