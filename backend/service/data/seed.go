package data

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/lgr"
)

const seedActor = "seed"

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

// seed fills each seeded table, but only when that table is empty, so it never overwrites
// or duplicates data that has since been edited.
//
// A launch's points are the sum of the points of its selected answers (its weighted score in
// hundredths), and its tier is the one whose [FromPoint, ToPoint] range holds the points: the
// higher the score, the more critical the asset and the lower the tier number.
func seed(ctx context.Context, db *sqlx.DB) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	categories, questions, predefined := seedQuestions()
	seeded := []string{}
	for _, s := range []struct {
		table table
		rows  func() error
	}{
		{workersTable, func() error { return seedRows(ctx, tx, workersTable, seedWorkers()) }},
		{tiersTable, func() error { return seedRows(ctx, tx, tiersTable, seedTiers()) }},
		{questionCategoriesTable, func() error { return seedRows(ctx, tx, questionCategoriesTable, categories) }},
		{questionsTable, func() error { return seedRows(ctx, tx, questionsTable, questions) }},
		{predefinedAnswersTable, func() error { return seedRows(ctx, tx, predefinedAnswersTable, predefined) }},
		{assetsTable, func() error { return seedRows(ctx, tx, assetsTable, seedAssets()) }},
	} {
		var n int
		if err := tx.GetContext(ctx, &n, "SELECT COUNT(*) FROM "+s.table.name); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		if err := s.rows(); err != nil {
			return fmt.Errorf("seed %s: %w", s.table.name, err)
		}
		seeded = append(seeded, s.table.name)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if len(seeded) > 0 {
		lgr.Logger.Info("seeded database", slog.Any("tables", seeded))
	}
	return nil
}

// seedRows inserts rows with creation times one millisecond apart, so lists come back in
// seed order.
func seedRows[T any](ctx context.Context, tx *sqlx.Tx, t table, rows []T) error {
	base := time.Now().UTC()
	for i, row := range rows {
		ts := base.Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano)
		v := reflect.ValueOf(&row)
		for _, col := range []string{"CREATED_AT", "UPDATED_AT"} {
			if f, ok := t.field(v, col); ok {
				f.SetString(ts)
			}
		}
		for _, col := range []string{"CREATED_BY", "UPDATED_BY"} {
			if f, ok := t.field(v, col); ok {
				f.SetString(seedActor)
			}
		}
		if err := insertRow(ctx, tx, t, row); err != nil {
			return err
		}
	}
	return nil
}

func seedWorkers() []model.Worker {
	return []model.Worker{
		{ID: "worker-owner-1", Name: "Dana Whitfield", Title: "Facilities Director", Organization: "Facilities"},
		{ID: "worker-owner-2", Name: "Marcus Ito", Title: "Lab Operations Lead", Organization: "Research"},
		{ID: "worker-manager-1", Name: "Priya Raman", Title: "VP Operations", Organization: "Operations"},
		{ID: "worker-manager-2", Name: "Tom Okafor", Title: "Director of Infrastructure", Organization: "IT"},
		{ID: "worker-reviewer-1", Name: "Elena Novak", Title: "Security Analyst", Organization: "Corporate Security"},
	}
}

// seedTiers holds the working score ranges of prd.md in hundredths: a launch's points are its
// weighted score (1.00–5.00) times 100. Tier 4 also takes 0, which only unanswered questions
// can produce. Non-Tier can never be calculated; the assessor assigns it when validating.
func seedTiers() []model.Tier {
	return []model.Tier{
		{ID: "tier-1", Name: "Tier 1", Title: "Severe impact", FromPoint: 400, ToPoint: 500, Cadence: model.ThreeMonthsTierCadence,
			Description: "Any resource whose loss, damage or compromise results in a degree of impact that critically changes and causes loss or irreversible business capabilities."},
		{ID: "tier-2", Name: "Tier 2", Title: "Significant impact", FromPoint: 300, ToPoint: 399, Cadence: model.SixMonthsTierCadence,
			Description: "Any resource whose loss, damage or compromise results in a degree of impact that damages and degrades business capabilities."},
		{ID: "tier-3", Name: "Tier 3", Title: "Moderate impact", FromPoint: 200, ToPoint: 299, Cadence: model.YearlyTierCadence,
			Description: "Any resource whose loss, damage or compromise results in a degree of impact that threatens to damage and degrade business capabilities."},
		{ID: "tier-4", Name: "Tier 4", Title: "Minor impact", FromPoint: 0, ToPoint: 199, Cadence: model.YearlyTierCadence,
			Description: "Any resource whose loss, damage or compromise results in a degree of impact that may disrupt business capabilities."},
		{ID: "non-tier", Name: "Non-Tier", Title: "No tier", FromPoint: -1, ToPoint: -1,
			Description: "Not a secure room or area. Assigned by the assessor when validating a launch; never calculated."},
	}
}

