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

// seed fills each seeded table, but only when that table is empty, so it never overwrites
// or duplicates data that has since been edited.
//
// Launch question weights add up to 100. A launch's points are the sum of the weights of its
// YES answers, and its tier is the one whose [FromPoint, ToPoint] range holds the points:
// the higher the score, the more critical the asset and the lower the tier number.
func seed(ctx context.Context, db *sqlx.DB) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	categories, questions := seedQuestions()
	seeded := []string{}
	for _, s := range []struct {
		table table
		rows  func() error
	}{
		{workersTable, func() error { return seedRows(ctx, tx, workersTable, seedWorkers()) }},
		{tiersTable, func() error { return seedRows(ctx, tx, tiersTable, seedTiers()) }},
		{questionCategoriesTable, func() error { return seedRows(ctx, tx, questionCategoriesTable, categories) }},
		{questionsTable, func() error { return seedRows(ctx, tx, questionsTable, questions) }},
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

func seedTiers() []model.Tier {
	return []model.Tier{
		{ID: "tier-1", Name: "Tier 1", Title: "Critical", FromPoint: 75, ToPoint: 100, Cadence: model.ThreeMonthsTierCadence,
			Description: "Mission-critical or highly sensitive assets. Strongest protection, reviewed every three months."},
		{ID: "tier-2", Name: "Tier 2", Title: "High", FromPoint: 50, ToPoint: 74, Cadence: model.SixMonthsTierCadence,
			Description: "Important assets whose compromise would seriously disrupt the business. Reviewed every six months."},
		{ID: "tier-3", Name: "Tier 3", Title: "Moderate", FromPoint: 25, ToPoint: 49, Cadence: model.YearlyTierCadence,
			Description: "Assets with moderate impact. Standard protection, reviewed yearly."},
		{ID: "tier-4", Name: "Tier 4", Title: "Low", FromPoint: 0, ToPoint: 24, Cadence: model.YearlyTierCadence,
			Description: "Low-impact assets. Baseline protection, reviewed yearly."},
	}
}

func seedAssets() []model.Asset {
	return []model.Asset{
		{ID: "asset-hq-dc", Name: "HQ Data Center", Campus: "Main Campus", Building: "B1", Section: "East",
			Level: "Basement", Wing: "East", Floor: "B1", Space: "DC-01", Priority: "high",
			State: model.PendingLaunchAssetState, BusinessOwnerID: "worker-owner-1", BusinessManagerID: "worker-manager-1"},
		{ID: "asset-rd-lab", Name: "R&D Lab 3", Campus: "North Campus", Building: "N2", Section: "West",
			Level: "Ground", Wing: "West", Floor: "1", Space: "LAB-03", Priority: "medium",
			State: model.PendingLaunchAssetState, BusinessOwnerID: "worker-owner-2", BusinessManagerID: "worker-manager-2"},
	}
}

type seedCategory struct {
	key, title, text string
	questions        []seedQuestion
}

type seedQuestion struct {
	text   string
	weight int
}

func seedQuestions() ([]model.QuestionCategory, []model.Question) {
	categories := []model.QuestionCategory{}
	questions := []model.Question{}
	add := func(t model.QuestionCategoryType, cats []seedCategory) {
		for _, c := range cats {
			catID := fmt.Sprintf("%s-%s", t, c.key)
			categories = append(categories, model.QuestionCategory{ID: catID, Title: c.title, Text: c.text, Type: t})
			for i, q := range c.questions {
				questions = append(questions, model.Question{
					ID: fmt.Sprintf("%s-%d", catID, i+1), CategoryID: catID, Text: q.text, Weight: q.weight,
				})
			}
		}
	}
	add(model.LaunchQuestionCategory, launchCategories)
	add(model.ShieldQuestionCategory, shieldCategories)
	return categories, questions
}

// Launch questions classify an asset. Their weights add up to 100.
var launchCategories = []seedCategory{
	{"occupancy", "Occupancy & Personnel", "Who works in or visits the space.", []seedQuestion{
		{"Do more than 50 people regularly work in the space?", 3},
		{"Is the space staffed outside business hours?", 2},
		{"Do executives or other high-profile people regularly work in the space?", 4},
		{"Do contractors or vendors work in the space unescorted?", 2},
		{"Is cash or are payments handled in the space?", 2},
	}},
	{"information", "Sensitive Information", "Information stored or processed in the space.", []seedQuestion{
		{"Is confidential or proprietary information stored in the space?", 3},
		{"Is personal data (PII) of customers or employees processed in the space?", 4},
		{"Are classified or export-controlled materials handled in the space?", 5},
		{"Are paper records containing sensitive data kept in the space?", 2},
	}},
	{"operations", "Critical Operations", "How much the business depends on the space.", []seedQuestion{
		{"Does the space host servers, network or telecom equipment?", 4},
		{"Would an outage of the space stop a critical business process?", 5},
		{"Does the space support 24x7 operations?", 3},
		{"Is the space a single point of failure with no alternate site?", 4},
		{"Does the space control building systems (power, HVAC, fire suppression)?", 3},
	}},
	{"hazards", "Hazardous Materials", "Dangerous materials present in the space.", []seedQuestion{
		{"Are hazardous chemicals stored or used in the space?", 3},
		{"Are flammable materials or fuels stored in the space?", 3},
		{"Are pressurized gases or gas cylinders present?", 2},
		{"Is radioactive or biological material present?", 4},
	}},
	{"valuables", "Valuable Assets", "Valuable property kept in the space.", []seedQuestion{
		{"Does the space contain equipment worth more than $1M in total?", 3},
		{"Are high-value portable items (laptops, devices) stored in the space?", 2},
		{"Is inventory or finished product stored in the space?", 2},
		{"Are cash, artwork or precious materials kept in the space?", 2},
		{"Are prototypes or other R&D intellectual property kept in the space?", 3},
	}},
	{"access", "Public Access", "How exposed the space is to outsiders.", []seedQuestion{
		{"Is the space open to the public?", 3},
		{"Is the space on the ground floor or street-facing?", 2},
		{"Does the space share an entrance with other tenants?", 2},
		{"Are deliveries received directly into the space?", 1},
	}},
	{"compliance", "Regulatory & Compliance", "Regulations that apply to the space.", []seedQuestion{
		{"Is the space in scope for PCI DSS?", 2},
		{"Is the space in scope for HIPAA?", 2},
		{"Do government security-clearance requirements apply to the space?", 3},
		{"Is the space's physical security audited by an external party?", 2},
		{"Does the space require environmental or safety permits?", 1},
	}},
	{"continuity", "Business Continuity", "Recovery needs and history of the space.", []seedQuestion{
		{"Must the space recover from a disruption in under 24 hours?", 3},
		{"Does the space host backup or disaster-recovery systems?", 3},
		{"Has the space had a security incident in the last three years?", 4},
		{"Is the space in an area prone to natural disasters?", 2},
	}},
}

// Shield questions assess the protection already in place.
var shieldCategories = []seedCategory{
	{"perimeter", "Perimeter Security", "Barriers around the building and site.", []seedQuestion{
		{"Is the site enclosed by a fence or wall?", 3},
		{"Are vehicle barriers or bollards installed at entrances?", 2},
		{"Are perimeter gates locked outside business hours?", 2},
		{"Is perimeter signage (no trespassing, monitored) posted?", 1},
	}},
	{"access-control", "Access Control", "How entry to the space is controlled.", []seedQuestion{
		{"Is entry controlled by badge readers?", 4},
		{"Is multi-factor entry (badge + PIN or biometric) required?", 4},
		{"Are access rights reviewed at least quarterly?", 3},
		{"Is anti-tailgating protection (turnstiles, mantraps) in place?", 3},
		{"Are physical keys inventoried and controlled?", 2},
	}},
	{"surveillance", "Video Surveillance", "Camera coverage and recording.", []seedQuestion{
		{"Do cameras cover all entrances and exits?", 4},
		{"Is footage retained for at least 30 days?", 2},
		{"Is video monitored live?", 3},
		{"Are cameras checked for faults at least monthly?", 1},
	}},
	{"intrusion", "Intrusion Detection", "Alarms that detect forced or unauthorized entry.", []seedQuestion{
		{"Are doors fitted with door-forced and door-held alarms?", 3},
		{"Are motion sensors installed?", 2},
		{"Do alarms report to a monitored central station?", 3},
		{"Is the alarm system tested at least twice a year?", 2},
		{"Do alarms have backup power?", 2},
	}},
	{"lighting", "Lighting", "Lighting around and inside the space.", []seedQuestion{
		{"Are entrances and parking areas lit at night?", 2},
		{"Is emergency lighting installed?", 2},
		{"Is exterior lighting checked regularly?", 1},
		{"Does lighting support camera image quality at night?", 2},
	}},
	{"personnel", "Security Personnel", "Guards and security staff.", []seedQuestion{
		{"Is a security officer on site during business hours?", 3},
		{"Is a security officer on site after hours?", 3},
		{"Do guards follow documented patrol routes?", 2},
		{"Are guards trained in incident response?", 2},
		{"Is there a staffed security operations center?", 3},
	}},
	{"emergency", "Emergency Response", "Preparedness for emergencies.", []seedQuestion{
		{"Is there a documented emergency response plan?", 3},
		{"Are evacuation drills held at least yearly?", 2},
		{"Are emergency contacts posted and up to date?", 1},
		{"Is there a mass-notification system for occupants?", 2},
	}},
	{"visitors", "Visitor Management", "How visitors are handled.", []seedQuestion{
		{"Must visitors sign in and show ID?", 3},
		{"Are visitors issued temporary badges?", 2},
		{"Are visitors escorted at all times?", 3},
		{"Are visitor logs kept for at least a year?", 1},
		{"Are deliveries screened before entering the space?", 2},
	}},
}
