package resources

// GroupEnabled describes this configuration's reference, never the shared
// resource definition. Disabled references remain ordered and exportable.
func (p Plan) GroupEnabled(id string) bool {
	if p.RulesDisabled || p.RuleStrategy == "none" {
		return false
	}
	for _, disabled := range p.DisabledStrategyGroupIDs {
		if id == disabled {
			return false
		}
	}
	return true
}
