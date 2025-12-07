/*
Author: Joyjeet Roy
*/
package balancer

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

type QLearning struct {
	pool       *ServerPool
	qTable     sync.Map
	counts     sync.Map
	mux        sync.RWMutex
	epsilon    float64
	alpha      float64
	gamma      float64
	maxQValue  float64
	lastQDelta float64
}

func NewQLearning(pool *ServerPool) *QLearning {
	return &QLearning{
		pool:    pool,
		epsilon: 0.05,
		alpha:   0.5,
		gamma:   0.95,
	}
}

func (ql *QLearning) getMaxFutureQ() float64 {
	maxQ := 0.0
	ql.qTable.Range(func(key, value interface{}) bool {
		if q, ok := value.(float64); ok {
			if q > maxQ {
				maxQ = q
			}
		}
		return true
	})
	return maxQ
}

func (ql *QLearning) NextBackend(r *http.Request) *Backend {
	ql.mux.Lock()
	defer ql.mux.Unlock()

	backends := ql.pool.Backends
	if len(backends) == 0 {
		return nil
	}

	if ql.epsilon > 0.01 && ql.maxQValue > 0 {
		decayFactor := 1.0 - (ql.lastQDelta / ql.maxQValue)
		if decayFactor > 0 && decayFactor < 1 {
			ql.epsilon *= decayFactor
		} else {
			ql.epsilon *= 0.95
		}
	}

	if rand.Float64() < ql.epsilon {
		aliveBackends := make([]*Backend, 0)
		for _, b := range backends {
			if b.IsAlive() {
				aliveBackends = append(aliveBackends, b)
			}
		}
		if len(aliveBackends) > 0 {
			return aliveBackends[rand.Intn(len(aliveBackends))]
		}
		return backends[rand.Intn(len(backends))]
	}

	var bestBackend *Backend
	var maxQ float64 = -1e9

	for _, b := range backends {
		if !b.IsAlive() {
			continue
		}

		qVal := 100.0
		if val, exists := ql.qTable.Load(b.URL.String()); exists {
			qVal = val.(float64)
		}

		if bestBackend == nil || qVal > maxQ {
			maxQ = qVal
			bestBackend = b
		}
	}

	if bestBackend == nil {
		for _, b := range backends {
			if b.IsAlive() {
				return b
			}
		}
		if len(backends) > 0 {
			return backends[0]
		}
		return nil
	}

	return bestBackend
}

func (ql *QLearning) OnRequestCompletion(u *url.URL, duration time.Duration, err error) {
	ql.mux.Lock()
	defer ql.mux.Unlock()

	urlStr := u.String()
	var reward float64

	if err != nil {
		reward = -1000.0
	} else {
		seconds := duration.Seconds()
		if seconds <= 0 {
			seconds = 0.0001
		}
		reward = (1.0 / (seconds * seconds)) + 50.0
	}

	oldQ := 0.0
	if val, exists := ql.qTable.Load(urlStr); exists {
		oldQ = val.(float64)
	}

	maxFutureQ := ql.getMaxFutureQ()
	newQ := (1-ql.alpha)*oldQ + ql.alpha*(reward+ql.gamma*maxFutureQ)

	ql.qTable.Store(urlStr, newQ)

	qDelta := newQ - oldQ
	if qDelta < 0 {
		qDelta = -qDelta
	}
	ql.lastQDelta = qDelta

	if newQ > ql.maxQValue {
		ql.maxQValue = newQ
	}

	count := int64(0)
	if val, exists := ql.counts.Load(urlStr); exists {
		count = val.(int64)
	}
	ql.counts.Store(urlStr, count+1)
}

func (ql *QLearning) AddBackend(b *Backend) {
	ql.pool.Backends = append(ql.pool.Backends, b)
}

func (ql *QLearning) UpdateBackendStatus(u *url.URL, alive bool) {
	for _, b := range ql.pool.Backends {
		if b.URL.String() == u.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func (ql *QLearning) Persist(path string) error {
	ql.mux.RLock()
	defer ql.mux.RUnlock()

	qTableMap := make(map[string]float64)
	ql.qTable.Range(func(key, value interface{}) bool {
		qTableMap[key.(string)] = value.(float64)
		return true
	})

	countsMap := make(map[string]int64)
	ql.counts.Range(func(key, value interface{}) bool {
		countsMap[key.(string)] = value.(int64)
		return true
	})

	data := make(map[string]interface{})
	data["qTable"] = qTableMap
	data["counts"] = countsMap
	data["epsilon"] = ql.epsilon
	data["gamma"] = ql.gamma
	data["maxQValue"] = ql.maxQValue
	data["lastQDelta"] = ql.lastQDelta

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (ql *QLearning) Load(path string) error {
	ql.mux.Lock()
	defer ql.mux.Unlock()

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return err
	}

	if qTable, ok := data["qTable"].(map[string]interface{}); ok {
		for k, v := range qTable {
			if val, ok := v.(float64); ok {
				ql.qTable.Store(k, val)
			}
		}
	}

	if counts, ok := data["counts"].(map[string]interface{}); ok {
		for k, v := range counts {
			if val, ok := v.(float64); ok {
				ql.counts.Store(k, int64(val))
			}
		}
	}

	if epsilon, ok := data["epsilon"].(float64); ok {
		ql.epsilon = epsilon
	}

	if gamma, ok := data["gamma"].(float64); ok {
		ql.gamma = gamma
	}

	if maxQValue, ok := data["maxQValue"].(float64); ok {
		ql.maxQValue = maxQValue
	}

	if lastQDelta, ok := data["lastQDelta"].(float64); ok {
		ql.lastQDelta = lastQDelta
	}

	return nil
}

func (ql *QLearning) GetBackends() []*Backend {
	return ql.pool.Backends
}
