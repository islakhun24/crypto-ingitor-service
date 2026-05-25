package retention

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	monthlyPartitionPattern = regexp.MustCompile(`(?i)_(\d{4})_(\d{2})$`)
	weeklyPartitionPattern  = regexp.MustCompile(`(?i)_(\d{4})_w(\d{2})$`)
)

func ParsePartitionRange(parentTable string, partitionName string) (time.Time, time.Time, bool) {
	name := strings.TrimSpace(partitionName)
	if parentTable != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(parentTable)+"_") {
		return time.Time{}, time.Time{}, false
	}

	if match := monthlyPartitionPattern.FindStringSubmatch(name); len(match) == 3 {
		year, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		if month < 1 || month > 12 {
			return time.Time{}, time.Time{}, false
		}
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0), true
	}

	if match := weeklyPartitionPattern.FindStringSubmatch(name); len(match) == 3 {
		year, _ := strconv.Atoi(match[1])
		week, _ := strconv.Atoi(match[2])
		if week < 1 || week > 53 {
			return time.Time{}, time.Time{}, false
		}
		start := isoWeekStart(year, week)
		return start, start.AddDate(0, 0, 7), true
	}

	return time.Time{}, time.Time{}, false
}

func EligiblePartitions(parentTable string, names []string, cutoff time.Time) []Partition {
	partitions := make([]Partition, 0, len(names))
	for _, name := range names {
		start, end, ok := ParsePartitionRange(parentTable, name)
		if !ok || end.After(cutoff) {
			continue
		}
		partitions = append(partitions, Partition{
			SchemaName: "public",
			TableName:  name,
			RangeStart: start,
			RangeEnd:   end,
		})
	}

	return partitions
}

func isoWeekStart(year int, week int) time.Time {
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekOneMonday := jan4.AddDate(0, 0, -(weekday - 1))
	return weekOneMonday.AddDate(0, 0, (week-1)*7)
}

func qualifiedPartitionName(partition Partition) string {
	if partition.SchemaName == "" {
		return partition.TableName
	}
	return fmt.Sprintf("%s.%s", partition.SchemaName, partition.TableName)
}
