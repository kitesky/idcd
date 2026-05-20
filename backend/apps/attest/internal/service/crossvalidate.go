package service

import "context"

// crossValidate stubs step 2. It collapses observations to (1) the set
// of distinct node IDs that responded and (2) a consistency percentage.
// In the S2 MVP "consistency" is just the fraction of OK observations;
// the real implementation will check that the substantive results
// (status code, response body hash, etc.) agree across nodes.
func crossValidate(_ context.Context, obs []observation) (nodes []string, consistencyPct float64) {
	if len(obs) == 0 {
		return nil, 0
	}
	seen := map[string]struct{}{}
	okCount := 0
	for _, o := range obs {
		if _, ok := seen[o.NodeID]; !ok {
			seen[o.NodeID] = struct{}{}
			nodes = append(nodes, o.NodeID)
		}
		if o.OK {
			okCount++
		}
	}
	consistencyPct = float64(okCount) / float64(len(obs)) * 100
	return nodes, consistencyPct
}
