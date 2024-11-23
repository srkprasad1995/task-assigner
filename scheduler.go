package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type Scheduler struct {
	tasks      []*Task
	developers []*Developer
	roles      map[string]*Role
	oncalls    []OnCall
	leaves     []Leave
}

func NewScheduler(tasks []*Task, devs []*Developer, roles map[string]*Role, oncalls []OnCall, leaves []Leave) *Scheduler {
	return &Scheduler{
		tasks:      tasks,
		developers: devs,
		roles:      roles,
		oncalls:    oncalls,
		leaves:     leaves,
	}
}

func (s *Scheduler) debug(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}

func (s *Scheduler) areDependenciesCompleted(task *Task) bool {
	s.debug("Checking dependencies for task: %s", task.Name)
	for _, depName := range task.Dependencies {
		if !s.isDependencyCompleted(depName) {
			s.debug("Dependency %s not completed for task %s", depName, task.Name)
			return false
		}
	}
	s.debug("All dependencies completed for task %s", task.Name)
	return true
}

func (s *Scheduler) isDependencyCompleted(depName string) bool {
	for _, t := range s.tasks {
		if t.Name == depName {
			s.debug("Checking dependency %s: completed=%v", depName, t.IsCompleted)
			return t.IsCompleted
		}
	}
	return false
}

func (s *Scheduler) findAvailableDevs(task *Task, date time.Time) []*Developer {
	s.debug("Finding available developers for task: %s at date: %v", task.Name, date)
	var availableDevs []*Developer

	for _, dev := range s.developers {
		if s.isDevAvailableForTask(dev, task, date) {
			availableDevs = append(availableDevs, dev)
			s.debug("Developer %s is available for task %s", dev.Name, task.Name)
		}
	}

	s.debug("Found %d available developers for task %s", len(availableDevs), task.Name)
	return availableDevs
}

func (s *Scheduler) isDevAvailableForTask(dev *Developer, task *Task, date time.Time) bool {
	if !s.canDevWorkOnTaskType(dev, task.TaskType) {
		s.debug("Developer %s cannot work on task type: %s", dev.Name, task.TaskType)
		return false
	}

	if date.Before(dev.NextFreeTime) {
		s.debug("Developer %s is busy until: %v", dev.Name, dev.NextFreeTime)
		return false
	}

	if s.isDevOnCall(dev, date) {
		return false
	}

	if s.isDevOnLeave(dev, date) {
		return false
	}

	return true
}

func (s *Scheduler) canDevWorkOnTaskType(dev *Developer, taskType string) bool {
	for _, t := range dev.TaskTypes {
		if t == taskType {
			return true
		}
	}
	return false
}

func (s *Scheduler) isDevOnCall(dev *Developer, date time.Time) bool {
	for _, oncall := range s.oncalls {
		if oncall.DevName == dev.Name &&
			!date.Before(oncall.StartTime) &&
			!date.After(oncall.EndTime) {
			s.debug("Developer %s is on-call between %v and %v", dev.Name, oncall.StartTime, oncall.EndTime)
			return true
		}
	}
	return false
}

func (s *Scheduler) isDevOnLeave(dev *Developer, date time.Time) bool {
	for _, leave := range s.leaves {
		if leave.DevName == dev.Name &&
			!date.Before(leave.StartTime) &&
			!date.After(leave.EndTime) {
			s.debug("Developer %s is on leave between %v and %v", dev.Name, leave.StartTime, leave.EndTime)
			return true
		}
	}
	return false
}

func (s *Scheduler) calculateEndDate(devs []*Developer, startDate time.Time, effortPerDev float64) time.Time {
	s.debug("Calculating end date for effort %.2f starting at %v", effortPerDev, startDate)
	currentDate := startDate
	remainingEffort := effortPerDev
	maxIterations := 365 // Safety limit to prevent infinite loops

	iterations := 0
	for remainingEffort > 0 && iterations < maxIterations {
		if isWeekend(currentDate) {
			currentDate = currentDate.AddDate(0, 0, 1)
			continue
		}

		availableDevs := make([]*Developer, 0)
		for _, dev := range devs {
			if !s.isDevOnCall(dev, currentDate) && !s.isDevOnLeave(dev, currentDate) {
				availableDevs = append(availableDevs, dev)
			}
		}

		dailyProgress := s.calculateDailyProgress(availableDevs)
		if dailyProgress > 0 {
			remainingEffort -= dailyProgress
		}

		if remainingEffort > 0 {
			currentDate = currentDate.AddDate(0, 0, 1)
		}
		iterations++
	}

	if iterations >= maxIterations {
		s.debug("WARNING: Max iterations reached while calculating end date")
		return startDate.AddDate(1, 0, 0) // Return date 1 year in future as fallback
	}

	s.debug("Calculated end date: %v", currentDate)
	return currentDate
}

