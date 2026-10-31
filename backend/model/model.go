package model

import (
	"fmt"
	"runtime/debug"
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
	TierID            *string    `json:"tierId,omitempty" db:"TIER_ID"`
	LastLaunchID      *string    `json:"lastLaunchId,omitempty" db:"LAST_LAUNCH_ID"`
	LastShieldID      *string    `json:"lastShieldId,omitempty" db:"LAST_SHIELD_ID"`
	LastBaselineID    *string    `json:"lastBaselineId,omitempty" db:"LAST_BASELINE_ID"`
	BusinessOwnerID   *string    `json:"businessOwner,omitempty" db:"BUSINESS_OWNER"`
	BusinessManagerID *string    `json:"businessManager,omitempty" db:"BUSINESS_MANAGER"`
	CreatedBy         string     `json:"createdBy" db:"CREATED_BY"`
	CreatedAt         string     `json:"createdAt" db:"CREATED_AT"`
	UpdatedBy         string     `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt         string     `json:"updatedAt" db:"UPDATED_AT"`
}

type QuestionCategoryType string

const (
	LaunchQuestionCategory QuestionCategoryType = "launch"
	ShieldQuestionCategory QuestionCategoryType = "shield"
)

type QuestionCategory struct {
	ID        string               `json:"id" db:"ID"`
	Title     string               `json:"title" db:"TITLE"`
	Text      string               `json:"text" db:"TEXT"`
	Type      QuestionCategoryType `json:"type" db:"TYPE"`
	CreatedAt string               `json:"createdAt" db:"CREATED_AT"`
	CreatedBy string               `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy string               `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt string               `json:"updatedAt" db:"UPDATED_AT"`
}

type Question struct {
	ID         string `json:"id" db:"ID"`
	CategoryID string `json:"categoryId" db:"CATEGORY_ID"`
	Text       string `json:"text" db:"TEXT"`
	Weight     int    `json:"weight" db:"WEIGHT"`
	CreatedAt  string `json:"createdAt" db:"CREATED_AT"`
	CreatedBy  string `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy  string `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt  string `json:"updatedAt" db:"UPDATED_AT"`
}

type Answer struct {
	ID         string  `json:"id" db:"ID"`
	QuestionID string  `json:"questionId" db:"QUESTION_ID"`
	LaunchID   *string `json:"launchId,omitempty" db:"LAUNCH_ID"`
	ShieldID   *string `json:"shieldId,omitempty" db:"SHIELD_ID"`
	Reply      string  `json:"reply" db:"REPLY"` // "" not provided, "YES" or "NO"
	CreatedAt  string  `json:"createdAt" db:"CREATED_AT"`
	CreatedBy  string  `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy  string  `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt  string  `json:"updatedAt" db:"UPDATED_AT"`
}

// Answer replies
const (
	UnansweredReply = ""
	YesReply        = "YES"
	NoReply         = "NO"
)

type AnswerHistory struct {
	ID            string  `json:"id" db:"ID"`
	AnswerID      string  `json:"answerId" db:"ANSWER_ID"`
	QuestionID    string  `json:"questionId" db:"QUESTION_ID"`
	LaunchID      *string `json:"launchId,omitempty" db:"LAUNCH_ID"`
	ShieldID      *string `json:"shieldId,omitempty" db:"SHIELD_ID"`
	PreviousReply string  `json:"previousReply" db:"PREVIOUS_REPLY"`
	NewReply      string  `json:"newReply" db:"NEW_REPLY"`
	ChangedBy     string  `json:"changedBy" db:"CHANGED_BY"`
	ChangedAt     string  `json:"changedAt" db:"CHANGED_AT"`
}

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

type Evidence struct {
	ID                 string `json:"id" db:"ID"`
	QuestionCategoryID string `json:"questionCategoryId" db:"QUESTION_CATEGORY_ID"`
	ShieldID           string `json:"shieldId" db:"SHIELD_ID"`
	Description        string `json:"description" db:"DESCRIPTION"`
	URL                string `json:"url" db:"URL"`
	CreatedAt          string `json:"createdAt" db:"CREATED_AT"`
	CreatedBy          string `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy          string `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt          string `json:"updatedAt" db:"UPDATED_AT"`
}

type ShieldState string

const (
	WaitingForAnswersShieldState  ShieldState = "waiting-4-answers"
	AnswersSubmittedShieldState   ShieldState = "answers-submitted"
	WaitingForEvidencesShieldState ShieldState = "waiting-4-evidences"
	EvidencesSubmittedShieldState ShieldState = "evidences-submitted"
	CompletedShieldState          ShieldState = "completed"
	TimedOutShieldState           ShieldState = "timedout"
	CancelledShieldState          ShieldState = "cancelled" // replaced by a newer shield of the same asset
)

type Shield struct {
	ID          string      `json:"id" db:"ID"`
	AssetID     string      `json:"assetId" db:"ASSET_ID"`
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

type BaselineState string

const (
	PendingLaunchBaselineState BaselineState = "pending-launch"
)

type Baseline struct {
	ID        string        `json:"id" db:"ID"`
	AssetID   string        `json:"assetId" db:"ASSET_ID"`
	State     BaselineState `json:"state" db:"STATE"`
	CreatedAt string        `json:"createdAt" db:"CREATED_AT"`
	CreatedBy string        `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy string        `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt string        `json:"updatedAt" db:"UPDATED_AT"`
}

type ActionItemState string

const (
	PendingLaunchActionItemState ActionItemState = "pending-launch"
)

type ActionItem struct {
	ID         string          `json:"id" db:"ID"`
	BaselineID string          `json:"baselineId" db:"BASELINE_ID"`
	State      ActionItemState `json:"state" db:"STATE"`
	CreatedAt  string          `json:"createdAt" db:"CREATED_AT"`
	CreatedBy  string          `json:"createdBy" db:"CREATED_BY"`
	UpdatedBy  string          `json:"updatedBy" db:"UPDATED_BY"`
	UpdatedAt  string          `json:"updatedAt" db:"UPDATED_AT"`
}
