package model

import (
	"fmt"
	"runtime/debug"
	"time"
)

type CustomEvent struct {
	Source            string                 `json:"source"`
	SourceType        string                 `json:"sourceType"`
	Target            string                 `json:"target"`
	TargetType        string                 `json:"targetType"`
	Relationship      string                 `json:"relationship"`
	Inner             string                 `json:"error"`
	Severity          string                 `json:"severity"`
	Message           string                 `json:"message"`
	StackTrace        string                 `json:"stackTrace"`
	SourceAttrs       map[string]interface{} `json:"sourceAttrs"`
	TargetAttrs       map[string]interface{} `json:"targetAttrs"`
	RelationshipAttrs map[string]interface{} `json:"relationshipAttrs"`
}

func GenErrEvent(source, sourceType string, severity string, err error, attrs map[string]interface{}, messagef string, args ...interface{}) CustomEvent {
	return CustomEvent{
		Source:            source,
		SourceType:        sourceType,
		Inner:             err.Error(),
		Severity:          severity,
		Message:           fmt.Sprintf(messagef, args...),
		StackTrace:        string(debug.Stack()),
		RelationshipAttrs: attrs,
	}
}

//******* Independent models ********

type Worker struct {
	ID           string `json:"id" db:"ID"`
	Name         string `json:"name" db:"NAME"`
	Title        string `json:"title" db:"TITLE"`
	Organization string `json:"organization" db:"ORGANIZATION"`
	CreatedBy    string `json:"createdBy" db:"CREATED_BY"`
	CreatedAt    string `json:"createdAt" db:"CREATED_AT"`
	UpdatedBy    string `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt    string `json:"updatedAt" db:"UPDATED_AT"`
}

type TierCadence string

const (
	ThreeMonthsTierCadence TierCadence = "three-months"
	SixMonthsTierCadence   TierCadence = "six-months"
	YearlyTierCadence      TierCadence = "yearly"
)

type Tier struct {
	ID          string      `json:"id" db:"ID"`
	Name        string      `json:"name" db:"NAME"`
	Title       string      `json:"title" db:"TITLE"`
	Description string      `json:"description" db:"DESCRIPTION"`
	FromPoint   int         `json:"fromPoint" db:"FROM_POINT"`
	ToPoint     int         `json:"toPoint" db:"TO_POINT"`
	Cadence     TierCadence `json:"cadence" db:"CADENCE"`
	CreatedAt   string      `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string      `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string      `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string      `json:"updatedAt" db:"UPDATED_AT"`
}

