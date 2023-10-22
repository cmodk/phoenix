package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cmodk/phoenix"
	"github.com/sirupsen/logrus"
)

const ()

var (
	app = phoenix.New()
	ctx = context.Background()
	re  = app.Redis
	lg  = app.Logger
	ca  = app.Cassandra

	popmax = flag.Int64("max-pop", 10, "Maximum number of expired messages to retrive at a time")
	debug  = flag.Bool("debug", false, "Enable debug messages")
)

func main() {
	flag.Parse()
	if *debug {
		app.Logger.Level = logrus.DebugLevel
	} else {
		app.Logger.Level = logrus.ErrorLevel
	}

	app.HandleEvent(phoenix.SampleSaved{}, sampleSaved)

	go handleRedis()

	app.ListenEvents()
}

func sampleSaved(event interface{}) error {
	e := event.(phoenix.SampleSaved)

	for average_key, _ := range phoenix.AverageConfigs {
		phoenix.ScheduleCalculation(re, ctx, e.Timestamp, average_key, e.Device, e.Stream, false)
	}

	return nil
}

func handleRedis() {

	key := "averages"

	for {

		count, err := re.ZCount(ctx, key, "0", fmt.Sprintf("%d", time.Now().Unix())).Result()
		if err != nil {
			panic(err)
		}

		if count == 0 {
			time.Sleep(time.Second)
			continue
		}

		popCount := *popmax

		if count <= *popmax {
			popCount = 1
		}

		expired, err := re.ZPopMin(ctx, key, popCount).Result()
		if err != nil {
			panic(err)
		}
		for _, exp := range expired {
			split := strings.Split(exp.Member.(string), "/")
			if len(split) != 4 {
				lg.WithField("zmember", exp.Member).Errorf("Wrong length for split z member: %d\n", len(split))
				continue
			}

			unix_time, err := strconv.ParseInt(split[0], 10, 64)
			if err != nil {
				lg.WithField("error", err).Errorf("Error parsing z member: %s\n", exp.Member)
				continue
			}

			calculation_time := time.Unix(unix_time, 0)
			average_key := split[1]
			device := split[2]
			stream := split[3]
			lg.Infof("exp: %f -> %s -> %s -> %s -> %s -> %s\n",
				exp.Score,
				exp.Member,
				calculation_time.Format(time.RFC3339),
				average_key,
				device,
				stream)

			calculation_time_end := calculation_time.Add(phoenix.AverageConfigs[average_key].Duration)

			query := ca.Query("SELECT value FROM samples WHERE device = ? AND stream = ? AND timestamp >= ? and timestamp < ?",
				device,
				stream,
				calculation_time,
				calculation_time_end)

			average := 0.0
			sample_count := 0
			max := math.NaN()
			min := math.NaN()

			iter := query.Iter()
			for {
				row := make(map[string]interface{})
				if !iter.MapScan(row) {
					break
				}

				value := row["value"].(float64)
				sample_count++
				average += value

				if math.IsNaN(max) || value > max {
					max = value
				}

				if math.IsNaN(min) || value < min {
					min = value
				}

			}

			if sample_count > 0 {
				average /= float64(sample_count)

				iter.Close()
				lg.Infof("Average(%d): %f, Max: %f, Min: %f\n", sample_count, average, max, min)

				insert := ca.Query(fmt.Sprintf("INSERT INTO samples_%s (device,stream,timestamp,average,max,min,count) VALUES(?,?,?,?,?,?,?)", average_key),
					device,
					stream,
					calculation_time,
					average,
					max,
					min,
					sample_count)

				if err := insert.Exec(); err != nil {
					lg.WithField("error", err).Errorf("Error inserting aggregated sample - MUST NOT HAPPEN!")
					panic(err)
				}
			} else {
				lg.WithField("exp", exp).Debugf("Skipping average, count =0")
			}

		}

	}

}
