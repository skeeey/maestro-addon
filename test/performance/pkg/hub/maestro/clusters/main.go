package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/stolostron/maestro-addon/pkg/helpers"
	"github.com/stolostron/maestro-addon/test/performance/pkg/common"
	"github.com/stolostron/maestro-addon/test/performance/pkg/util"
)

var (
	maestroServiceAddress = flag.String("maestro-service-address", "http://127.0.0.1:8000", "")
	beginIndex            = flag.Int("begin-index", 1, "Begin index of the clusters")
	counts                = flag.Int("counts", common.DEFAULT_CLUSTER_COUNTS, "Counts of the clusters")
)

func main() {
	flag.Parse()

	apiClient := helpers.NewMaestroAPIClient(*maestroServiceAddress)

	index := *beginIndex
	startTime := time.Now()
	for i := 0; i < *counts; i++ {
		clusterName := util.ClusterName(index)

		startTime := time.Now()
		if err := helpers.CreateConsumer(context.Background(), apiClient, clusterName); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("cluster %s is created, time=%dms\n", clusterName, util.UsedTime(startTime, time.Millisecond))
		index++
	}
	fmt.Printf("Clusters (%d) are created, time=%dms\n", *counts, util.UsedTime(startTime, time.Millisecond))
}
