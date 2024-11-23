package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

func main2() {
	// Create and write sample data to CSV files
	// createTasksCSV()
	// createDevelopersCSV()
	// createRolesCSV()
	// createOncallsCSV()
	// createLeavesCSV()

	// Load data from CSVs
	tasks, developers, roles, err := LoadFromCSV("roles.csv", "tasks.csv", "developers.csv")
	if err != nil {
		panic(err)
	}

	// Load oncalls
	oncalls, err := loadOncalls("oncalls.csv")
	if err != nil {
		panic(err)
	}

	// Load leaves
	leaves, err := loadLeaves("leaves.csv")
	if err != nil {
		panic(err)
	}

	// Create and run scheduler
	scheduler := NewScheduler(tasks, developers, roles, oncalls, leaves)
	scheduler.Schedule(time.Now())
}

func loadOncalls(filename string) ([]OnCall, error) {
	var oncalls []OnCall
	records, err := readCSV(filename)
	if err != nil {
		return nil, err
	}

	for _, record := range records[1:] { // Skip header
		startTime, _ := time.Parse("2006-01-02", record[1])
		endTime, _ := time.Parse("2006-01-02", record[2])
		oncalls = append(oncalls, OnCall{
			DevName:   record[0],
			StartTime: startTime,
			EndTime:   endTime,
		})
	}
	return oncalls, nil
}

func loadLeaves(filename string) ([]Leave, error) {
	var leaves []Leave
	records, err := readCSV(filename)
	if err != nil {
		return nil, err
	}

	for _, record := range records[1:] { // Skip header
		startTime, _ := time.Parse("2006-01-02", record[1])
		endTime, _ := time.Parse("2006-01-02", record[2])
		leaves = append(leaves, Leave{
			DevName:   record[0],
			StartTime: startTime,
			EndTime:   endTime,
		})
	}
	return leaves, nil
}

func LoadFromCSV(rolesFile, tasksFile, devsFile string) ([]*Task, []*Developer, map[string]*Role, error) {
	// Load Roles
	roles := make(map[string]*Role)
	if roleRecords, err := readCSV(rolesFile); err == nil {
		for _, record := range roleRecords[1:] { // Skip header
			availability, _ := strconv.ParseFloat(record[1], 64)
			roles[record[0]] = &Role{
				Name:                record[0],
				AvailabilityPercent: availability,
			}
		}
	} else {
		return nil, nil, nil, fmt.Errorf("error reading roles: %v", err)
	}

	// Load Tasks
	var tasks []*Task
	if taskRecords, err := readCSV(tasksFile); err == nil {
		for _, record := range taskRecords[1:] { // Skip header
			priority, _ := strconv.Atoi(record[2])
			effort, _ := strconv.ParseFloat(record[3], 64)
			parallel, _ := strconv.Atoi(record[4])
			dependencies := strings.Split(record[5], ",")
			if record[5] == "" {
				dependencies = []string{}
			}

			// Parse FE and QA required flags
			needsFE := false
			needsQA := false
			if len(record) > 6 {
				needsFE = record[6] == "true"
			}
			if len(record) > 7 {
				needsQA = record[7] == "true"
			}

			// Create main task
			taskName := record[0]
			mainTask := &Task{
				Name:           taskName,
				TaskType:       record[1],
				Priority:       priority,
				ParallelFactor: parallel,
				Dependencies:   dependencies,
				IsCompleted:    false,
			}

			// Increase effort by (10*parallelFactor + 40)%
			effortIncrease := 1.0 + float64(10*parallel+40)/100.0
			mainTask.Effort = math.Round(effort * effortIncrease)

			tasks = append(tasks, mainTask)

			// Add Frontend task if flag is true
			var feTaskName string
			if needsFE {
				feTaskName = taskName + "_Frontend"
				feTask := &Task{
					Name:           feTaskName,
					TaskType:       "Frontend",
					Priority:       priority,
					Effort:         math.Round(effort * 0.25 * effortIncrease),
					ParallelFactor: 1,
					Dependencies:   []string{taskName},
					IsCompleted:    false,
				}
				tasks = append(tasks, feTask)
			}

			// Add QA task if both flags are true
			if needsQA {
				qaTaskName := taskName + "_QA"
				dependencies := []string{taskName}
				if needsFE {
					dependencies = append(dependencies, feTaskName)
				}
				qaTask := &Task{
					Name:           qaTaskName,
					TaskType:       "QA",
					Priority:       priority,
					Effort:         math.Round(effort * 0.25 * effortIncrease),
					ParallelFactor: 1,
					Dependencies:   dependencies,
					IsCompleted:    false,
				}
				tasks = append(tasks, qaTask)
			}
		}
	} else {
		return nil, nil, nil, fmt.Errorf("error reading tasks: %v", err)
	}

	// Load Developers
	var developers []*Developer
	if devRecords, err := readCSV(devsFile); err == nil {
		for _, record := range devRecords[1:] { // Skip header
			taskTypes := strings.Split(record[2], ",")
			developers = append(developers, &Developer{
				Name:      record[0],
				Role:      record[1],
				TaskTypes: taskTypes,
			})
		}
	} else {
		return nil, nil, nil, fmt.Errorf("error reading developers: %v", err)
	}

	return tasks, developers, roles, nil
}

func readCSV(filename string) ([][]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return records, nil
}

// func createTasksCSV() {
// 	tasksFile, err := os.Create("tasks.csv")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer tasksFile.Close()

// 	writer := csv.NewWriter(tasksFile)
// 	defer writer.Flush()

