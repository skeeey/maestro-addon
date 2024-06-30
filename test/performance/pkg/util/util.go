package util

import (
	"fmt"
	"time"
)

const clusterNamePrefix = "maestro-cluster"

func ClusterName(index int) string {
	return fmt.Sprintf("%s-%d", clusterNamePrefix, index)
}

func UsedTime(start time.Time, unit time.Duration) time.Duration {
	used := time.Since(start)
	return used / unit
}
