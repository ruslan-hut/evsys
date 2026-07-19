package common

import (
	"fmt"
	"reflect"
	"sync"
)

// FeatureRegistry manages the registration and lookup of OCPP features across protocol versions
// It provides a centralized registry for mapping action names to their corresponding request/response types
type FeatureRegistry interface {
	// RegisterFeature registers a feature for a specific protocol version
	// Parameters:
	//   - version: The protocol version (e.g., OCPP16, OCPP201)
	//   - action: The feature/action name (e.g., "BootNotification", "Authorize")
	//   - requestType: The Go reflection type of the request struct
	//   - responseType: The Go reflection type of the response struct
	RegisterFeature(version ProtocolVersion, action string, requestType, responseType reflect.Type)

	// GetTypes retrieves the request and response types for a specific version and action
	// Returns an error if the feature is not registered
	GetTypes(version ProtocolVersion, action string) (requestType, responseType reflect.Type, err error)

	// IsSupported checks if a feature is supported for a specific protocol version
	IsSupported(version ProtocolVersion, action string) bool

	// GetFeatures returns all registered feature names for a specific protocol version
	GetFeatures(version ProtocolVersion) []string

	// GetVersions returns all protocol versions that have registered features
	GetVersions() []ProtocolVersion

	// GetFeatureCount returns the total number of features registered for a specific version
	GetFeatureCount(version ProtocolVersion) int

	// UnregisterFeature removes a feature registration (useful for testing)
	UnregisterFeature(version ProtocolVersion, action string)

	// Clear removes all feature registrations (useful for testing)
	Clear()
}

// featureTypes holds the request and response types for a feature
type featureTypes struct {
	requestType  reflect.Type
	responseType reflect.Type
}

// featureRegistry is the default implementation of FeatureRegistry
type featureRegistry struct {
	// registry maps: version → action → types
	registry map[ProtocolVersion]map[string]featureTypes
	mu       sync.RWMutex
}

// NewFeatureRegistry creates a new feature registry
func NewFeatureRegistry() FeatureRegistry {
	return &featureRegistry{
		registry: make(map[ProtocolVersion]map[string]featureTypes),
	}
}

// RegisterFeature registers a feature for a specific protocol version
func (r *featureRegistry) RegisterFeature(version ProtocolVersion, action string, requestType, responseType reflect.Type) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize version map if it doesn't exist
	if r.registry[version] == nil {
		r.registry[version] = make(map[string]featureTypes)
	}

	// Register the feature
	r.registry[version][action] = featureTypes{
		requestType:  requestType,
		responseType: responseType,
	}
}

// GetTypes retrieves the request and response types for a specific version and action
func (r *featureRegistry) GetTypes(version ProtocolVersion, action string) (requestType, responseType reflect.Type, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if version exists
	versionMap, ok := r.registry[version]
	if !ok {
		return nil, nil, fmt.Errorf("protocol version not supported: %s", version)
	}

	// Check if action exists
	types, ok := versionMap[action]
	if !ok {
		return nil, nil, fmt.Errorf("feature not supported for version %s: %s", version, action)
	}

	return types.requestType, types.responseType, nil
}

// IsSupported checks if a feature is supported for a specific protocol version
func (r *featureRegistry) IsSupported(version ProtocolVersion, action string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versionMap, ok := r.registry[version]
	if !ok {
		return false
	}

	_, ok = versionMap[action]
	return ok
}

// GetFeatures returns all registered feature names for a specific protocol version
func (r *featureRegistry) GetFeatures(version ProtocolVersion) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versionMap, ok := r.registry[version]
	if !ok {
		return []string{}
	}

	features := make([]string, 0, len(versionMap))
	for action := range versionMap {
		features = append(features, action)
	}

	return features
}

// GetVersions returns all protocol versions that have registered features
func (r *featureRegistry) GetVersions() []ProtocolVersion {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions := make([]ProtocolVersion, 0, len(r.registry))
	for version := range r.registry {
		versions = append(versions, version)
	}

	return versions
}

// GetFeatureCount returns the total number of features registered for a specific version
func (r *featureRegistry) GetFeatureCount(version ProtocolVersion) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versionMap, ok := r.registry[version]
	if !ok {
		return 0
	}

	return len(versionMap)
}

// UnregisterFeature removes a feature registration
func (r *featureRegistry) UnregisterFeature(version ProtocolVersion, action string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if versionMap, ok := r.registry[version]; ok {
		delete(versionMap, action)
	}
}

// Clear removes all feature registrations
func (r *featureRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.registry = make(map[ProtocolVersion]map[string]featureTypes)
}

// globalRegistry is the singleton instance used throughout the application
var globalRegistry = NewFeatureRegistry()

// GetGlobalRegistry returns the global feature registry instance
func GetGlobalRegistry() FeatureRegistry {
	return globalRegistry
}

// RegisterFeature is a convenience function to register a feature in the global registry
func RegisterFeature(version ProtocolVersion, action string, requestType, responseType reflect.Type) {
	globalRegistry.RegisterFeature(version, action, requestType, responseType)
}

// GetTypes is a convenience function to get types from the global registry
func GetTypes(version ProtocolVersion, action string) (requestType, responseType reflect.Type, err error) {
	return globalRegistry.GetTypes(version, action)
}

// IsSupported is a convenience function to check support in the global registry
func IsSupported(version ProtocolVersion, action string) bool {
	return globalRegistry.IsSupported(version, action)
}

// GetFeatures is a convenience function to get features from the global registry
func GetFeatures(version ProtocolVersion) []string {
	return globalRegistry.GetFeatures(version)
}
