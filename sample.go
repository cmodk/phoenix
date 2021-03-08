package phoenix

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

type Sample struct {
	DeviceId  *uint64   `json:"-"`
	Device    string    `json:"device"`
	Stream    string    `json:"stream"`
	Timestamp time.Time `json:"timestamp"`

	//For raw samples
	Value *float64 `json:"value,omitempty"`

	//For aggregated
	Average *float64 `json:"average,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Min     *float64 `json:"min,omitempty"`
	Count   *int     `json:"count,omitempty"`
}

type SampleCriteria struct {
	Streams   []string  `schema:"stream"`
	From      time.Time `schema:"from"`
	To        time.Time `schema:"to"`
	Frequency string    `schema:"frequency"`

	Limit int `schema:"limit"`
}

type AverageConfig struct {
	ScheduleTime time.Duration
	Duration     time.Duration
}

var (
	AverageConfigs = map[string]AverageConfig{
		"minute": {10 * time.Second, time.Minute},
		"hour":   {10 * time.Minute, time.Hour},
	}
)

func ScheduleCalculation(re *redis.Client, ctx context.Context, sampleTime time.Time, averageKey string, deviceGuid string, stream string) error {
	average_config, ok := AverageConfigs[averageKey]
	if !ok {
		return fmt.Errorf("Bad average config requested: %s\n", averageKey)
	}

	calculationTime := sampleTime.Truncate(average_config.Duration)

	key := fmt.Sprintf("%d/%s/%s/%s", calculationTime.Unix(), averageKey, deviceGuid, stream)

	schedulation_time := time.Now().
		//Truncate(average_config.Duration).
		//Add(average_config.ScheduleTime).
		Add(average_config.ScheduleTime)

	phoenix.Logger.Infof("%s: now: %s -> %s -> %s\n",
		averageKey,
		calculationTime.Format(time.RFC3339),
		time.Now().Format(time.RFC3339),
		schedulation_time.Format(time.RFC3339))

	z := redis.Z{
		Score:  float64(schedulation_time.Unix()),
		Member: key,
	}

	if err := re.ZAdd(ctx, "averages", &z).Err(); err != nil {
		return err
	}

	return nil

}