type Baseline struct {
	ID          string `json:"id" db:"ID"`
	TierID      string `json:"tierId" db:"TIER_ID"`
	Name        string `json:"name" db:"NAME"`
	Title       string `json:"title" db:"TITLE"`
	Description string `json:"description" db:"DESCRIPTION"`
	CreatedAt   string `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string `json:"updatedAt" db:"UPDATED_AT"`
}

type QuestionType string

const (
	LaunchQuestionType QuestionType = "launch"
	ShieldQuestionType QuestionType = "shield"
	ThreatQuestionType QuestionType = "threat"
)

type QuestionWeightType string

const (
	ContextWeightType     QuestionWeightType = "context"
	FlagWeightType        QuestionWeightType = "flag"
	CoefficientWeightType QuestionWeightType = "coefficient"
)

type Question struct {
	ID                   string             `json:"id" db:"ID"`
	Title                string             `json:"title" db:"TITLE"`
	Text                 string             `json:"text" db:"TEXT"`
	AllowMultipleAnswers bool               `json:"allowMultipleAnswers" db:"ALLOW_MULTIPLE_ANSWERS"`
	EvidenceRequired     bool               `json:"evidenceRequired" db:"EVIDENCE_REQUIRED"`
	DisplayOrder         int                `json:"displayOrder" db:"DISPLAY_ORDER"`
	Type                 QuestionType       `json:"type" db:"TYPE"`
	WeightType           QuestionWeightType `json:"weightType" db:"WEIGHT_TYPE"`
	WeightValue          float64            `json:"weightValue" db:"WEIGHT_VALUE"`
	CreatedAt            string             `json:"createdAt" db:"CREATED_AT"`
	CreatedBy            string             `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy            string             `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt            string             `json:"updatedAt" db:"UPDATED_AT"`
}

type AnswerType string

const (
	BoolAnswerType    AnswerType = "bool"
	IntegerAnswerType AnswerType = "integer"
	StringAnswerType  AnswerType = "string"
)

type Answer struct {
	ID           string     `json:"id" db:"ID"`
	QuestionID   string     `json:"questionId" db:"QUESTION_ID"`
	Text         string     `json:"text" db:"TEXT"`
	Type         AnswerType `json:"type" db:"TYPE"`
	WeightValue  float64    `json:"weightValue" db:"WEIGHT_VALUE"`
	DisplayOrder int        `json:"displayOrder" db:"DISPLAY_ORDER"`
	CreatedAt    string     `json:"createdAt" db:"CREATED_AT"`
	CreatedBy    string     `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy    string     `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt    string     `json:"updatedAt" db:"UPDATED_AT"`
}

//******* Launch related models ********

type LaunchState string

const (
	WaitingForAnswersLaunchState      LaunchState = "waiting-4-answers"
	AnswersSubmittedLaunchState       LaunchState = "answers-submitted"
	WaitingForTierApprovalLaunchState LaunchState = "waiting-4-tier-approval"
	TierValidatedLaunchState          LaunchState = "tier-validated"
	TimedOutLaunchState               LaunchState = "timedout"
	CancelledLaunchState              LaunchState = "cancelled" // replaced by a newer launch of the same asset
)

type Launch struct {
	ID          string      `json:"id" db:"ID"`
	AssetID     string      `json:"assetId" db:"ASSET_ID"`
	AssetmentID string      `json:"assetmentId" db:"ASSETMENT_ID"`
	TierID      *string     `json:"tierId,omitempty" db:"TIER_ID"`
	State       LaunchState `json:"state" db:"STATE"`
	Reminders   int         `json:"reminders" db:"REMINDERS"`
	Warnings    int         `json:"warnings" db:"WARNINGS"`
	Escalations int         `json:"escalations" db:"ESCALATIONS"`
	CreatedAt   string      `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string      `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string      `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string      `json:"updatedAt" db:"UPDATED_AT"`
}

type LaunchAnswer struct {
	ID          string  `json:"id" db:"ID"`
	QuestionID  string  `json:"questionId" db:"QUESTION_ID"`
	LaunchID    string  `json:"launchId" db:"LAUNCH_ID"`
	AnswerID    string  `json:"answerId" db:"ANSWER_ID"`
	AnswerText  *string `json:"answerText" db:"ANSWER_TEXT"`
	SubmittedBy string  `json:"submittedBy" db:"SUBMITTED_BY"`
	CreatedAt   string  `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string  `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string  `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string  `json:"updatedAt" db:"UPDATED_AT"`
}

type LaunchEvidence struct {
	ID          string  `json:"id" db:"ID"`
	QuestionID  string  `json:"questionId" db:"QUESTION_ID"`
	LaunchID    string  `json:"launchId" db:"LAUNCH_ID"`
	Desc        *string `json:"desc" db:"DESC"`
	URL         *string `json:"url" db:"URL"`
	SubmittedBy string  `json:"submittedBy" db:"SUBMITTED_BY"`
	CreatedAt   string  `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string  `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string  `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string  `json:"updatedAt" db:"UPDATED_AT"`
}

//******* Shield related models ********

type ShieldState string

const (
	WaitingForAnswersShieldState   ShieldState = "waiting-4-answers"
	AnswersSubmittedShieldState    ShieldState = "answers-submitted"
	WaitingForEvidencesShieldState ShieldState = "waiting-4-evidences"
	EvidencesSubmittedShieldState  ShieldState = "evidences-submitted"
	CompletedShieldState           ShieldState = "completed"
	TimedOutShieldState            ShieldState = "timedout"
	CancelledShieldState           ShieldState = "cancelled" // replaced by a newer shield of the same asset
)