func seedAssets() []model.Asset {
	return []model.Asset{
		{ID: "asset-hq-dc", Name: "HQ Data Center", Campus: "Main Campus", Building: "B1", Section: "East",
			Level: "Basement", Wing: "East", Floor: "B1", Space: "DC-01", Priority: "high",
			State: model.PendingLaunchAssetState, BusinessOwnerID: stringPtr("worker-owner-1"), BusinessManagerID: stringPtr("worker-manager-1")},
		{ID: "asset-rd-lab", Name: "R&D Lab 3", Campus: "North Campus", Building: "N2", Section: "West",
			Level: "Ground", Wing: "West", Floor: "1", Space: "LAB-03", Priority: "medium",
			State: model.PendingLaunchAssetState, BusinessOwnerID: stringPtr("worker-owner-2"), BusinessManagerID: stringPtr("worker-manager-2")},
	}
}

type seedCategory struct {
	key, title, text string
	questions        []seedQuestion
}

type seedQuestion struct {
	text     string
	multiple bool // more than one answer may be selected
	answers  []seedAnswer
}

type seedAnswer struct {
	text   string
	points int
}

func seedQuestions() ([]model.QuestionCategory, []model.Question, []model.PredefinedAnswer) {
	categories := []model.QuestionCategory{}
	questions := []model.Question{}
	answers := []model.PredefinedAnswer{}
	add := func(t model.QuestionCategoryType, cats []seedCategory) {
		for _, c := range cats {
			catID := fmt.Sprintf("%s-%s", t, c.key)
			categories = append(categories, model.QuestionCategory{ID: catID, Title: c.title, Text: c.text, Type: t})
			for i, q := range c.questions {
				qID := fmt.Sprintf("%s-%d", catID, i+1)
				questions = append(questions, model.Question{
					ID: qID, CategoryID: catID, Text: q.text, AllowMultipleAnswers: q.multiple,
					// Shield requires evidence for every answer; launch is validated by the assessor instead.
					EvidenceRequired: t == model.ShieldQuestionCategory,
					DisplayOrder:     i + 1,
				})
				for j, a := range q.answers {
					answers = append(answers, model.PredefinedAnswer{
						ID: fmt.Sprintf("%s-%d", qID, j+1), QuestionID: qID, Text: a.text, Points: a.points, DisplayOrder: j + 1,
					})
				}
			}
		}
	}
	add(model.LaunchQuestionCategory, launchCategories)
	add(model.ShieldQuestionCategory, shieldCategories)
	return categories, questions, answers
}

// scored is a single-choice question whose answers score 1, 2, 3… in the order given (higher
// means more critical or more exposed). Each answer's points are its score times weight, the
// question's share of the total score; a weight of 0 makes the question informational only.
func scored(text string, weight int, answers ...string) seedQuestion {
	q := seedQuestion{text: text}
	for i, a := range answers {
		q.answers = append(q.answers, seedAnswer{a, (i + 1) * weight})
	}
	return q
}

