package main

import "time"

type Task struct {
	Name           string
	Priority       int
	ParallelFactor int
	Effort         float64
	TaskType       string
	Dependencies   []string
	AssignedDevs   []*Developer
	StartTime      time.Time
	EndTime        time.Time
	IsCompleted    bool
	DevStartTimes  map[string]time.Time
}

type Developer struct {
	Name         string
	Role         string
	TaskTypes    []string
	NextFreeTime time.Time
}

type Role struct {
	Name                string
	AvailabilityPercent float64
}

type OnCall struct {
	DevName   string
	StartTime time.Time
	EndTime   time.Time
}

type Leave struct {
	DevName   string
	StartTime time.Time
	EndTime   time.Time
}