type Shield struct {
	ID          string      `json:"id" db:"ID"`
	AssetID     string      `json:"assetId" db:"ASSET_ID"`
	AssetmentID string      `json:"assetmentId" db:"ASSETMENT_ID"`
	Score       int         `json:"score" db:"SCORE"`
	State       ShieldState `json:"state" db:"STATE"`
	Reminders   int         `json:"reminders" db:"REMINDERS"`
	Warnings    int         `json:"warnings" db:"WARNINGS"`
	Escalations int         `json:"escalations" db:"ESCALATIONS"`
	CreatedAt   string      `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string      `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string      `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string      `json:"updatedAt" db:"UPDATED_AT"`
}

type ShieldAnswer struct {
	ID          string  `json:"id" db:"ID"`
	QuestionID  string  `json:"questionId" db:"QUESTION_ID"`
	ShieldID    string  `json:"shieldId" db:"SHIELD_ID"`
	AnswerID    string  `json:"answerId" db:"ANSWER_ID"`
	AnswerText  *string `json:"answerText" db:"ANSWER_TEXT"`
	SubmittedBy string  `json:"submittedBy" db:"SUBMITTED_BY"`
	CreatedAt   string  `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string  `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string  `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string  `json:"updatedAt" db:"UPDATED_AT"`
}

type ShieldEvidence struct {
	ID          string  `json:"id" db:"ID"`
	QuestionID  string  `json:"questionId" db:"QUESTION_ID"`
	ShieldID    string  `json:"shieldId" db:"SHIELD_ID"`
	Desc        *string `json:"desc" db:"DESC"`
	URL         *string `json:"url" db:"URL"`
	SubmittedBy string  `json:"submittedBy" db:"SUBMITTED_BY"`
	CreatedAt   string  `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string  `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string  `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string  `json:"updatedAt" db:"UPDATED_AT"`
}

//******* Threat related models ********

type ThreatState string

const (
	OpenThreatState      ThreatState = "open"
	MitigatedThreatState ThreatState = "mitigated"
	AcceptedThreatState  ThreatState = "accepted"
	ClosedThreatState    ThreatState = "closed"
)

type Threat struct {
	ID          string      `json:"id" db:"ID"`
	AssetID     string      `json:"assetId" db:"ASSET_ID"`
	AssetmentID string      `json:"assetmentId" db:"ASSETMENT_ID"`
	Score       int         `json:"score" db:"SCORE"`
	State       ThreatState `json:"state" db:"STATE"`
	Reminders   int         `json:"reminders" db:"REMINDERS"`
	Warnings    int         `json:"warnings" db:"WARNINGS"`
	Escalations int         `json:"escalations" db:"ESCALATIONS"`
	CreatedAt   string      `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string      `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string      `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string      `json:"updatedAt" db:"UPDATED_AT"`
}

type ThreatAnswer struct {
	ID          string  `json:"id" db:"ID"`
	QuestionID  string  `json:"questionId" db:"QUESTION_ID"`
	ThreatID    string  `json:"threatId" db:"THREAT_ID"`
	AnswerID    string  `json:"answerId" db:"ANSWER_ID"`
	AnswerText  *string `json:"answerText" db:"ANSWER_TEXT"`
	SubmittedBy string  `json:"submittedBy" db:"SUBMITTED_BY"`
	CreatedAt   string  `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string  `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string  `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string  `json:"updatedAt" db:"UPDATED_AT"`
}

type ThreatEvidence struct {
	ID          string  `json:"id" db:"ID"`
	QuestionID  string  `json:"questionId" db:"QUESTION_ID"`
	ThreatID    string  `json:"threatId" db:"THREAT_ID"`
	Desc        *string `json:"desc" db:"DESC"`
	URL         *string `json:"url" db:"URL"`
	SubmittedBy string  `json:"submittedBy" db:"SUBMITTED_BY"`
	CreatedAt   string  `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string  `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string  `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string  `json:"updatedAt" db:"UPDATED_AT"`
}

//******* Action Item related models ********

type ActionItemState string

