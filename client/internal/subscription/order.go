package subscription

// ApplyOrder puts saved IDs first and appends new subscriptions in their
// existing creation order. Missing IDs are ignored without rewriting settings.
func ApplyOrder(items []Summary, ids []string) []Summary {
	byID := make(map[string]Summary, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	ordered := make([]Summary, 0, len(items))
	for _, id := range ids {
		if item, found := byID[id]; found {
			ordered = append(ordered, item)
			delete(byID, id)
		}
	}
	for _, item := range items {
		if _, found := byID[item.ID]; found {
			ordered = append(ordered, item)
		}
	}
	return ordered
}
