package model

import (
	"strings"
	"sync"
	"time"
)

// ParseModelList splits a comma-separated model list into clean trimmed names.
func ParseModelList(val string) []string {
	var list []string
	for _, s := range strings.Split(val, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			list = append(list, s)
		}
	}
	return list
}

// ModelMapping stores an alias-to-upstream mapping. Users request using the
// alias; Mochi resolves it to the upstream model name before forwarding.
type ModelMapping struct {
	Id           int    `gorm:"primaryKey" json:"id"`
	Alias        string `gorm:"uniqueIndex;size:128;not null" json:"alias"`
	UpstreamName string `gorm:"size:128;not null" json:"upstream_name"`
	CreatedAt    int64  `json:"created_at"`
}

// ---------------------------------------------------------------------------
// In-memory cache: alias -> upstream name (or comma-separated upstream names)
// ---------------------------------------------------------------------------

var (
	mappingMu   sync.RWMutex
	aliasToUp   = make(map[string]string) // alias -> upstream list string
	upstreamSet = make(map[string]bool)   // upstream names that have ≥1 alias
	aliasList   []string                  // all alias names
)

// ResolveAlias returns the upstream model name for an alias. O(1) lookup.
func ResolveAlias(name string) (upstream string, isAlias bool) {
	mappingMu.RLock()
	defer mappingMu.RUnlock()
	up, ok := aliasToUp[name]
	return up, ok
}

// ResolveModelTargets returns the upstream models eligible for a requested
// name. Explicit mapping targets come first, while the literal requested name
// remains a fallback because an alias may also be a real model on an upstream.
func ResolveModelTargets(name string) []string {
	upstream, isAlias := ResolveAlias(name)
	if !isAlias {
		return []string{name}
	}

	targets := ParseModelList(upstream)
	for _, target := range targets {
		if target == name {
			return targets
		}
	}
	return append(targets, name)
}

// GetAllAliases returns all alias names from the cache.
func GetAllAliases() []string {
	mappingMu.RLock()
	defer mappingMu.RUnlock()
	out := make([]string, len(aliasList))
	copy(out, aliasList)
	return out
}

// GetUpstreamNamesWithAliases returns the set of upstream names that have at
// least one alias defined. Used to filter /v1/models.
func GetUpstreamNamesWithAliases() map[string]bool {
	mappingMu.RLock()
	defer mappingMu.RUnlock()
	out := make(map[string]bool, len(upstreamSet))
	for k, v := range upstreamSet {
		out[k] = v
	}
	return out
}

// RefreshMappingCache loads all mappings from the database and rebuilds the
// in-memory cache atomically.
func RefreshMappingCache() error {
	var mappings []ModelMapping
	if err := DB.Find(&mappings).Error; err != nil {
		return err
	}

	newAlias := make(map[string]string, len(mappings))
	newUpstream := make(map[string]bool)
	newList := make([]string, 0, len(mappings))

	for _, m := range mappings {
		newAlias[m.Alias] = m.UpstreamName
		for _, up := range ParseModelList(m.UpstreamName) {
			newUpstream[up] = true
		}
		newList = append(newList, m.Alias)
	}

	mappingMu.Lock()
	aliasToUp = newAlias
	upstreamSet = newUpstream
	aliasList = newList
	mappingMu.Unlock()

	return nil
}

// ---------------------------------------------------------------------------
// Database operations
// ---------------------------------------------------------------------------

// GetAllModelMappings returns every mapping row for the admin listing.
func GetAllModelMappings() ([]ModelMapping, error) {
	var mappings []ModelMapping
	if err := DB.Order("id asc").Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

// CreateModelMapping inserts a new mapping and refreshes the cache.
func CreateModelMapping(m *ModelMapping) error {
	m.CreatedAt = time.Now().Unix()
	if err := DB.Create(m).Error; err != nil {
		return err
	}
	return RefreshMappingCache()
}

// UpdateModelMapping saves changes to an existing mapping and refreshes the cache.
func UpdateModelMapping(m *ModelMapping) error {
	if err := DB.Save(m).Error; err != nil {
		return err
	}
	return RefreshMappingCache()
}

// DeleteModelMapping removes a mapping by ID and refreshes the cache.
func DeleteModelMapping(id int) error {
	if err := DB.Delete(&ModelMapping{}, id).Error; err != nil {
		return err
	}
	return RefreshMappingCache()
}