func (s *Scheduler) calculateDailyProgress(devs []*Developer) float64 {
	dailyProgress := 0.0
	for _, dev := range devs {
		if role, exists := s.roles[dev.Role]; exists {
			availability := role.AvailabilityPercent
			dailyProgress += availability
			s.debug("Developer %s contributes %.2f progress with %.2f availability",
				dev.Name, availability, role.AvailabilityPercent)
		}
	}
	return dailyProgress
}

func (s *Scheduler) Schedule(startDate time.Time) {
	s.debug("Starting scheduling from date: %v", startDate)
	s.initializeSchedule(startDate)

	currentDate := startDate
	maxIterations := 3650 // Safety limit
	iterations := 0

	for iterations < maxIterations {
		if s.processSchedulingIteration(currentDate) {
			break
		}
		currentDate = addWorkdays(currentDate, 1)
		iterations++
	}

	if iterations >= maxIterations {
		s.debug("WARNING: Max scheduling iterations reached")
	}

	s.writeScheduleToCSV()
}

func (s *Scheduler) initializeSchedule(startDate time.Time) {
	s.tasks = s.filterTasksWithValidDevs()
	s.sortTasksByPriority()
	s.initializeDevStartTimes(startDate)
}

func (s *Scheduler) filterTasksWithValidDevs() []*Task {
	var validTasks []*Task
	for _, task := range s.tasks {
		if s.hasMatchingDeveloper(task) {
			validTasks = append(validTasks, task)
		}
	}
	return validTasks
}

func (s *Scheduler) hasMatchingDeveloper(task *Task) bool {
	for _, dev := range s.developers {
		for _, devTaskType := range dev.TaskTypes {
			if devTaskType == task.TaskType {
				return true
			}
		}
	}
	return false
}

func (s *Scheduler) sortTasksByPriority() {
	sort.Slice(s.tasks, func(i, j int) bool {
		return s.tasks[i].Priority < s.tasks[j].Priority
	})
}

func (s *Scheduler) initializeDevStartTimes(startDate time.Time) {
	for _, dev := range s.developers {
		dev.NextFreeTime = startDate
	}
}

func (s *Scheduler) processSchedulingIteration(currentDate time.Time) bool {
	completedAll := true

	s.processCompletedTasks()

	for _, task := range s.tasks {
		if !s.processTask(task, currentDate) {
			completedAll = false
		}
	}

	return completedAll
}

func (s *Scheduler) processCompletedTasks() {
	for _, task := range s.tasks {
		if task.IsCompleted && task.AssignedDevs != nil {
			s.processCompletedTask(task)
		}
	}
}

func (s *Scheduler) processTask(task *Task, currentDate time.Time) bool {
	if task.IsCompleted {
		return true
	}

	if !s.areDependenciesCompleted(task) {
		return false
	}

	availableDevs := s.findAvailableDevs(task, currentDate)
	if len(availableDevs) > 0 {
		s.assignDevsToTask(task, availableDevs, currentDate)
		s.updateTaskCompletion(task, currentDate)
	}

	return task.IsCompleted
}

func (s *Scheduler) assignDevsToTask(task *Task, availableDevs []*Developer, currentDate time.Time) {
	if task.AssignedDevs == nil {
		s.initializeTaskAssignment(task, currentDate)
	}

	remainingSlots := task.ParallelFactor - len(task.AssignedDevs)
	if remainingSlots > 0 {
		s.fillTaskSlots(task, availableDevs, remainingSlots, currentDate)
	}
}

func (s *Scheduler) initializeTaskAssignment(task *Task, currentDate time.Time) {
	task.AssignedDevs = make([]*Developer, 0)
	task.DevStartTimes = make(map[string]time.Time)
	task.StartTime = currentDate
}

func (s *Scheduler) fillTaskSlots(task *Task, availableDevs []*Developer, remainingSlots int, currentDate time.Time) {
	slotsToFill := min(len(availableDevs), remainingSlots)
	newDevs := availableDevs[:slotsToFill]

	if len(newDevs) == 0 {
		s.debug("No available developers to fill task slots for task %s", task.Name)
		return
	}

	task.AssignedDevs = append(task.AssignedDevs, newDevs...)

	for _, dev := range newDevs {
		task.DevStartTimes[dev.Name] = currentDate
	}

	s.updateTaskEndTime(task, newDevs, currentDate)
}

