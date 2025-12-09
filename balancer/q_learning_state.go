package balancer

func (ql *QLearning) ExportState(qTable *map[string]float64, counts *map[string]int64, epsilon, gamma, maxQValue, lastQDelta *float64) {
	ql.mux.RLock()
	defer ql.mux.RUnlock()

	ql.qTable.Range(func(key, value interface{}) bool {
		(*qTable)[key.(string)] = value.(float64)
		return true
	})

	ql.counts.Range(func(key, value interface{}) bool {
		(*counts)[key.(string)] = value.(int64)
		return true
	})

	*epsilon = ql.epsilon
	*gamma = ql.gamma
	*maxQValue = ql.maxQValue
	*lastQDelta = ql.lastQDelta
}

func (ql *QLearning) ImportState(qTable map[string]float64, counts map[string]int64, epsilon, gamma, maxQValue, lastQDelta float64) {
	ql.mux.Lock()
	defer ql.mux.Unlock()

	for k, v := range qTable {
		ql.qTable.Store(k, v)
		if v > ql.cachedMaxQ {
			ql.cachedMaxQ = v
		}
	}

	for k, v := range counts {
		ql.counts.Store(k, v)
	}

	ql.epsilon = epsilon
	ql.gamma = gamma
	ql.maxQValue = maxQValue
	ql.lastQDelta = lastQDelta
}