const (
	GapActionItemState ActionItemState = "gap"
	MetActionItemState ActionItemState = "met"
)

type ActionItem struct {
	ID           string          `json:"id" db:"ID"`
	BaselineID   string          `json:"baselineId" db:"BASELINE_ID"`
	AssessmentID string          `json:"assessmentId" db:"ASSESSMENT_ID"`
	State        ActionItemState `json:"state" db:"STATE"`
	CreatedAt    string          `json:"createdAt" db:"CREATED_AT"`
	CreatedBy    string          `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy    string          `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt    string          `json:"updatedAt" db:"UPDATED_AT"`
}

type ActionItemTransaction struct {
	ID           string `json:"id" db:"ID"`
	ActionItemID string `json:"actionItemId" db:"ACTION_ITEM_ID"`
	Text         string `json:"text" db:"TEXT"`
	CreatedAt    string `json:"createdAt" db:"CREATED_AT"`
	CreatedBy    string `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy    string `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt    string `json:"updatedAt" db:"UPDATED_AT"`
}

//******* Assessment related models ********

type AssessmentState string

const (
	PendingLaunchAssessmentState AssessmentState = "pending-launch"
)

type Assessment struct {
	ID          string          `json:"id" db:"ID"`
	AssetID     string          `json:"assetId" db:"ASSET_ID"`
	TierID      *string         `json:"tierId" db:"TIER_ID"`
	ThreatID    *string         `json:"threatId,omitempty" db:"THREAT_ID"`
	LaunchID    *string         `json:"launchId,omitempty" db:"LAUNCH_ID"`
	ShieldID    *string         `json:"shieldId,omitempty" db:"SHIELD_ID"`
	State       AssessmentState `json:"state" db:"STATE"`
	StartedAt   *time.Time      `json:"startedAt,omitempty" db:"STARTED_AT"`
	CompletedAt *time.Time      `json:"completedAt,omitempty" db:"COMPLETED_AT"`
	Result      *string         `json:"result,omitempty" db:"RESULT"`
	CreatedAt   string          `json:"createdAt" db:"CREATED_AT"`
	CreatedBy   string          `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy   string          `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt   string          `json:"updatedAt" db:"UPDATED_AT"`
}

//******* Asset related models ********

type AssetState string

const (
	PendingLaunchAssetState                 AssetState = "pending-launch"
	LaunchedAssetState                      AssetState = "launched"
	PendingTierValidationAssetState         AssetState = "pending-tier-validation"
	PendingShieldAssetState                 AssetState = "pending-shield"
	ShieldedAssetState                      AssetState = "shielded"
	PendingSecurityAssessmentAssetState     AssetState = "pending-security-assessment"
	PendingSecurityImplementationAssetState AssetState = "pending-security-implementation"
	ImplementedAssetState                   AssetState = "implemented"
)

type Asset struct {
	ID                string     `json:"id" db:"ASSET_ID"`
	Name              string     `json:"name" db:"ASSET_NAME"`
	Campus            string     `json:"campus" db:"CAMPUS"`
	Building          string     `json:"building" db:"BUILDING"`
	Section           string     `json:"section" db:"SECTION"`
	Level             string     `json:"level" db:"LEVEL"`
	Wing              string     `json:"wing" db:"WING"`
	Floor             string     `json:"floor" db:"FLOOR"`
	Space             string     `json:"space" db:"SPACE"`
	Priority          string     `json:"priority" db:"PRIORITY"`
	State             AssetState `json:"state" db:"STATE"`
	LastAssessmentID  *string    `json:"lastAssessmentId,omitempty" db:"LAST_ASSESSMENT_ID"`
	BusinessOwnerID   *string    `json:"businessOwner,omitempty" db:"BUSINESS_OWNER"`
	BusinessManagerID *string    `json:"businessManager,omitempty" db:"BUSINESS_MANAGER"`
	CreatedBy         string     `json:"createdBy" db:"CREATED_BY"`
	CreatedAt         string     `json:"createdAt" db:"CREATED_AT"`
	UpdatedBy         string     `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt         string     `json:"updatedAt" db:"UPDATED_AT"`
}