// Launch questions classify an asset by business impact (see prd.md). Every question scores
// 1–5 in the order its answers appear. The launch score is
// Q3×0.20 + Q5×0.25 + Q6×0.20 + Q7×0.25 + Q8×0.10; Q1, Q2 and Q4 carry no weight. Weights are
// percentages, so a launch's points are its score in hundredths and tiers are ranges of those.
var launchCategories = []seedCategory{
	{"room-function", "Room Function", "Functions that the room supports.", []seedQuestion{
		{text: "What functions does this room or area support? Select all that apply.", multiple: true, answers: []seedAnswer{
			{"Safekeeping of valuables such as currency or IT equipment", 0},
			{"High-value asset storage", 0},
			{"Data transmission / network infrastructure", 0},
			{"Data storage", 0},
			{"Sensitive business operations", 0},
			{"Building infrastructure such as water, electrical or HVAC", 0},
			{"Executive leadership area", 0},
			{"Physical security operations", 0},
			{"Life-safety systems", 0},
			{"Other", 0},
		}},
	}},
	{"regulatory-requirement", "Regulatory Requirement", "Legal, regulatory or business requirements for physical security.", []seedQuestion{
		scored("Are there any legal, regulatory, or business requirements that require physical security measures for this room or area?", 0,
			"Yes: Requirement identified by business/SME",
			"No: No known requirement",
			"Unknown: Needs Security/Compliance validation"),
	}},
	{"asset-type", "Asset Type", "What the room or area houses.", []seedQuestion{
		scored("What type of assets are housed in this room or area? Answer for the highest-criticality asset, system or information actually housed in the room.", 20,
			"General: General office equipment or supplies",
			"Department-specific: Department-specific equipment, records or resources",
			"Sensitive: Shared business systems or sensitive information",
			"Regulatory: Association systems, security systems or regulated information",
			"Mission-critical infrastructure: life-safety systems or highly sensitive assets"),
	}},
	{"routine-access", "Routine Access", "Who legitimately needs access.", []seedQuestion{
		scored("How limited is the legitimate business population requiring access to this asset?", 0,
			"Broad business population: no access restrictions",
			"Defined business population: all badged employees and contractors",
			"Limited business population: badged employees only",
			"Restricted access population: access limited to a small number of specifically approved individuals",
			"Highly restricted access only: normally limited to very few individuals and may require approval for a specific purpose each time"),
	}},
	{"business-dependency", "Business Dependency", "How much daily operations depend on the room.", []seedQuestion{
		scored("How dependent are daily business operations on this room or area?", 25,
			"Minimal: convenience function only",
			"Limited: supports a single team or department",
			"Moderate: supports multiple departments or a business function",
			"Significant: supports critical business operations or major site functions",
			"Severe: supports Association-wide operations or could cause major business interruption"),
	}},
	{"recovery-capability", "Recovery Capability", "How quickly operations could be restored.", []seedQuestion{
		scored("If this room or area became unavailable, how quickly could operations be restored using existing recovery plans, redundancies or alternate locations?", 20,
			"Within hours",
			"Within 1 day",
			"Within 1 week",
			"Within 1 month",
			"More than 1 month or no practical recovery option"),
	}},
	{"unauthorized-access-impact", "Unauthorized Access Impact", "Impact of access by an unauthorized person.", []seedQuestion{
		scored("What could be the impact if assets, systems, information or equipment in this room were accessed by an unauthorized person?", 25,
			"No meaningful impact",
			"Limited impact to a small workshop",
			"Moderate impact to a department or business function",
			"Significant impact to multiple departments, a site or customers",
			"Severe enterprise, regulatory, financial or reputational impact"),
	}},
	{"backup-alternate-resources", "Backup / Alternate Resources", "Availability of backups if a primary system fails.", []seedQuestion{
		// "Not applicable" adds no criticality, so it scores like the lowest answer.
		{text: "Are backup systems or alternate resources available if a primary system or component in this room fails?", answers: []seedAnswer{
			{"Full redundancy: continuity with little or no interruption", 10},
			{"High redundancy: backup available with minimal downtime", 20},
			{"Moderate redundancy: backups exist, but single points of failure remain", 30},
			{"Limited redundancy: some backup capability exists but recovery requires significant effort", 40},
			{"No redundancy: critical functions depend on a single point of failure", 50},
			{"Not applicable: no function requiring redundancy", 10},
		}},
	}},
}

