package graph

import "time"

// MonthMark is the first row of a month in the history: where, reading down, the commits
// stop being from one month and start being from another.
type MonthMark struct {
	Row int `json:"row"`
	// Time is when that row's commit was made, in Unix seconds.
	Time int64 `json:"time"`
}

// MonthMarks returns where the month changes in the rows laid out so far, in local time and
// in row order, starting with the first row. It reads only what's loaded.
//
// A long history has millions of rows and a few hundred months, so the month of a row is
// only worked out when its time falls outside the month of the row before.
func (g *Graph) MonthMarks() []MonthMark {
	g.mu.RLock()
	defer g.mu.RUnlock()
	marks := []MonthMark{}
	var from, to int64 // the current month, as [from, to) in Unix seconds
	for i, t := range g.times {
		if i > 0 && t >= from && t < to {
			continue
		}
		at := time.Unix(t, 0).Local()
		start := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, at.Location())
		from, to = start.Unix(), start.AddDate(0, 1, 0).Unix()
		marks = append(marks, MonthMark{Row: i, Time: t})
	}
	return marks
}
