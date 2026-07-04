package cache

import "time"

// TTLs configures how long each cached data type is retained. A zero value
// for a field disables caching for that type — Store.Set already treats
// ttl <= 0 as "don't cache".
type TTLs struct {
	CurrentUser        time.Duration
	Project            time.Duration
	MergeRequestList   time.Duration
	MergeRequestDetail time.Duration
	Diff               time.Duration
	Pipeline           time.Duration
	PipelineJobs       time.Duration
	Commits            time.Duration
	ContributionEvents time.Duration
}

// DefaultTTLs returns gzlab's built-in cache retention windows.
func DefaultTTLs() TTLs {
	return TTLs{
		CurrentUser:        24 * time.Hour,
		Project:            24 * time.Hour,
		MergeRequestList:   45 * time.Second,
		MergeRequestDetail: 45 * time.Second,
		Diff:               5 * time.Minute,
		Pipeline:           15 * time.Second,
		PipelineJobs:       15 * time.Second,
		Commits:            5 * time.Minute,
		ContributionEvents: 5 * time.Minute,
	}
}