// Shield questions score observable exposure (see prd.md). Each is single-choice, scored 1
// (lowest exposure) to 5 (highest) in the order its answers appear, and requires evidence.
var shieldCategories = []seedCategory{
	{"physical-proximity", "Physical Proximity", "How far the room is from uncontrolled areas.", []seedQuestion{
		scored("Counting from the nearest uncontrolled or public point, how many access-controlled zone boundaries must be crossed to reach the room or area?", 1,
			"Must cross 3 or more access-controlled zone boundaries",
			"Must cross 2 access-controlled zone boundaries",
			"Must cross 1 access-controlled zone boundary",
			"Can reach the immediate area without crossing a zone boundary, but not directly from a public area",
			"Can directly approach the asset from a public or unrestricted area"),
	}},
	{"entry", "Entry", "How hard it is to get in through the room's own boundary.", []seedQuestion{
		scored("How difficult would it be for an unauthorized individual to gain entry through the room or area’s own boundary?", 1,
			"Entry requires defeating 3 or more independent barriers at the room/area boundary",
			"Entry requires defeating 2 independent barriers at the room/area boundary",
			"Entry requires defeating 1 hardened or rated barrier (per job-aid barrier table)",
			"Entry requires bypassing 1 standard, unrated barrier (standard door, gate or similar)",
			"No physical barrier prevents entry"),
	}},
	{"time", "Time", "How long it takes to reach the room.", []seedQuestion{
		scored("How long does it take to walk from the nearest uncontrolled approach point to the room or area’s outermost door or opening? (Record a timed walk)", 1,
			"More than 10 minutes",
			"5 to 10 minutes",
			"2 to 5 minutes",
			"30 seconds to 2 minutes",
			"Less than 30 seconds"),
	}},
	{"skill", "Skill", "What it takes to defeat the room's boundary.", []seedQuestion{
		scored("What level of skill, tools, inside knowledge, or specialized equipment would be needed to bypass or defeat the room or area’s physical boundary?", 1,
			"Requires specialized technical knowledge and equipment not commonly available",
			"Requires specialized equipment or documented technical knowledge",
			"Requires commercially available tools and basic knowledge of the barrier or control",
			"Ordinary hand tools or simple manipulation could defeat the boundary",
			"No tools or specialized knowledge needed; boundary is easily bypassed"),
	}},
	{"visual-exposure", "Visual Exposure", "How visible the asset is.", []seedQuestion{
		scored("How easily can the asset be seen and observed in its environment?", 1,
			"Not visible from any uncontrolled area",
			"Visible only from a restricted or controlled adjacent area",
			"Partially visible from an uncontrolled area",
			"Fully visible from an uncontrolled (non-public) area",
			"Fully visible from a public or unrestricted area"),
	}},
	{"recog", "Recog", "How easily the asset's purpose is recognized.", []seedQuestion{
		scored("How easily can the asset’s purpose, value or importance be understood by someone unfamiliar with it?", 1,
			"Purpose cannot reasonably be determined from visible characteristics",
			"Purpose can only be determined with specialized or internal knowledge",
			"General function can be inferred, but specific purpose/value is not apparent",
			"Purpose is apparent from signage, equipment, layout or routine activity",
			"Purpose and security significance are explicitly identified or obvious"),
	}},
	{"information-availability", "Information Availability", "How much public information exists about the asset.", []seedQuestion{
		scored("How readily available is information about the asset’s characteristics, location or function to the general public?", 1,
			"No public information identifies the asset or its location",
			"General facility information is public, but the asset is not identified",
			"Asset type/function is publicly identifiable, but location/details are not",
			"Asset location or operational details are publicly available",
			"Detailed location, function, configuration or access information is publicly available"),
	}},
	{"human-operational-exposure", "Human / Operational Exposure", "Who not assigned to the room enters its area.", []seedQuestion{
		scored("Which people not permanently assigned to the room or area enter or work in the immediate area, and how often?", 1,
			"No non-assigned persons, or exception-only (escorted) access",
			"Non-assigned persons enter less than monthly",
			"Non-assigned employees or contractors enter monthly to weekly",
			"Contractors or service personnel enter daily or routinely",
			"Visitors, customers/members or the public enter daily or routinely"),
	}},
	{"exterior-environmental-exposure", "Exterior / Environmental Exposure", "Exposure to exterior and environmental conditions.", []seedQuestion{
		scored("Does the room or area have direct exposure to exterior, uncontrolled or environmental conditions that affect required protection?", 1,
			"Fully interior; no uncontrolled adjacency or exterior exposure",
			"Adjacent to lower-controlled interior space only",
			"Exterior wall, service penetration or nearby utility/environmental exposure",
			"Accessible exterior opening, service route, dock or equipment opening",
			"Direct public exterior door, vehicle-accessible boundary or high-risk environmental exposure"),
	}},
}
