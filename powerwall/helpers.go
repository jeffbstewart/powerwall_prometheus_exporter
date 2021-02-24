package powerwall

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"math"
	"regexp"
	"strconv"
	"time"
)

type TimeZone struct {
	loc *time.Location
}

func (t TimeZone) Location() *time.Location {
	return t.loc
}

func (t *TimeZone) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	ld, err := time.LoadLocation(s)
	if err != nil {
		// this is failing the whole shebang when run on a machine
		// without go installed.  Just log it and live without
		// knowing the timezone.
		glog.Errorf("The server reports timezone %q, but we can't decode it because: %v", s, err)
		return nil
	}
	t.loc = ld
	return nil
}

func (t TimeZone) MarshalJSON() ([]byte, error) {
	if t.loc == nil {
		return []byte(`nil`), nil
	}
	return []byte(t.loc.String()), nil
}

type Time struct {
	t time.Time
}

func (t Time) Time() time.Time {
	return t.t
}

var stripFractionalSeconds = regexp.MustCompile("^(.*)\\.\\d+(.*)$")

func (t *Time) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		var zero time.Time
		t.t = zero
		return nil
	}
	if got := stripFractionalSeconds.FindStringSubmatch(s); got != nil {
		s = got[1] + got[2]
	}
	// known formats:
	// YYYY-MM-DDTHH:MM:SS.XXXXXXXXX-XX:XX

	layouts := []string{
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02T15:04:05Z",
	}
	for _, l := range layouts {
		g, err := time.Parse(l, s)
		if err == nil {
			t.t = g
			return nil
		}
	}
	return fmt.Errorf("no layout matched timestamp %q", s)
}

type FloatDurationSeconds struct {
	d time.Duration
}

func (f FloatDurationSeconds) Duration() time.Duration {
	return f.d
}

func (f *FloatDurationSeconds) UnmarshalJSON(b []byte) error {
	var fj float64
	if err := json.Unmarshal(b, &fj); err != nil {
		return err
	}
	secs := time.Duration(int64(math.Trunc(fj))) * time.Second
	// ignoring fractional seconds
	f.d = secs
	return nil
}

type Duration struct {
	d time.Duration
}

func (d Duration) Duration() time.Duration {
	return d.d
}

// "143h54m32.539257895s"
var uptimeRegex = regexp.MustCompile(`((?P<hours>\d+?)h)?((?P<minutes>\d+?)m)?((?P<seconds>\d+?).)((?P<nanoseconds>\d+?)s)`)

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	match := uptimeRegex.FindStringSubmatch(s)
	result := make(map[string]int64)
	names := uptimeRegex.SubexpNames()
	for i, capture := range match {
		if names[i] == "" {
			continue
		}
		icap, err := strconv.ParseInt(capture, 10, 64)
		if err != nil {
			return err
		}
		result[names[i]] = icap
	}
	r := time.Duration(result["hours"]) * time.Hour
	r += time.Duration(result["minutes"]) * time.Minute
	r += time.Duration(result["seconds"]) * time.Second
	r += time.Duration(result["nanoseconds"]) * time.Nanosecond
	d.d = r
	return nil
}
