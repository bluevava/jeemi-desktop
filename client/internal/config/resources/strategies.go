package resources

import "jeemi/internal/config/compose"

func composeStrategyReplace() compose.Strategy { return compose.StrategyReplace }
func composeStrategyPrepend() compose.Strategy { return compose.StrategyPrepend }
func composeStrategyAppendBeforeTerminal() compose.Strategy {
	return compose.StrategyAppendBeforeTerminal
}