// 	// Write header
// 	writer.Write([]string{"Name", "TaskType", "Priority", "Effort", "ParallelFactor", "Dependencies"})

// 	// Write data
// 	tasks := []*Task{
// 		{Name: "API Authentication", Priority: 1, ParallelFactor: 2, Effort: 8, TaskType: "Backend", Dependencies: []string{}},
// 		{Name: "Database Schema Design", Priority: 1, ParallelFactor: 1, Effort: 4, TaskType: "Backend", Dependencies: []string{}},
// 		{Name: "User Interface Design", Priority: 2, ParallelFactor: 2, Effort: 6, TaskType: "Frontend", Dependencies: []string{}},
// 		{Name: "API Implementation", Priority: 3, ParallelFactor: 3, Effort: 12, TaskType: "Backend", Dependencies: []string{"API Authentication", "Database Schema Design"}},
// 		{Name: "Frontend Implementation", Priority: 4, ParallelFactor: 2, Effort: 10, TaskType: "Frontend", Dependencies: []string{"User Interface Design", "API Implementation"}},
// 		{Name: "Integration Testing", Priority: 5, ParallelFactor: 2, Effort: 6, TaskType: "QA", Dependencies: []string{"Frontend Implementation", "API Implementation"}},
// 	}

// 	for _, task := range tasks {
// 		writer.Write([]string{
// 			task.Name,
// 			task.TaskType,
// 			strconv.Itoa(task.Priority),
// 			fmt.Sprintf("%.1f", task.Effort),
// 			strconv.Itoa(task.ParallelFactor),
// 			strings.Join(task.Dependencies, ","),
// 		})
// 	}
// }

// func createDevelopersCSV() {
// 	devsFile, err := os.Create("developers.csv")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer devsFile.Close()

// 	writer := csv.NewWriter(devsFile)
// 	defer writer.Flush()

// 	// Write header
// 	writer.Write([]string{"Name", "Role", "TaskTypes"})

// 	// Write data
// 	devs := []*Developer{
// 		{Name: "Dev1", Role: "Senior", TaskTypes: []string{"Backend", "Frontend"}},
// 		{Name: "Dev2", Role: "Senior", TaskTypes: []string{"Backend", "QA"}},
// 		{Name: "Dev3", Role: "Mid", TaskTypes: []string{"Frontend", "QA"}},
// 		{Name: "Dev4", Role: "Junior", TaskTypes: []string{"Frontend", "Backend"}},
// 		{Name: "Dev5", Role: "Senior", TaskTypes: []string{"Backend", "QA"}},
// 	}

// 	for _, dev := range devs {
// 		writer.Write([]string{
// 			dev.Name,
// 			dev.Role,
// 			strings.Join(dev.TaskTypes, ","),
// 		})
// 	}
// }

// func createRolesCSV() {
// 	rolesFile, err := os.Create("roles.csv")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer rolesFile.Close()

// 	writer := csv.NewWriter(rolesFile)
// 	defer writer.Flush()

// 	// Write header
// 	writer.Write([]string{"Name", "AvailabilityPercent"})

// 	// Write data
// 	roles := map[string]*Role{
// 		"Senior": {Name: "Senior", AvailabilityPercent: 0.8},
// 		"Mid":    {Name: "Mid", AvailabilityPercent: 0.9},
// 		"Junior": {Name: "Junior", AvailabilityPercent: 1.0},
// 	}

// 	for _, role := range roles {
// 		writer.Write([]string{
// 			role.Name,
// 			fmt.Sprintf("%.1f", role.AvailabilityPercent),
// 		})
// 	}
// }

// func createOncallsCSV() {
// 	file, err := os.Create("oncalls.csv")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer file.Close()

// 	writer := csv.NewWriter(file)
// 	defer writer.Flush()

// 	// Write header
// 	writer.Write([]string{"DevName", "StartTime", "EndTime"})

// 	// Write data
// 	now := time.Now()
// 	oncalls := []OnCall{
// 		{DevName: "Dev1", StartTime: now, EndTime: now.Add(24 * time.Hour * 7)},
// 		{DevName: "Dev2", StartTime: now.Add(24 * time.Hour * 7), EndTime: now.Add(24 * time.Hour * 14)},
// 		{DevName: "Dev5", StartTime: now.Add(24 * time.Hour * 14), EndTime: now.Add(24 * time.Hour * 21)},
// 	}

// 	for _, oncall := range oncalls {
// 		writer.Write([]string{
// 			oncall.DevName,
// 			oncall.StartTime.Format("2006-01-02"),
// 			oncall.EndTime.Format("2006-01-02"),
// 		})
// 	}
// }

// func createLeavesCSV() {
// 	file, err := os.Create("leaves.csv")
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer file.Close()

// 	writer := csv.NewWriter(file)
// 	defer writer.Flush()

// 	// Write header
// 	writer.Write([]string{"DevName", "StartTime", "EndTime"})

// 	// Write data
// 	now := time.Now()
// 	leaves := []Leave{
// 		{DevName: "Dev3", StartTime: now.Add(24 * time.Hour * 3), EndTime: now.Add(24 * time.Hour * 7)},
// 		{DevName: "Dev4", StartTime: now.Add(24 * time.Hour * 10), EndTime: now.Add(24 * time.Hour * 15)},
// 	}

// 	for _, leave := range leaves {
// 		writer.Write([]string{
// 			leave.DevName,
// 			leave.StartTime.Format("2006-01-02"),
// 			leave.EndTime.Format("2006-01-02"),
// 		})
// 	}
// }
