package retention

import (
	"encoding/json"
	"sort"
	"time"
)

type Planner struct {
	Now func() time.Time
}

func (p Planner) Plan(policies []Policy) ([]PlanItem, error) {
	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}

	sort.SliceStable(policies, func(i, j int) bool {
		if policies[i].Priority == policies[j].Priority {
			return policies[i].ID < policies[j].ID
		}
		return policies[i].Priority < policies[j].Priority
	})

	plan := make([]PlanItem, 0, len(policies))
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		spec, err := ValidatePolicy(policy)
		if err != nil {
			return nil, err
		}

		plan = append(plan, PlanItem{
			Policy:               policy,
			CutoffTime:           now.AddDate(0, 0, -policy.RetentionDays),
			DryRun:               policy.DryRun,
			UsePartitionDrop:     shouldUsePartitionDrop(policy, spec),
			RollupTargetInterval: rollupTargetInterval(policy),
		})
	}

	return plan, nil
}

func shouldUsePartitionDrop(policy Policy, spec TableSpec) bool {
	switch policy.PartitionStrategy {
	case "drop":
		return true
	case "delete":
		return false
	default:
		return spec.PartitionPreferred
	}
}

func rollupTargetInterval(policy Policy) string {
	var metadata struct {
		RollupTargetInterval string `json:"rollup_target_interval"`
	}
	_ = json.Unmarshal(policy.Metadata, &metadata)
	if metadata.RollupTargetInterval != "" {
		return metadata.RollupTargetInterval
	}

	return klineRollupTarget(policy.IntervalValue)
}
