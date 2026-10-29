package launch

import "github.com/khaledhikmat/psap/model"

type LaunchCreationRequest struct {
	AssetID string
}

type LaunchCreationResult struct {
	LaunchID string
}

type LaunchAnswersAnalysisRequest struct {
	AssetID  string
	LaunchID string
	Answers  []model.Answer
}

type LaunchAnswersAnalysisResult struct {
	TierID string
}
