// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	gotime "time"

	"github.com/juju/errors"
)

type mysqlTime struct {
	year        uint16 // year <= 9999
	month       uint8  // month <= 12
	day         uint8  // day <= 31
	hour        uint8  // hour <= 23
	minute      uint8  // minute <= 59
	second      uint8  // second <= 59
	microsecond uint32
}

func (t mysqlTime) Year() int {
	return int(t.year)
}

func (t mysqlTime) Month() int {
	return int(t.month)
}

func (t mysqlTime) Day() int {
	return int(t.day)
}

func (t mysqlTime) Hour() int {
	return int(t.hour)
}

func (t mysqlTime) Minute() int {
	return int(t.minute)
}

func (t mysqlTime) Second() int {
	return int(t.second)
}

func (t mysqlTime) Microsecond() int {
	return int(t.microsecond)
}

func (t mysqlTime) Weekday() gotime.Weekday {
	t1, err := t.GoTime()
	if err != nil {
		return 0
	}
	return t1.Weekday()
}

func (t mysqlTime) YearDay() int {
	t1, err := t.GoTime()
	if err != nil {
		// TODO: Fix here.
		return 0
	}
	return t1.YearDay()
}

func (t mysqlTime) YearWeek(mode int) (int, int) {
	if t.month == 0 || t.day == 0 {
		return 0, 0
	}
	behavior := weekMode(mode) | weekBehaviourYear
	return calcWeek(&t, behavior)
}

func (t mysqlTime) Week(mode int) int {
	if t.month == 0 || t.day == 0 {
		return 0
	}
	_, week := calcWeek(&t, weekMode(mode))
	return week
}

func (t mysqlTime) GoTime() (gotime.Time, error) {
	// gotime.Time can't represent month 0 or day 0, date contains 0 would be converted to a nearest date,
	// For example, 2006-12-00 00:00:00 would become 2015-11-30 23:59:59.
	tm := gotime.Date(t.Year(), gotime.Month(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Microsecond()*1000, gotime.Local)
	year, month, day := tm.Date()
	hour, minute, second := tm.Clock()
	microsec := tm.Nanosecond() / 1000
	// This function will check the result, and return an error if it's not the same with the origin input.
	if year != t.Year() || int(month) != t.Month() || day != t.Day() ||
		hour != t.Hour() || minute != t.Minute() || second != t.Second() ||
		microsec != t.Microsecond() {
		return tm, errors.Trace(ErrInvalidTimeFormat)
	}
	return tm, nil
}

func newMysqlTime(year, month, day, hour, minute, second, microsecond int) mysqlTime {
	return mysqlTime{
		uint16(year),
		uint8(month),
		uint8(day),
		uint8(hour),
		uint8(minute),
		uint8(second),
		uint32(microsecond),
	}
}

// calcDaynr calculates days since 0000-00-00.
func calcDaynr(year, month, day int) int {
	if year == 0 && month == 0 {
		return 0
	}

	delsum := 365*year + 31*(month-1) + day
	if month <= 2 {
		year--
	} else {
		delsum -= (month*4 + 23) / 10
	}
	temp := ((year/100 + 1) * 3) / 4
	return delsum + year/4 - temp
}

// calcDaysInYear calculates days in one year, it works with 0 <= year <= 99.
func calcDaysInYear(year int) int {
	if (year&3) == 0 && (year%100 != 0 || (year%400 == 0 && (year != 0))) {
		return 366
	}
	return 365
}

// calcWeekday calculates weekday from daynr, returns 0 for Monday, 1 for Tuesday ...
func calcWeekday(daynr int, sundayFirstDayOfWeek bool) int {
	daynr += 5
	if sundayFirstDayOfWeek {
		daynr++
	}
	return daynr % 7
}

type weekBehaviour uint

const (
	// If set, Sunday is first day of week, otherwise Monday is first day of week.
	weekBehaviourMondayFirst weekBehaviour = 1 << iota
	// If set, Week is in range 1-53, otherwise Week is in range 0-53.
	// Note that this flag is only releveant if WEEK_JANUARY is not set.
	weekBehaviourYear
	// If not set, Weeks are numbered according to ISO 8601:1988.
	// If set, the week that contains the first 'first-day-of-week' is week 1.
	weekBehaviourFirstWeekday
)

func (v weekBehaviour) test(flag weekBehaviour) bool {
	return (v & flag) != 0
}

func weekMode(mode int) weekBehaviour {
	var weekFormat weekBehaviour
	weekFormat = weekBehaviour(mode & 7)
	if (weekFormat & weekBehaviourMondayFirst) == 0 {
		weekFormat ^= weekBehaviourFirstWeekday
	}
	return weekFormat
}

// calcWeek calculates week and year for the time.
func calcWeek(t *mysqlTime, wb weekBehaviour) (year int, week int) {
	var days int
	daynr := calcDaynr(int(t.year), int(t.month), int(t.day))
	firstDaynr := calcDaynr(int(t.year), 1, 1)
	mondayFirst := wb.test(weekBehaviourMondayFirst)
	weekYear := wb.test(weekBehaviourYear)
	firstWeekday := wb.test(weekBehaviourFirstWeekday)

	weekday := calcWeekday(int(firstDaynr), !mondayFirst)

	year = int(t.year)

	if t.month == 1 && int(t.day) <= 7-weekday {
		if !weekYear &&
			((firstWeekday && weekday != 0) || (!firstWeekday && weekday >= 4)) {
			week = 0
			return
		}
		weekYear = true
		(year)--
		days = calcDaysInYear(year)
		firstDaynr -= days
		weekday = (weekday + 53*7 - days) % 7
	}

	if (firstWeekday && weekday != 0) ||
		(!firstWeekday && weekday >= 4) {
		days = daynr - (firstDaynr + 7 - weekday)
	} else {
		days = daynr - (firstDaynr - weekday)
	}

	if weekYear && days >= 52*7 {
		weekday = (weekday + calcDaysInYear(year)) % 7
		if (!firstWeekday && weekday < 4) ||
			(firstWeekday && weekday == 0) {
			year++
			week = 1
			return
		}
	}
	week = days/7 + 1
	return
}
