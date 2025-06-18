package pool

import (
    "fmt"
    "math"
    "sync"
)

// ResultRecorder is an optional interface that allows recording
// proxy usage results (success/failure). Implementations may use
// these statistics to improve proxy selection quality.
// This design keeps backward-compatibility – existing ProxyStore users
// do not need to be changed; only stores interested in the metrics
// implement it.
//
// RecordResult should be called after each proxy usage attempt.
// success == true  -> the attempt succeeded.
// success == false -> the attempt failed and the proxy will usually be removed/marked invalid.
//
// The function should be safe for concurrent use.
type ResultRecorder interface {
    RecordResult(proxy string, success bool)
}

// UCBProxyStore is a decorator over any ProxyStore that selects
// proxies using an Upper Confidence Bound (UCB1) algorithm with a
// sliding window to prioritise high-quality proxies while still
// allowing exploration.
//
// The wrapped store is responsible for maintaining the actual set of
// proxies (Add/Remove/Len etc.). UCBProxyStore only influences the
// GetNext selection order.
//
// The algorithm maintains, for each proxy, the results of the last
// `window` uses (success/failure). From these it computes:
//   averageSuccess = successes / n
//   score          = averageSuccess + sqrt( 2 * ln(totalUses) / n )
// A proxy with no history gets a very high score to encourage early
// exploration.
//
// The implementation is intentionally simple and fully in-memory to
// minimise intrusiveness; it can be refined later if needed.

type ucbStats struct {
    results      []bool // circular buffer of recent results
    successCount int    // number of successes inside the window
}

type UCBProxyStore struct {
    base   ProxyStore
    window int

    mu         sync.Mutex
    stats      map[string]*ucbStats
    totalCalls int // across all proxies, for UCB formula
}

// NewUCBProxyStore wraps an existing ProxyStore with UCB selection.
// window defines the sliding-window size used when computing success
// statistics.
func NewUCBProxyStore(base ProxyStore, window int) *UCBProxyStore {
    return &UCBProxyStore{
        base:   base,
        window: window,
        stats:  make(map[string]*ucbStats),
    }
}

// ---- ProxyStore interface passthroughs ----

func (u *UCBProxyStore) Add(proxy string) error {
    u.mu.Lock()
    if _, ok := u.stats[proxy]; !ok {
        u.stats[proxy] = &ucbStats{}
    }
    u.mu.Unlock()
    return u.base.Add(proxy)
}

func (u *UCBProxyStore) Remove(proxy string) error {
    u.mu.Lock()
    delete(u.stats, proxy)
    u.mu.Unlock()
    return u.base.Remove(proxy)
}

func (u *UCBProxyStore) MarkInvalid(proxy string) error {
    u.RecordResult(proxy, false)
    return u.base.MarkInvalid(proxy)
}

func (u *UCBProxyStore) GetAll() ([]string, error) { return u.base.GetAll() }

func (u *UCBProxyStore) Len() (int, error) { return u.base.Len() }

// GetNext chooses a proxy using UCB with sliding window.
func (u *UCBProxyStore) GetNext() (string, error) {
    proxies, err := u.base.GetAll()
    if err != nil {
        return "", err
    }
    if len(proxies) == 0 {
        return "", ErrEmptyPool
    }

    u.mu.Lock()
    defer u.mu.Unlock()

    var bestProxy string
    bestScore := -1.0
    // Small constant to avoid division by zero
    epsilon := 1e-6

    for _, p := range proxies {
        st, ok := u.stats[p]
        if !ok {
            // unseen proxy -> ensure entry and give it infinite score to encourage exploration
            u.stats[p] = &ucbStats{}
            return p, nil
        }

        n := len(st.results)
        if n == 0 {
            return p, nil // also encourage exploration
        }

        mean := float64(st.successCount) / float64(n)
        bonus := math.Sqrt(2 * math.Log(float64(u.totalCalls+1)) / float64(n))
        score := mean + bonus

        if score > bestScore+epsilon {
            bestScore = score
            bestProxy = p
        }
    }

    if bestProxy == "" {
        // Fallback to first proxy in list
        bestProxy = proxies[0]
    }
    return bestProxy, nil
}

// RecordResult updates statistics after a proxy attempt.
func (u *UCBProxyStore) RecordResult(proxy string, success bool) {
    u.mu.Lock()
    defer u.mu.Unlock()

    st, ok := u.stats[proxy]
    if !ok {
        st = &ucbStats{}
        u.stats[proxy] = st
    }

    st.results = append(st.results, success)
    if success {
        st.successCount++
    }
    if len(st.results) > u.window {
        // pop oldest
        oldest := st.results[0]
        st.results = st.results[1:]
        if oldest {
            st.successCount--
        }
    }

    u.totalCalls++
}

// ErrEmptyPool is returned when there are no proxies available.
var ErrEmptyPool = fmt.Errorf("代理池为空") 