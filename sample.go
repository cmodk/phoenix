package phoenix

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
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

	//Optional values
	Diff *float64 `json:"diff,omitempty"`
}

type SampleCriteria struct {
	Streams     []string  `schema:"stream,omitempty" db:"stream"`
	From        time.Time `schema:"from,omitempty" db:"from"`
	To          time.Time `schema:"to,omitempty" db:"to"`
	Frequency   string    `schema:"frequency,omitempty" db:"frequency"`
	IncludeDiff bool      `schema:"include_diff,omitempty"`

	Limit int `schema:"limit,omitempty"`
}

type Samples []Sample

func (samples Samples) Len() int {
	return len(samples)
}

func (samples Samples) Swap(i int, j int) {
	samples[i], samples[j] = samples[j], samples[i]
}

func (samples Samples) Less(i int, j int) bool {
	return samples[i].Timestamp.After(samples[j].Timestamp)
}

type StreamStringValue struct {
	Timestamp time.Time `json:"timestamp"`
	Value     string    `json:"value"`
}

type AverageConfig struct {
	ScheduleTime time.Duration
	Duration     time.Duration
}

var (
	AverageConfigs = map[string]AverageConfig{
		"minute": {10 * time.Second, time.Minute},
		"hour":   {10 * time.Minute, time.Hour},
		"day":    {6 * time.Hour, 24 * time.Hour},
	}
)

func FrequencyToDuration(frequency string) time.Duration {
	return AverageConfigs[frequency].Duration
}

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

	phoenix.Logger.Infof("%s: now: %s:%s -> %s -> %s: %s\n",
		averageKey,
		sampleTime,
		calculationTime.Format(time.RFC3339),
		time.Now().Format(time.RFC3339),
		schedulation_time.Format(time.RFC3339),
		key)

	z := redis.Z{
		Score:  float64(schedulation_time.Unix()),
		Member: key,
	}

	if err := re.ZAdd(ctx, "averages", &z).Err(); err != nil {
		return err
	}

	return nil

}