func (s *Scheduler) updateTaskEndTime(task *Task, newDevs []*Developer, currentDate time.Time) {
	daysWorked := currentDate.Sub(task.StartTime).Hours() / 24
	progressPerDay := 0.0
	for _, dev := range task.AssignedDevs[:len(task.AssignedDevs)-len(newDevs)] {
		if role, exists := s.roles[dev.Role]; exists {
			progressPerDay += role.AvailabilityPercent
		}
	}
	remainingEffort := task.Effort - (progressPerDay * daysWorked)
	task.EndTime = s.calculateEndDate(task.AssignedDevs, currentDate, remainingEffort)

	for _, dev := range task.AssignedDevs {
		dev.NextFreeTime = task.EndTime
	}
}

func (s *Scheduler) updateTaskCompletion(task *Task, currentDate time.Time) {
	if !currentDate.Before(task.EndTime) {
		task.IsCompleted = true
	}
}

func (s *Scheduler) writeScheduleToCSV() {
	file, err := os.Create("schedule.csv")
	if err != nil {
		s.debug("Error creating schedule file: %v", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	s.writeCSVHeader(writer)
	s.writeCSVRecords(writer)
}

func (s *Scheduler) writeCSVHeader(writer *csv.Writer) {
	header := []string{"Task", "Start Date", "End Date", "Assigned Developers", "Effort Per Developer"}
	if err := writer.Write(header); err != nil {
		s.debug("Error writing header: %v", err)
	}
}

func (s *Scheduler) writeCSVRecords(writer *csv.Writer) {
	writeRecord := s.createRecordWriter(writer)

	s.writeOncallRecords(writeRecord)
	s.writeLeaveRecords(writeRecord)
	s.writeTaskRecords(writeRecord)
}

func (s *Scheduler) createRecordWriter(writer *csv.Writer) func([]string) {
	return func(record []string) {
		if err := writer.Write(record); err != nil {
			s.debug("Error writing record: %v", err)
			return
		}
		fmt.Println(strings.Join(record, ","))
	}
}

func (s *Scheduler) writeOncallRecords(writeRecord func([]string)) {
	for _, oncall := range s.oncalls {
		record := []string{
			"On-Call Duty",
			oncall.StartTime.Format("2006-01-02"),
			oncall.EndTime.Format("2006-01-02"),
			oncall.DevName,
			fmt.Sprintf("%.2f", float64(oncall.EndTime.Sub(oncall.StartTime).Hours()/24)),
		}
		writeRecord(record)
	}
}

func (s *Scheduler) writeLeaveRecords(writeRecord func([]string)) {
	for _, leave := range s.leaves {
		record := []string{
			"Leave",
			leave.StartTime.Format("2006-01-02"),
			leave.EndTime.Format("2006-01-02"),
			leave.DevName,
			fmt.Sprintf("%.2f", float64(leave.EndTime.Sub(leave.StartTime).Hours()/24)),
		}
		writeRecord(record)
	}
}

func (s *Scheduler) writeTaskRecords(writeRecord func([]string)) {
	for _, task := range s.tasks {
		if !task.IsCompleted {
			continue
		}
		for devName, startTime := range task.DevStartTimes {
			record := []string{
				task.Name,
				startTime.Format("2006-01-02"),
				task.EndTime.Format("2006-01-02"),
				strings.TrimSpace(devName),
				fmt.Sprintf("%.2f", float64(task.EndTime.Sub(startTime).Hours()/24)),
			}
			writeRecord(record)
		}
	}
}

func (s *Scheduler) processCompletedTask(task *Task) {
	s.debug("Processing completed task: %s", task.Name)
	if task.AssignedDevs == nil {
		s.debug("No developers assigned to task %s", task.Name)
		return
	}

	for _, dev := range task.AssignedDevs {
		s.debug("Freed developer %s from task %s at %v", dev.Name, task.Name, task.EndTime)
	}
}

func isWeekend(date time.Time) bool {
	day := date.Weekday()
	return day == time.Saturday || day == time.Sunday
}

func addWorkdays(date time.Time, days int) time.Time {
	result := date
	for days > 0 {
		result = result.AddDate(0, 0, 1)
		if !isWeekend(result) {
			days--
		}
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
