package policy

import (
	"sync"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)



type PolicyIndex struct {
	byTenantApp    map[string]map[string][]*CompiledPolicy
	globalPolicies map[string][]*CompiledPolicy
	version        int64
	mu             sync.RWMutex
}

func NewIndex() *PolicyIndex {
	return &PolicyIndex{
		byTenantApp:    make(map[string]map[string][]*CompiledPolicy),
		globalPolicies: make(map[string][]*CompiledPolicy),
	}
}

func (idx *PolicyIndex) Add(cp *CompiledPolicy) {
	tenantID := cp.Proto.TenantId
	isGlobal := len(cp.AppIDs) == 0 || hasWildcard(cp.AppIDs)

	if isGlobal {
		idx.globalPolicies[tenantID] = append(idx.globalPolicies[tenantID], cp)
		return
	}

	if idx.byTenantApp[tenantID] == nil {
		idx.byTenantApp[tenantID] = make(map[string][]*CompiledPolicy)
	}
	for appID := range cp.AppIDs {
		idx.byTenantApp[tenantID][appID] = append(idx.byTenantApp[tenantID][appID], cp)
	}
}

func BuildFromBundle(bundle *proto.PolicyBundle) (*PolicyIndex, error) {
	idx := NewIndex()
	idx.version = bundle.Version
	for _, rule := range bundle.Rules {
		cp, err := Compile(rule)
		if err != nil {
			continue
		}
		idx.Add(cp)
	}
	return idx, nil
}

func hasWildcard(m map[string]struct{}) bool {
	_, ok := m["*"]
	return ok
}

func (idx *PolicyIndex) Version() int64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.version
}